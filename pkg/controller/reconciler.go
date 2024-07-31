/*
Copyright 2021 Juicedata Inc

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

package controller

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

const (
	retryPeriod    = 5 * time.Second
	maxRetryPeriod = 300 * time.Second
)

type PodReconciler struct {
	mount.SafeFormatAndMount
	*k8sclient.K8sClient
}

func StartReconciler() error {
	// gen kubelet client
	port, err := strconv.Atoi(config.KubeletPort)
	if err != nil {
		return err
	}
	kc, err := k8sclient.NewKubeletClient(config.HostIp, port)
	if err != nil {
		return err
	}

	// check if kubelet can be connected
	_, err = kc.GetNodeRunningPods()
	if err != nil {
		return err
	}

	// gen podDriver
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Could not create k8s client %v", err)
		return err
	}

	go doReconcile(k8sClient, kc)
	return nil
}

type PodStatus struct {
	podStatus
	syncAt     time.Time
	nextSyncAt time.Time
}

func doReconcile(ks *k8sclient.K8sClient, kc *k8sclient.KubeletClient) {
	backOff := flowcontrol.NewBackOff(retryPeriod, maxRetryPeriod)
	lastPodStatus := make(map[string]PodStatus)
	statusMu := sync.Mutex{}
	for {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), config.ReconcileTimeout)
		g, ctx := errgroup.WithContext(timeoutCtx)

		mit := newMountInfoTable()
		podList, err := kc.GetNodeRunningPods()
		if err != nil {
			klog.Errorf("doReconcile GetNodeRunningPods: %v", err)
			goto finish
		}
		if err := mit.parse(); err != nil {
			klog.Errorf("doReconcile ParseMountInfo: %v", err)
			goto finish
		}

		for i := range podList.Items {
			pod := &podList.Items[i]
			if pod.Namespace != config.Namespace {
				continue
			}
			// check label
			if value, ok := pod.Labels[config.PodTypeKey]; !ok || value != config.PodTypeValue {
				continue
			}
			crtPodStatus := getPodStatus(pod)
			statusMu.Lock()
			lastStatus, ok := lastPodStatus[pod.Name]
			statusMu.Unlock()
			if ok {
				if lastStatus.podStatus == crtPodStatus && time.Now().Before(lastStatus.nextSyncAt) {
					// skipped
					continue
				}
			}

			backOffID := fmt.Sprintf("mountpod/%s", pod.Name)
			g.Go(func() error {
				mounter := mount.SafeFormatAndMount{
					Interface: mount.New(""),
					Exec:      k8sexec.New(),
				}

				podDriver := NewPodDriver(ks, mounter)
				podDriver.SetMountInfo(*mit)
				podDriver.mit.setPodsStatus(podList)

				select {
				case <-ctx.Done():
					klog.Infof("goroutine of pod %s cancel", pod.Name)
					return nil
				default:
					if !backOff.IsInBackOffSinceUpdate(backOffID, backOff.Clock.Now()) {
						defer func() {
							statusMu.Lock()
							lastStatus.podStatus = crtPodStatus
							lastPodStatus[pod.Name] = lastStatus
							statusMu.Unlock()
						}()
						result, err := podDriver.Run(ctx, pod)
						lastStatus.syncAt = time.Now()
						if err != nil {
							klog.Errorf("Driver check pod %s error, will retry: %v", pod.Name, err)
							backOff.Next(backOffID, time.Now())
							lastStatus.nextSyncAt = time.Now()
							return err
						}
						backOff.Reset(backOffID)
						if result.RequeueImmediately {
							lastStatus.nextSyncAt = time.Now()
						} else if result.RequeueAfter > 0 {
							lastStatus.nextSyncAt = time.Now().Add(result.RequeueAfter)
						} else {
							lastStatus.nextSyncAt = time.Now().Add(10 * time.Minute)
						}
					}
				}
				return nil
			})
		}
		backOff.GC()
		_ = g.Wait()
	finish:
		cancel()
		time.Sleep(time.Duration(config.ReconcilerInterval) * time.Second)
	}
}
