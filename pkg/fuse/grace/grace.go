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
	err := util.DoWithTimeout(context.TODO(), 2*time.Second, func() error {
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

			log.Info("Start to graceful shutdown")
			go handleShutdown(conn)
		}
	}()
	return nil
}

type upgradeRequest struct {
	action string
	name   string
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

	log.V(1).Info("Received shutdown message", "message", message)

	client, err := k8s.NewClient()
	if err != nil {
		log.Error(err, "failed to create k8s client")
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
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] can not upgrade: %s", pu.pod.Name, reason))
		return
	}

	if err := pu.gracefulShutdown(ctx, conn); err != nil {
		log.Error(err, "graceful shutdown error")
		return
	}
}

type PodUpgrade struct {
	client      *k8s.K8sClient
	pod         *corev1.Pod
	recreate    bool
	ce          bool
	hashVal     string
	upgradeUUID string
	status      podUpgradeStatus
}

type podUpgradeStatus string

const (
	podUpgradeSuccess podUpgradeStatus = "success"
	podUpgradeFail    podUpgradeStatus = "fail"
)

func NewPodUpgrade(ctx context.Context, client *k8s.K8sClient, name string, recreate bool, conn net.Conn) (*PodUpgrade, error) {
	mountPod, err := client.GetPod(ctx, name, config.Namespace)
	if err != nil {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] can not get pod", name))
		log.Error(err, "get pod error", "name", name)
		return nil, err
	}
	if mountPod.Spec.NodeName != config.NodeName {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] pod is not on node", name))
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
	err := func() error {
		lock.Lock()
		defer lock.Unlock()
		var jfsConf *util.JuiceConf
		var err error

		if jfsConf, err = p.prepareShutdown(ctx, conn); err != nil {
			sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] "+err.Error(), p.pod.Name))
			p.status = podUpgradeFail
			return err
		}

		if err := p.sighup(ctx, conn, jfsConf); err != nil {
			sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] "+err.Error(), p.pod.Name))
			p.status = podUpgradeFail
			return err
		}
		return nil
	}()
	if err != nil {
		return err
	}

	if p.recreate {
		p.waitForUpgrade(ctx, conn)
	}
	return nil
}

func (p *PodUpgrade) sighup(ctx context.Context, conn net.Conn, jfsConf *util.JuiceConf) error {
	// send SIGHUP to mount pod
	log.Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", p.pod.Name)
	sendMessage(conn, fmt.Sprintf("send SIGHUP to mount pod %s", p.pod.Name))
	if stdout, stderr, err := p.client.ExecuteInContainer(
		ctx,
		p.pod.Name,
		p.pod.Namespace,
		common.MountContainerName,
		[]string{"kill", "-s", "SIGHUP", strconv.Itoa(jfsConf.Pid)},
	); err != nil {
		log.V(1).Info("kill -s SIGHUP", "pid", jfsConf.Pid, "stdout", stdout, "stderr", stderr, "error", err)
		sendMessage(conn, fmt.Sprintf("fail to send SIGHUP to mount pod: %v", err))
		p.status = podUpgradeFail
		return err
	}
	upgradeEvtMsg := fmt.Sprintf("[%s] Upgrade binary in %s", p.pod.Name, common.MountContainerName)
	if p.recreate {
		upgradeEvtMsg = "Upgrade pod with recreating"
		sendMessage(conn, upgradeEvtMsg)
	} else {
		sendMessage(conn, "POD-SUCCESS "+upgradeEvtMsg)
		p.status = podUpgradeSuccess
	}
	if err := p.client.CreateEvent(ctx, *p.pod, corev1.EventTypeNormal, "Upgrade", upgradeEvtMsg); err != nil {
		log.Error(err, "fail to create event")
	}
	return nil
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
	err = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		conf, err = os.ReadFile(path.Join(mntPath, ".config"))
		return err
	})
	jfsConf, err := util.ParseConfig(conf)
	if err != nil {
		return nil, err
	}
	log.V(1).Info("get pid in mount pod", "pid", jfsConf.Pid)

	cJob, err := builder.NewCanaryJob(ctx, p.client, p.pod, p.recreate)
	if err != nil {
		return nil, err
	}
	log.V(1).Info("create canary job", "job", cJob.Name)
	if _, err := p.client.CreateJob(ctx, cJob); err != nil && !k8serrors.IsAlreadyExists(err) {
		log.Error(err, "create canary pod error", "name", p.pod.Name)
		return nil, err
	}

	log.V(1).Info("wait for canary job completed", "job", cJob.Name)
	if err := resource.WaitForJobComplete(ctx, p.client, cJob.Name, 5*time.Minute); err != nil {
		log.Error(err, "canary job is not complete, delete it.", "job", cJob.Name)
		_ = p.client.DeleteJob(ctx, cJob.Name, cJob.Namespace)
		return nil, err
	}
	sendMessage(conn, fmt.Sprintf("canary job of mount pod %s completed", p.pod.Name))

	if p.recreate {
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
			return nil, err
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
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	reportDeleted := false
	for {
		select {
		case <-t.C:
			po, err := p.client.GetPod(ctx, p.pod.Name, p.pod.Namespace)
			if err != nil && !k8serrors.IsNotFound(err) {
				log.Error(err, "get pod error", "pod", p.pod.Name)
				sendMessage(conn, fmt.Sprintf("WARNING get pod error: %v", err))
				continue
			}
			if po != nil {
				if resource.IsPodComplete(po) {
					sendMessage(conn, fmt.Sprintf("Mount pod %s received signal and completed", p.pod.Name))
				}
			} else if !reportDeleted {
				sendMessage(conn, fmt.Sprintf("Mount pod %s is deleted", p.pod.Name))
				reportDeleted = true
			}
			labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
				common.PodTypeKey:             common.PodTypeValue,
				common.PodUpgradeUUIDLabelKey: upgradeUUID,
			}}
			fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
			pods, err := p.client.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
			if err != nil {
				log.Error(err, "List pod error")
				sendMessage(conn, fmt.Sprintf("WARNING list pod error: %v", err))
				continue
			}
			for _, po := range pods {
				if po.DeletionTimestamp == nil && !resource.IsPodComplete(&po) && po.Name != p.pod.Name {
					if resource.IsPodReady(&po) {
						sendMessage(conn, fmt.Sprintf("POD-SUCCESS [%s] Upgrade mount pod and recreate one: %s !", p.pod.Name, po.Name))
						p.status = podUpgradeSuccess
						return
					}
				}
			}
		case <-ctx.Done():
			sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] node may be busy, upgrade mount pod timeout, please check it later manually.", p.pod.Name))
			p.status = podUpgradeFail
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
