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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type BatchUpgrade struct {
	lock     sync.Mutex
	status   upgradeStatus
	client   *k8s.K8sClient
	recreate bool

	// batch
	podsToUpgrade []*PodUpgrade
}

type upgradeStatus string

const (
	batchUpgradeRunning upgradeStatus = "running"
	batchUpgradeWaiting upgradeStatus = "waiting"
)

var globalBatchUpgrade *BatchUpgrade

func InitBatchUpgrade(client *k8s.K8sClient) {
	globalBatchUpgrade = &BatchUpgrade{
		client:        client,
		lock:          sync.Mutex{},
		status:        batchUpgradeWaiting,
		podsToUpgrade: []*PodUpgrade{},
	}
}

func (u *BatchUpgrade) fetchPods(ctx context.Context) error {
	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		common.PodTypeKey: common.PodTypeValue,
	}}
	fieldSelector := &fields.Set{"spec.nodeName": config.NodeName}
	podLists, err := u.client.ListPod(ctx, config.Namespace, labelSelector, fieldSelector)
	if err != nil {
		log.Error(err, "reconcile ListPod error")
		return err
	}
	for _, pod := range podLists {
		ce := util.ContainSubString(pod.Spec.Containers[0].Command, "metaurl")
		hashVal := pod.Labels[common.PodJuiceHashLabelKey]
		if hashVal == "" {
			log.Info("pod has no hash label")
			continue
		}
		// todo: filter pod do not need to upgrade
		pu := &PodUpgrade{
			client:   u.client,
			pod:      &pod,
			recreate: u.recreate,
			ce:       ce,
			hashVal:  hashVal,
		}
		if ok := pu.canUpgrade(); !ok {
			log.Info("pod can not upgrade, ignore", "pod", pod.Name)
			continue
		}
		u.podsToUpgrade = append(u.podsToUpgrade, pu)
	}

	podNames := []string{}
	for _, pu := range u.podsToUpgrade {
		podNames = append(podNames, pu.pod.Name)
	}
	log.Info("node pods to upgrade", "pods", strings.Join(podNames, ", "))
	return nil
}

func (u *BatchUpgrade) BatchUpgrade(ctx context.Context, conn net.Conn, recreate bool) {
	if u.status == batchUpgradeRunning {
		log.Info("upgrade is running")
		sendMessage(conn, "upgrade is running")
		u.syncStatus(ctx, conn)
		return
	}
	u.batchUpgrade(ctx, conn, recreate)
}

func (u *BatchUpgrade) batchUpgrade(ctx context.Context, conn net.Conn, recreate bool) {
	u.lock.Lock()
	u.status = batchUpgradeRunning
	defer func() {
		u.status = batchUpgradeWaiting
		u.podsToUpgrade = []*PodUpgrade{}
		defer u.lock.Unlock()
	}()
	if err := u.fetchPods(ctx); err != nil {
		return
	}
	var (
		wg      = sync.WaitGroup{}
		success = true
	)
	for _, pu := range u.podsToUpgrade {
		p := pu
		p.recreate = recreate
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.gracefulShutdown(ctx, conn); err != nil {
				log.Error(err, "upgrade pod error", "pod", p.pod.Name)
				sendMessage(conn, fmt.Sprintf("POD-FAIL upgrade pod [%s] in node [%s] error", p.pod.Name, config.NodeName))
				success = false
				return
			}
			if p.status == podUpgradeSuccess {
				sendMessage(conn, fmt.Sprintf("POD-SUCCESS pod [%s] in node [%s] upgraded success", p.pod.Name, config.NodeName))
			} else {
				sendMessage(conn, fmt.Sprintf("POD-FAIL pod [%s] in node [%s] upgraded failed", p.pod.Name, config.NodeName))
				success = false
			}
		}()
	}

	wg.Wait()
	if success {
		sendMessage(conn, "BATCH-SUCCESS all pods upgrade success")
	} else {
		sendMessage(conn, "BATCH-FAIL some pods upgrade failed")
	}
}

func (u *BatchUpgrade) syncStatus(ctx context.Context, conn net.Conn) {
	var (
		finishPod = []string{}
		success   = true
	)
	for {
		select {
		case <-ctx.Done():
			sendMessage(conn, "BATCH-FAIL upgrade timeout")
			return
		default:
			for _, pu := range u.podsToUpgrade {
				if pu.status == podUpgradeSuccess && !util.ContainsString(finishPod, pu.pod.Name) {
					sendMessage(conn, fmt.Sprintf("POD-SUCCESS pod [%s] in node [%s] upgraded success", pu.pod.Name, config.NodeName))
					finishPod = append(finishPod, pu.pod.Name)
				}
				if pu.status == podUpgradeFail && !util.ContainsString(finishPod, pu.pod.Name) {
					success = false
					sendMessage(conn, fmt.Sprintf("POD-FAIL pod [%s] in node [%s] upgraded failed", pu.pod.Name, config.NodeName))
					finishPod = append(finishPod, pu.pod.Name)
				}
			}
			if len(finishPod) == len(u.podsToUpgrade) {
				if success {
					sendMessage(conn, "BATCH-SUCCESS all pods upgrade success")
				} else {
					sendMessage(conn, "BATCH-FAIL some pods upgrade failed")
				}
				return
			}
		}
	}
}

func TriggerBatchUpgrade(socketPath string, recreate bool) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Error(err, "error connecting to socket")
		return err
	}
	message := "BATCH"
	if recreate {
		message = "BATCH RECREATE"
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error(err, "error sending message")
		return err
	}
	log.Info("trigger batch upgrade success")

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message = scanner.Text()
		log.Info(message)
		if strings.HasPrefix(message, "BATCH-SUCCESS") || strings.HasPrefix(message, "BATCH-FAIL") {
			break
		}
	}

	return scanner.Err()
}
