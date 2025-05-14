/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package grace

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var log = klog.NewKlogr().WithName("grace")

const (
	recreate             = "RECREATE"
	noRecreate           = "NORECREATE"
	singleUpgradeTimeout = 30 * time.Minute
)

func ServeGfShutdown(addr string) error {
	err := util.DoWithTimeout(context.TODO(), 2*time.Second, func(ctx context.Context) error {
		if util.Exists(addr) {
			return os.Remove(addr)
		}
		return nil
	})
	if err != nil {
		return err
	}

	listener, err := net.Listen("unix", addr)
	if err != nil {
		log.Error(err, "error listening on socket")
		return err
	}

	log.Info("Serve gracefully shutdown is listening", "addr", addr)

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Error(err, "error accepting connection")
				continue
			}

			go handleShutdown(conn)
		}
	}()
	return nil
}

type upgradeRequest struct {
	action     string
	name       string
	configName string
	batchIndex int
}

// parseRequest parse request from message
// message format: <pod-name> [recreate/noRecreate]
func parseRequest(message string) upgradeRequest {
	req := upgradeRequest{
		action: noRecreate,
	}

	ss := strings.Split(message, " ")
	req.name = ss[0]
	if len(ss) < 2 {
		return req
	}
	req.action = ss[1]
	if ss[0] == "BATCH" && len(ss) > 2 {
		options := strings.Split(ss[2], ",")
		for _, option := range options {
			ops := strings.Split(option, "=")
			if len(ops) < 2 {
				continue
			}
			if ops[0] == "batchIndex" {
				w, err := strconv.Atoi(ops[1])
				if err != nil {
					log.Error(err, "failed to parse options", "option", option)
					continue
				}
				req.batchIndex = w
			}
			if ops[0] == "batchConfig" {
				req.configName = ops[1]
			}
		}
		return req
	}
	return req
}

func handleShutdown(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Error(err, "error reading from connection")
		return
	}

	message := string(buf[:n])
	req := parseRequest(message)

	if req.name == "list" {
		_, _ = conn.Write(passfd.GlobalFds.PrintFds())
		return
	}

	log.Info("Received shutdown message", "message", message)

	client, err := k8s.NewClient()
	if err != nil {
		log.Error(err, "failed to create k8s client")
		return
	}
	if req.name == "BATCH" {
		NewBatchUpgrade(client, req).BatchUpgrade(context.TODO(), conn)
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), singleUpgradeTimeout)
	defer cancel()
	SinglePodUpgrade(ctx, client, req.name, req.action == recreate, conn)
}

func SinglePodUpgrade(ctx context.Context, client *k8s.K8sClient, name string, recreate bool, conn net.Conn) {
	sendMessage(conn, fmt.Sprintf("POD-START [%s] start to upgrade", name))
	pu, err := NewPodUpgrade(ctx, client, name, recreate, conn)
	if err != nil {
		log.Error(err, "failed to create pod upgrade")
		return
	}

	canUpgrade, reason, err := resource.CanUpgradeWithHash(ctx, client, *pu.pod, pu.recreate)
	if err != nil || !canUpgrade {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] can not upgrade: %s.", pu.pod.Name, reason))
		return
	}

	if err := pu.gracefulShutdown(ctx, conn); err != nil {
		log.Error(err, "graceful shutdown error")
		if pu.recreate {
			if e := resource.DelPodAnnotation(ctx, client, pu.pod.Name, pu.pod.Namespace, []string{common.JfsUpgradeProcess}); e != nil {
				sendMessage(conn, fmt.Sprintf("WARNING delete annotation uprgadeProcess in [%s] error: %s.", pu.pod.Name, e.Error()))
				return
			}
		}
		return
	}
	if pu.recreate {
		pu.waitForUpgrade(ctx, conn)
	}
}

type PodUpgrade struct {
	client      *k8s.K8sClient
	pod         *corev1.Pod
	recreate    bool
	ce          bool
	hashVal     string
	upgradeUUID string
	status      config.UpgradeStatus
}

