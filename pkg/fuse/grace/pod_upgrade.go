/*
 Copyright 2024 Juicedata Inc

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
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

type PodUpgrade struct {
	*GraceUpgrade
	client      *k8s.K8sClient
	pod         *corev1.Pod
	recreate    bool
	ce          bool
	hashVal     string
	upgradeUUID string
	status      config.UpgradeStatus
}

var _ GraceRunner = &PodUpgrade{}

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
		GraceUpgrade: &GraceUpgrade{
			client: client,
			conn:   conn,
		},
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
	if p.isInUpgradeProcess() {
		sendMessage(conn, fmt.Sprintf("POD-SKIP [%s] pod is already in upgrade process.", p.pod.Name))
		return nil
	}
	p.GraceUpgrade = &GraceUpgrade{client: p.client, conn: conn}
	return p.GraceUpgrade.runGracefulUpgrade(ctx, p)
}

func (p *PodUpgrade) LockKey() string {
	return p.hashVal
}

func (p *PodUpgrade) OnFail() {
	p.status = config.Fail
}

func (p *PodUpgrade) StatusPrefix() string {
	return "POD"
}

func (p *PodUpgrade) TargetName() string {
	return p.pod.Name
}

func (p *PodUpgrade) Sighup(ctx context.Context, jfsConf *util.JuiceConf) error {
	// send SIGHUP to mount pod
	log.Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", p.pod.Name, "namespace", p.pod.Namespace)
	p.sendMessage(fmt.Sprintf("send SIGHUP to mount pod %s", p.pod.Name))
	if err := p.GraceUpgrade.sighup(ctx, p.pod, common.MountContainerName, jfsConf.Pid); err != nil {
		p.status = config.Fail
		return fmt.Errorf("fail to send SIGHUP to mount pod: %v", err)
	}
	upgradeEvtMsg := fmt.Sprintf("[%s] Upgrade binary in %s", p.pod.Name, common.MountContainerName)
	if p.recreate {
		upgradeEvtMsg = fmt.Sprintf("Upgrade pod [%s] with recreating", p.pod.Name)
		p.sendMessage(upgradeEvtMsg)
	} else {
		p.sendMessage("POD-SUCCESS " + upgradeEvtMsg)
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

func (p *PodUpgrade) PrepareShutdown(ctx context.Context) (*util.JuiceConf, error) {
	mntPath, _, err := util.GetMountPathOfPod(*p.pod)
	if err != nil {
		return nil, err
	}

	// get pid and sid from <mountpoint>/.config
	msg := "get pid from config"
	log.V(1).Info(msg, "path", mntPath, "pod", p.pod.Name)
	var conf []byte
	err = util.DoWithTimeout(ctx, 2*time.Second, func(ctx context.Context) error {
		configFileName := util.GetJfsInternalFileName(p.pod, ".config")
		confPath := path.Join(mntPath, configFileName)
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
	log.Info("create canary job", "job", cJob.Name, "pod", p.pod.Name)
	p.sendMessage(fmt.Sprintf("create canary job %s for %s", cJob.Name, p.pod.Name))
	if _, err := p.client.CreateJob(ctx, cJob); err != nil && !k8serrors.IsAlreadyExists(err) {
		log.Error(err, "create canary pod error", "name", p.pod.Name)
		return nil, fmt.Errorf("fail to create canary job: %v", err)
	}

	log.Info("wait for canary job completed", "job", cJob.Name)
	p.sendMessage(fmt.Sprintf("wait for canary job %s for %s completed", cJob.Name, p.pod.Name))
	if err := resource.WaitForJobComplete(ctx, p.client, cJob.Name, 5*time.Minute); err != nil {
		log.Error(err, "canary job is not completed.", "job", cJob.Name)
		// check its pods
		labelSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{common.CanaryJobLabelKey: cJob.Name},
		}
		fieldSelector := fields.Set{
			"spec.nodeName": config.NodeName,
		}
		canaryPods, perr := p.client.ListPod(ctx, cJob.Namespace, &labelSelector, &fieldSelector)
		if perr != nil {
			return nil, fmt.Errorf("fail to list pods: %v", perr)
		}
		if len(canaryPods) == 0 {
			return nil, fmt.Errorf("fail to wait for canary job complete, no pods found: %v", err)
		}
		return nil, fmt.Errorf("fail to wait for canary job complete, its pod %s status: %s, err: %v", canaryPods[0].Name, resource.GetPodStatus(&canaryPods[0]), err)
	}
	p.sendMessage(fmt.Sprintf("canary job of mount pod %s completed", p.pod.Name))

	if p.recreate {
		if err := resource.AddPodAnnotation(ctx, p.client, p.pod.Name, p.pod.Namespace, map[string]string{common.JfsUpgradeProcess: time.Now().Format(time.DateTime)}); err != nil {
			p.sendMessage(fmt.Sprintf("POD-FAIL [%s] %s.", p.pod.Name, err.Error()))
			return nil, err
		}

		// set fuse fd to -1 in mount pod
		// update sid
		if p.ce {
			passfd.GlobalFds.UpdateSid(p.pod, jfsConf.Meta.Sid)
			log.Info("update sid", "mountPod", p.pod.Name, "sid", jfsConf.Meta.Sid)
		}
		ppid, err := resolvePpid(jfsConf, p.pod.Spec.HostPID)
		if err != nil {
			return nil, fmt.Errorf("mount pod %s/%s: %w", p.pod.Namespace, p.pod.Name, err)
		}
		commPath, err := resource.GetCommPath("/tmp", *p.pod, ppid)
		if err != nil {
			return nil, err
		}
		passfd.GlobalFds.UpdateStatePath(p.pod, jfsConf.StatePath)
		log.Info("update state path", "mountPod", p.pod.Name, "statePath", jfsConf.StatePath)

		// close fuse fd in mount pod
		msg = "close fuse fd in mount pod"
		log.Info(msg, "path", commPath, "pod", p.pod.Name)
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
		p.sendMessage(msg)
		if err := p.GraceUpgrade.uploadBinary(ctx, p.pod, common.MountContainerName, p.ce); err != nil {
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
