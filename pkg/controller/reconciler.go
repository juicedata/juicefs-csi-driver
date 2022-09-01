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
	"strconv"
	"time"

	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"golang.org/x/net/context"
	"k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"k8s.io/klog"
	k8sexec "k8s.io/utils/exec"
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
	kc, err := newKubeletClient(config.HostIp, port)
	if err != nil {
		return err
	}

	// gen podDriver
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		klog.V(5).Infof("Could not create k8s client %v", err)
		return err
	}

	mounter := mount.SafeFormatAndMount{
		Interface: mount.New(""),
		Exec:      k8sexec.New(),
	}

	podDriver := NewPodDriver(k8sClient, mounter)

	go doReconcile(kc, podDriver)
	return nil
}

func doReconcile(kc *kubeletClient, driver *PodDriver) {
	for {
		podList, err := kc.GetNodeRunningPods()
		if err != nil {
			klog.Errorf("doReconcile GetNodeRunningPods: %v", err)
			goto finish
		}
		if err := driver.mit.parse(); err != nil {
			klog.Errorf("doReconcile ParseMountInfo: %v", err)
			goto finish
		}
		driver.mit.setPodsStatus(podList)

		for i := range podList.Items {
			pod := &podList.Items[i]
			if pod.Namespace != config.Namespace {
				continue
			}
			// check label
			if value, ok := pod.Labels[config.PodTypeKey]; !ok || value != config.PodTypeValue {
				continue
			}
			err := driver.Run(context.TODO(), pod)
			if err != nil {
				klog.Errorf("Check pod %s: %s", pod.Name, err)
			}
		}

	finish:
		time.Sleep(time.Duration(config.ReconcilerInterval) * time.Second)
	}
}