func NewPodUpgrade(ctx context.Context, client *k8s.K8sClient, name string, recreate bool, conn net.Conn) (*PodUpgrade, error) {
	mountPod, err := client.GetPod(ctx, name, config.Namespace)
	if err != nil {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] can not get pod.", name))
		log.Error(err, "get pod error", "name", name)
		return nil, err
	}
	if mountPod.Spec.NodeName != config.NodeName {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] pod is not on node.", name))
		return nil, err
	}
	ce := util.ContainSubString(mountPod.Spec.Containers[0].Command, "metaurl")
	hashVal := mountPod.Labels[common.PodJuiceHashLabelKey]
	if hashVal == "" {
		log.Info("pod has no hash label")
		return nil, err
	}
	log.V(1).Info("get hash val from pod", "pod", mountPod.Name, "hash", hashVal)
	pu := &PodUpgrade{
		client:      client,
		pod:         mountPod,
		recreate:    recreate,
		ce:          ce,
		hashVal:     hashVal,
		upgradeUUID: resource.GetUpgradeUUID(mountPod),
	}
	return pu, nil
}

func (p *PodUpgrade) gracefulShutdown(ctx context.Context, conn net.Conn) error {
	lock := config.GetPodLock(p.hashVal)
	lock.Lock()
	defer lock.Unlock()

	var jfsConf *util.JuiceConf
	var err error

	if p.isInUpgradeProcess() {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] pod is already in upgrade process.", p.pod.Name))
		return nil
	}
	if jfsConf, err = p.prepareShutdown(ctx, conn); err != nil {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] "+err.Error()+".", p.pod.Name))
		p.status = config.Fail
		return err
	}

	if err := p.sighup(ctx, conn, jfsConf); err != nil {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] "+err.Error()+".", p.pod.Name))
		p.status = config.Fail
		return err
	}
	return nil
}

func (p *PodUpgrade) sighup(ctx context.Context, conn net.Conn, jfsConf *util.JuiceConf) error {
	// send SIGHUP to mount pod
	log.Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", p.pod.Name, "namespace", p.pod.Namespace)
	sendMessage(conn, fmt.Sprintf("send SIGHUP to mount pod %s", p.pod.Name))
	if stdout, stderr, err := p.client.ExecuteInContainer(
		ctx,
		p.pod.Name,
		p.pod.Namespace,
		common.MountContainerName,
		[]string{"kill", "-s", "SIGHUP", strconv.Itoa(jfsConf.Pid)},
	); err != nil {
		log.V(1).Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", p.pod.Name, "stdout", stdout, "stderr", stderr, "error", err)
		p.status = config.Fail
		return fmt.Errorf("fail to send SIGHUP to mount pod: %v", err)
	}
	upgradeEvtMsg := fmt.Sprintf("[%s] Upgrade binary in %s", p.pod.Name, common.MountContainerName)
	if p.recreate {
		upgradeEvtMsg = fmt.Sprintf("Upgrade pod [%s] with recreating", p.pod.Name)
		sendMessage(conn, upgradeEvtMsg)
	} else {
		sendMessage(conn, "POD-SUCCESS "+upgradeEvtMsg)
		p.status = config.Success
	}
	if err := p.client.CreateEvent(ctx, *p.pod, corev1.EventTypeNormal, "Upgrade", upgradeEvtMsg); err != nil {
		log.Error(err, "fail to create event")
	}
	return nil
}

func (p *PodUpgrade) isInUpgradeProcess() bool {
	if p.pod.Annotations == nil || p.pod.Annotations[common.JfsUpgradeProcess] == "" {
		return false
	}
	t, err := time.Parse(time.DateTime, p.pod.Annotations[common.JfsUpgradeProcess])
	if err != nil {
		log.Error(err, "parse upgrade time from pod upgradeProcess error", "pod", p.pod.Name, "time", p.pod.Annotations[common.JfsUpgradeProcess])
		return false
	}
	return time.Since(t) < 5*time.Minute
}

