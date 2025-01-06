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
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

type BatchUpgrade struct {
	lock            sync.Mutex
	batchConfigName string
	batchConfig     *config.BatchConfig
	crtBatchIndex   int
	client          *k8s.K8sClient
	recreate        bool

	// batch
	podsToUpgrade []*PodUpgrade
	successSum    map[string]bool
	failSum       map[string]bool
}

func NewBatchUpgrade(client *k8s.K8sClient, req upgradeRequest) *BatchUpgrade {
	return &BatchUpgrade{
		lock:            sync.Mutex{},
		client:          client,
		recreate:        true,
		batchConfigName: req.configName,
		crtBatchIndex:   req.batchIndex,
		podsToUpgrade:   []*PodUpgrade{},
		successSum:      map[string]bool{},
		failSum:         map[string]bool{},
	}
}

func (u *BatchUpgrade) fetchPods(ctx context.Context, conn net.Conn) error {
	batchConfig, err := config.LoadUpgradeConfig(ctx, u.client, u.batchConfigName)
	if err != nil {
		return err
	}
	u.batchConfig = batchConfig
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.PodTypeValue,
		},
	}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	podLists, err := u.client.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		log.Error(err, "reconcile ListPod error")
		return err
	}
	u.podsToUpgrade = make([]*PodUpgrade, 0)
	podNames := make(map[string]bool)
	for _, batch := range batchConfig.Batches[u.crtBatchIndex-1] {
		if batch.Node == config.NodeName {
			podNames[batch.Name] = true
		}
	}
	for _, pod := range podLists {
		po := pod
		if _, ok := podNames[po.Name]; !ok {
			continue
		}
		delete(podNames, po.Name)
		canUpgrade, reason, err := resource.CanUpgradeWithHash(ctx, u.client, po, u.recreate)
		if err != nil || !canUpgrade {
			log.Info("pod can not upgrade, ignore", "pod", pod.Name, "err", err, "reason", reason)
			continue
		}
		ce := util.ContainSubString(pod.Spec.Containers[0].Command, "metaurl")
		pu := &PodUpgrade{
			client:      u.client,
			pod:         &po,
			recreate:    true,
			ce:          ce,
			hashVal:     pod.Labels[common.PodJuiceHashLabelKey],
			upgradeUUID: resource.GetUpgradeUUID(&po),
			status:      config.Running,
		}
		u.podsToUpgrade = append(u.podsToUpgrade, pu)
	}

	podNameStrs := []string{}
	for _, pu := range u.podsToUpgrade {
		name := pu.pod.Name
		podNameStrs = append(podNameStrs, name)
	}
	log.Info("pods to upgrade", "pods", strings.Join(podNameStrs, ", "))
	for name := range podNames {
		sendMessage(conn, fmt.Sprintf("POD-SUCCESS [%s] has already upgraded in node %s.", name, config.NodeName))
	}
	return nil
}

func (u *BatchUpgrade) BatchUpgrade(ctx context.Context, conn net.Conn) {
	if err := u.fetchPods(ctx, conn); err != nil {
		log.Error(err, "fetch pods error", "config", u.batchConfigName)
		sendMessage(conn, fmt.Sprintf("CRT-BATCH-FAIL fetch pods in node %s error: %s", config.NodeName, err.Error()))
		return
	}
	var (
		wg sync.WaitGroup
	)

	ctx, canF := context.WithCancel(ctx)
	defer canF()

	for _, p := range u.podsToUpgrade {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sendMessage(conn, fmt.Sprintf("POD-START [%s] start to upgrade", p.pod.Name))
			if err := p.gracefulShutdown(ctx, conn); err != nil && !u.failSum[p.pod.Name] {
				log.Error(err, "upgrade pod error", "pod", p.pod.Name)
				p.status = config.Fail
				u.lock.Lock()
				u.failSum[p.pod.Name] = true
				u.lock.Unlock()
				sendMessage(conn, fmt.Sprintf("pod [%s] upgrade pod error", p.pod.Name))
				if e := resource.DelPodAnnotation(ctx, u.client, p.pod.Name, p.pod.Namespace, []string{common.JfsUpgradeProcess}); e != nil {
					sendMessage(conn, fmt.Sprintf("WARNING delete annotation uprgadeProcess in [%s] error: %s.", p.pod.Name, e.Error()))
				}
				return
			}
		}()
	}
	wg.Wait()

	u.waitForUpgrade(ctx, conn)
}

func (u *BatchUpgrade) waitForUpgrade(ctx context.Context, conn net.Conn) {
	ctx, cancel := context.WithTimeout(ctx, 1200*time.Second)
	defer cancel()

	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.PodTypeValue,
		},
	})
	fieldSelector := fields.Set{
		"spec.nodeName": config.NodeName,
	}

	stop := make(chan struct{})
	defer func() {
		close(stop)
	}()
	watchlist := cache.NewFilteredListWatchFromClient(
		u.client.CoreV1().RESTClient(),
		"pods",
		config.Namespace,
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
		var pu *PodUpgrade
		for _, p := range u.podsToUpgrade {
			if p.pod.Labels[common.PodUpgradeUUIDLabelKey] == po.Labels[common.PodUpgradeUUIDLabelKey] {
				pu = p
				break
			}
		}
		if pu == nil {
			return
		}
		if po.Name != pu.pod.Name {
			if po.DeletionTimestamp == nil && !resource.IsPodComplete(po) {
				if resource.IsPodReady(po) && !u.successSum[pu.pod.Name] {
					pu.status = config.Success
					u.lock.Lock()
					u.successSum[pu.pod.Name] = true
					u.lock.Unlock()
					sendMessage(conn, fmt.Sprintf("POD-SUCCESS [%s] Upgrade mount pod and recreate one: %s !", pu.pod.Name, po.Name))
					return
				}
			}
		}
		if po.Name == pu.pod.Name {
			if resource.IsPodComplete(po) {
				sendMessage(conn, fmt.Sprintf("Mount pod %s received signal and completed", pu.pod.Name))
				return
			}
			if po.DeletionTimestamp != nil {
				sendMessage(conn, fmt.Sprintf("Mount pod %s is deleted", pu.pod.Name))
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
			for _, p := range u.podsToUpgrade {
				if p.status != config.Success {
					sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] node may be busy, upgrade mount pod timeout, please check it later manually.", p.pod.Name))
				}
			}
			sendMessage(conn, fmt.Sprintf("CRT-BATCH-FAIL pods of current batch upgrade timeout in node %s", config.NodeName))
			return
		default:
			if len(u.successSum) == len(u.podsToUpgrade) {
				sendMessage(conn, fmt.Sprintf("CRT-BATCH-SUCCESS all pods of current batch upgrade success in node %s", config.NodeName))
				return
			}
			if len(u.failSum) > 0 && len(u.failSum)+len(u.successSum) == len(u.podsToUpgrade) {
				sendMessage(conn, fmt.Sprintf("CRT-BATCH-FAIL some pods of current batch upgrade failed in node %s", config.NodeName))
				return
			}
		}
	}
}

func TriggerBatchUpgrade(socketPath string, batchConfigName string, batchIndex int) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Error(err, "error connecting to socket")
		return err
	}
	var message string
	message = fmt.Sprintf("BATCH %s batchConfig=%s,batchIndex=%d", recreate, batchConfigName, batchIndex)

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error(err, "error sending message")
		return err
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message = scanner.Text()
		fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
		if strings.HasPrefix(message, "CRT-BATCH") {
			break
		}
	}

	return scanner.Err()
}
