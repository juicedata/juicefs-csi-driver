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

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var log = klog.NewKlogr().WithName("grace")

func ServeGfShutdown(addr string) error {
	_ = os.RemoveAll(addr)

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

func handleShutdown(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Error(err, "error reading from connection")
		return
	}

	message := string(buf[:n])

	var recreate bool
	ss := strings.Split(message, " ")
	name := ss[0]
	if len(ss) == 2 {
		recreate = true
	}

	log.V(1).Info("Received shutdown message", "message", message)

	client, err := k8s.NewClient()
	if err != nil {
		log.Error(err, "failed to create k8s client")
		return
	}
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Minute)
	defer cancel()

	mountPod, err := client.GetPod(ctx, name, config.Namespace)
	if err != nil {
		sendMessage(conn, "FAIL get pod")
		log.Error(err, "get pod error", "name", name)
		return
	}
	if mountPod.Spec.NodeName != config.NodeName {
		sendMessage(conn, "FAIL pod is not on node")
		return
	}
	ce := util.ContainSubString(mountPod.Spec.Containers[0].Command, "format")
	hashVal := mountPod.Labels[config.PodJuiceHashLabelKey]
	if hashVal == "" {
		log.Info("pod has no hash label")
		return
	}
	log.V(1).Info("get hash val from pod", "pod", mountPod.Name, "hash", hashVal)
	pu := &podUpgrade{
		client:   client,
		pod:      mountPod,
		recreate: recreate,
		ce:       ce,
		hashVal:  hashVal,
	}
	if err := pu.gracefulShutdown(ctx, conn); err != nil {
		log.Error(err, "graceful shutdown error")
		return
	}
}

type podUpgrade struct {
	client     *k8s.K8sClient
	pod        *corev1.Pod
	recreate   bool
	ce         bool
	hashVal    string
	newVersion string
}