func (p *PodUpgrade) prepareShutdown(ctx context.Context, conn net.Conn) (*util.JuiceConf, error) {
	mntPath, _, err := util.GetMountPathOfPod(*p.pod)
	if err != nil {
		return nil, err
	}

	// get pid and sid from <mountpoint>/.config
	msg := "get pid from config"
	log.V(1).Info(msg, "path", mntPath, "pod", p.pod.Name)
	var conf []byte
	err = util.DoWithTimeout(ctx, 2*time.Second, func(ctx context.Context) error {
		confPath := path.Join(mntPath, ".config")
		conf, err = os.ReadFile(confPath)
		if err != nil {
			return fmt.Errorf("fail to read config file %s: %v", confPath, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	jfsConf, err := util.ParseConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("fail to parse config file: %v", err)
	}
	log.V(1).Info("get pid in mount pod", "pid", jfsConf.Pid)

	cJob, err := builder.NewCanaryJob(ctx, p.client, p.pod, p.recreate)
	if err != nil {
		return nil, fmt.Errorf("fail to new canary job: %v", err)
	}
	log.V(1).Info("create canary job", "job", cJob.Name)
	if _, err := p.client.CreateJob(ctx, cJob); err != nil && !k8serrors.IsAlreadyExists(err) {
		log.Error(err, "create canary pod error", "name", p.pod.Name)
		return nil, fmt.Errorf("fail to create canary job: %v", err)
	}

	log.V(1).Info("wait for canary job completed", "job", cJob.Name)
	if err := resource.WaitForJobComplete(ctx, p.client, cJob.Name, 5*time.Minute); err != nil {
		log.Error(err, "canary job is not complete, delete it.", "job", cJob.Name)
		_ = p.client.DeleteJob(ctx, cJob.Name, cJob.Namespace)
		return nil, fmt.Errorf("fail to wait for canary job complete: %v", err)
	}
	sendMessage(conn, fmt.Sprintf("canary job of mount pod %s completed", p.pod.Name))

	if p.recreate {
		if err := resource.AddPodAnnotation(ctx, p.client, p.pod.Name, p.pod.Namespace, map[string]string{common.JfsUpgradeProcess: time.Now().Format(time.DateTime)}); err != nil {
			sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] %s.", p.pod.Name, err.Error()))
			return nil, err
		}

		// set fuse fd to -1 in mount pod
		// update sid
		if p.ce {
			passfd.GlobalFds.UpdateSid(p.pod, jfsConf.Meta.Sid)
			log.V(1).Info("update sid", "mountPod", p.pod.Name, "sid", jfsConf.Meta.Sid)
		}

		// close fuse fd in mount pod
		commPath, err := resource.GetCommPath("/tmp", *p.pod)
		if err != nil {
			return nil, err
		}
		msg = "close fuse fd in mount pod"
		log.V(1).Info(msg, "path", commPath, "pod", p.pod.Name)
		fuseFd, _ := passfd.GetFuseFd(commPath, true)
		for i := 0; i < 100 && fuseFd < 0; i++ {
			time.Sleep(time.Millisecond * 100)
			fuseFd, _ = passfd.GetFuseFd(commPath, true)
		}
		if fuseFd < 0 {
			return nil, fmt.Errorf("fail to recv FUSE fd from %s", commPath)
		}
		log.Info("recv FUSE fd", "fd", fuseFd)
	} else {
		// upgrade binary
		msg = "upgrade binary to mount pod"
		log.Info(msg, "pod", p.pod.Name)
		sendMessage(conn, msg)
		if err := p.uploadBinary(ctx); err != nil {
			return nil, fmt.Errorf("fail to upload binary: %v", err)
		}
	}
	return jfsConf, nil
}

func (p *PodUpgrade) waitForUpgrade(ctx context.Context, conn net.Conn) {
	log.Info("wait for upgrade", "pod", p.pod.Name)
	upgradeUUID := p.upgradeUUID
	if upgradeUUID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	matchLabels := map[string]string{
		common.PodTypeKey: common.PodTypeValue,
	}
	if p.pod.Labels[common.PodUpgradeUUIDLabelKey] != "" {
		matchLabels[common.PodUpgradeUUIDLabelKey] = upgradeUUID
	}
	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: matchLabels})
	fieldSelector := fields.Set{"spec.nodeName": p.pod.Spec.NodeName}

	stop := make(chan struct{})
	done := make(chan struct{})
	defer func() {
		close(stop)
		close(done)
	}()
	watchlist := cache.NewFilteredListWatchFromClient(
		p.client.CoreV1().RESTClient(),
		"pods",
		p.pod.Namespace,
		func(options *metav1.ListOptions) {
			options.ResourceVersion = "0"
			options.FieldSelector = fieldSelector.String()
			options.LabelSelector = labelSelector.String()
		},
	)
	handle := func(obj interface{}) {
		po, ok := obj.(*corev1.Pod)
		if !ok {
			return
		}
		if resource.GetUpgradeUUID(po) == upgradeUUID && po.Name != p.pod.Name {
			if po.DeletionTimestamp == nil && !resource.IsPodComplete(po) {
				if resource.IsPodReady(po) {
					sendMessage(conn, fmt.Sprintf("POD-SUCCESS [%s] Upgrade mount pod and recreate one: %s !", p.pod.Name, po.Name))
					p.status = config.Success
					done <- struct{}{}
					return
				}
			}
		}
		if po.Name == p.pod.Name {
			if resource.IsPodComplete(po) {
				sendMessage(conn, fmt.Sprintf("Mount pod %s received signal and completed", p.pod.Name))
				return
			}
			if po.DeletionTimestamp != nil {
				sendMessage(conn, fmt.Sprintf("Mount pod %s is deleted", p.pod.Name))
				return
			}
		}
	}
	_, controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: watchlist,
		ObjectType:    &corev1.Pod{},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				handle(obj)
			},
			DeleteFunc: func(obj interface{}) {
				handle(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				handle(newObj)
			},
		},
	})
	go controller.Run(stop)

	for {
		select {
		case <-ctx.Done():
			sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] node may be busy, upgrade mount pod timeout, please check it later manually.", p.pod.Name))
			p.status = config.Fail
			return
		case <-done:
			return
		}
	}
}

func (p *PodUpgrade) uploadBinary(ctx context.Context) error {
	if p.ce {
		stdout, stderr, err := p.client.ExecuteInContainer(
			ctx,
			p.pod.Name,
			p.pod.Namespace,
			common.MountContainerName,
			[]string{"sh", "-c", "rm -rf /usr/local/bin/juicefs && mv /tmp/juicefs /usr/local/bin/juicefs"},
		)
		if err != nil {
			log.Error(err, "upload binary error", "pod", p.pod.Name, "stdout", stdout, "stderr", stderr)
			return err
		}
		return nil
	}

	stdout, stderr, err := p.client.ExecuteInContainer(
		ctx,
		p.pod.Name,
		p.pod.Namespace,
		common.MountContainerName,
		[]string{"sh", "-c", "rm -rf /usr/bin/juicefs && mv /tmp/juicefs /usr/bin/juicefs  && rm -rf /usr/local/juicefs/mount/jfsmount && mv /tmp/jfsmount /usr/local/juicefs/mount/jfsmount"},
	)
	if err != nil {
		log.Error(err, "upload binary error", "pod", p.pod.Name, "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil

}

func TriggerShutdown(socketPath string, name string, recreateFlag bool) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Error(err, "error connecting to socket")
		return err
	}
	defer conn.Close()

	var message string
	if recreateFlag {
		message = fmt.Sprintf("%s %s", name, recreate)
	} else {
		message = fmt.Sprintf("%s %s", name, noRecreate)
	}
	if name == "list" {
		message = "list"
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error(err, "error sending message")
		return err
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message = scanner.Text()
		fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
		if strings.HasPrefix(message, "POD-SUCCESS") || strings.HasPrefix(message, "POD-FAIL") {
			break
		}
	}

	return scanner.Err()
}

func sendMessage(conn net.Conn, message string) {
	_, err := conn.Write([]byte(message + "\n"))
	if err != nil {
		log.V(1).Info("error sending message", "message", message, "error", err)
	}
}