func (p *podUpgrade) gracefulShutdown(ctx context.Context, conn net.Conn) error {
	lock := config.GetPodLock(p.hashVal)
	err := func() error {
		lock.Lock()
		defer lock.Unlock()
		var jfsConf *util.JuiceConf
		var err error

		if jfsConf, err = p.prepareShutdown(ctx, conn); err != nil {
			return err
		}

		if err := p.sighup(ctx, conn, jfsConf); err != nil {
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

func (p *podUpgrade) sighup(ctx context.Context, conn net.Conn, jfsConf *util.JuiceConf) error {
	// send SIGHUP to mount pod
	for i := 0; i < 600; i++ {
		log.Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", p.pod.Name)
		sendMessage(conn, "send SIGHUP to mount pod")
		if stdout, stderr, err := p.client.ExecuteInContainer(
			ctx,
			p.pod.Name,
			p.pod.Namespace,
			config.MountContainerName,
			[]string{"kill", "-s", "SIGHUP", strconv.Itoa(jfsConf.Pid)},
		); err != nil {
			log.V(1).Info("kill -s SIGHUP", "pid", jfsConf.Pid, "stdout", stdout, "stderr", stderr, "error", err)
			continue
		}
		upgradeEvtMsg := fmt.Sprintf("Upgrade binary to %s in %s", p.newVersion, config.MountContainerName)
		if p.recreate {
			upgradeEvtMsg = "Upgrade pod with recreating"
			sendMessage(conn, upgradeEvtMsg)
		} else {
			sendMessage(conn, "SUCCESS "+upgradeEvtMsg)
		}
		if err := p.client.CreateEvent(ctx, *p.pod, corev1.EventTypeNormal, "Upgrade", upgradeEvtMsg); err != nil {
			log.Error(err, "fail to create event")
		}
		return nil
	}
	sendMessage(conn, "FAIL to send SIGHUP to mount pod")
	log.Info("mount point of mount pod is busy, stop upgrade", "podName", p.pod.Name)
	return nil
}

func (p *podUpgrade) prepareShutdown(ctx context.Context, conn net.Conn) (*util.JuiceConf, error) {
	mntPath, _, err := util.GetMountPathOfPod(*p.pod)
	if err != nil {
		return nil, err
	}

	hashVal := p.pod.Labels[config.PodJuiceHashLabelKey]

	// get pid and sid from <mountpoint>/.config
	msg := "get pid from config"
	sendMessage(conn, msg)
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
	sendMessage(conn, fmt.Sprintf("pid in mount pod: %d", jfsConf.Pid))

	cJob, err := builder.NewCanaryJob(ctx, p.client, p.pod, p.recreate)
	if err != nil {
		return nil, err
	}
	sendMessage(conn, fmt.Sprintf("create canary job %s", cJob.Name))
	if _, err := p.client.CreateJob(ctx, cJob); err != nil {
		log.Error(err, "create canary pod error", "name", p.pod.Name)
		return nil, err
	}

	sendMessage(conn, "wait for canary job completed")
	if err := resource.WaitForJobComplete(ctx, p.client, cJob.Name, 5*time.Minute); err != nil {
		log.Error(err, "canary job is not complete, delete it.", "job", cJob.Name)
		_ = p.client.DeleteJob(ctx, cJob.Name, cJob.Namespace)
		return nil, err
	}

	sendMessage(conn, fmt.Sprintf("new image: %s", cJob.Spec.Template.Spec.Containers[0].Image))
	sendMessage(conn, "validate new version")
	v := p.validateVersion(ctx, conn)
	if !v {
		return nil, fmt.Errorf("new version is not supported")
	}

	if p.recreate {
		// set fuse fd to -1 in mount pod

		// update sid
		if p.ce {
			passfd.GlobalFds.UpdateSid(hashVal, jfsConf.Meta.Sid)
			log.V(1).Info("update sid", "mountPod", p.pod.Name, "sid", jfsConf.Meta.Sid)
			sendMessage(conn, fmt.Sprintf("sid in mount pod: %d", jfsConf.Meta.Sid))
		}

		// close fuse fd in mount pod
		commPath, err := resource.GetCommPath("/tmp", *p.pod)
		if err != nil {
			return nil, err
		}
		msg = "close fuse fd in mount pod"
		sendMessage(conn, msg)
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
		log.V(1).Info(msg, "pod", p.pod.Name)
		sendMessage(conn, msg)
		if err := p.uploadBinary(ctx); err != nil {
			return nil, err
		}
	}
	return jfsConf, nil
}

func (p *podUpgrade) validateVersion(ctx context.Context, conn net.Conn) bool {
	hashVal := p.pod.Labels[config.PodJuiceHashLabelKey]
	if hashVal == "" {
		return false
	}
	// read from version file
	var (
		v   []byte
		err error
	)
	err = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		v, err = os.ReadFile(fmt.Sprintf("/tmp/%s/version", hashVal))
		return err
	})
	if err != nil {
		log.Error(err, "read version file error", "hash", hashVal)
		sendMessage(conn, fmt.Sprintf("FAIL read version file error: %v", err))
		return false
	}
	p.newVersion = string(v)
	if p.recreate {
		supported := util.SupportUpgradeRecreate(p.ce, string(v))
		if !supported {
			sendMessage(conn, fmt.Sprintf("FAIL new version %s is not supported", string(v)))
		}
		return supported
	}
	supported := util.SupportUpgradeBinary(p.ce, string(v))
	if !supported {
		sendMessage(conn, fmt.Sprintf("FAIL new version %s is not supported", string(v)))
	}
	return supported
}

func (p *podUpgrade) waitForUpgrade(ctx context.Context, conn net.Conn) {
	hashVal := p.pod.Labels[config.PodJuiceHashLabelKey]
	if hashVal == "" {
		return
	}
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
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
				config.PodTypeKey:           config.PodTypeValue,
				config.PodJuiceHashLabelKey: hashVal,
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
						sendMessage(conn, fmt.Sprintf("SUCCESS Upgrade mount pod and recreate one: %s", po.Name))
						return
					} else {
						sendMessage(conn, fmt.Sprintf("Wait for new mount pod ready: %s", po.Name))
					}
				}
			}
		case <-ctx.Done():
			sendMessage(conn, "FAIL Upgrade mount pod timeout")
			return
		}
	}
}

func (p *podUpgrade) uploadBinary(ctx context.Context) error {
	ce := util.ContainSubString(p.pod.Spec.Containers[0].Command, "format")
	if ce {
		stdout, stderr, err := p.client.ExecuteInContainer(
			ctx,
			p.pod.Name,
			p.pod.Namespace,
			config.MountContainerName,
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
		config.MountContainerName,
		[]string{"sh", "-c", "rm -rf /usr/bin/juicefs && mv /tmp/juicefs /usr/bin/juicefs  && rm -rf /usr/local/juicefs/mount/jfsmount && mv /tmp/jfsmount /usr/local/juicefs/mount/jfsmount"},
	)
	if err != nil {
		log.Error(err, "upload binary error", "pod", p.pod.Name, "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil

}

func TriggerShutdown(socketPath string, name string, restart bool) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Error(err, "error connecting to socket")
		return err
	}
	defer conn.Close()

	message := name
	if restart {
		message = fmt.Sprintf("%s RESTART", name)
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error(err, "error sending message")
		return err
	}
	log.Info("trigger gracefully shutdown successfully", "name", name)

	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}

		message = string(buf[:n])
		log.Info(message)
		if strings.HasPrefix(message, "SUCCESS") || strings.HasPrefix(message, "FAIL") {
			break
		}
	}

	return nil
}

func sendMessage(conn net.Conn, message string) {
	_, err := conn.Write([]byte(message))
	if err != nil {
		log.V(1).Info("error sending message", "message", message, "error", err)
	}
}
