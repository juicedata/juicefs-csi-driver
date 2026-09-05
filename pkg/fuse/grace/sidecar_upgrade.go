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
	"encoding/json"
	"fmt"
	"net"
	"path"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

type SidecarUpgradeTarget struct {
	Namespace     string
	PodName       string
	ContainerName string
}

type SidecarUpgradeRunner struct {
	*GraceUpgrade
	client      *k8s.K8sClient
	target      SidecarUpgradeTarget
	pod         *corev1.Pod
	confPath    string
	isCe        bool
	targetImage string
	onFail      func()
}

var _ GraceRunner = &SidecarUpgradeRunner{}

func NewSidecarUpgradeRunner(client *k8s.K8sClient, target SidecarUpgradeTarget, pod *corev1.Pod) *SidecarUpgradeRunner {
	return &SidecarUpgradeRunner{
		GraceUpgrade: &GraceUpgrade{client: client},
		client:       client,
		target:       target,
		pod:          pod,
	}
}

func RunSidecarUpgrade(ctx context.Context, client *k8s.K8sClient, target SidecarUpgradeTarget) error {
	return NewSidecarUpgradeRunner(client, target, nil).run(ctx, nil)
}

func (r *SidecarUpgradeRunner) run(ctx context.Context, conn net.Conn) error {
	r.GraceUpgrade = &GraceUpgrade{client: r.client, conn: conn}
	return r.GraceUpgrade.runGracefulUpgrade(ctx, r)
}

func (r *SidecarUpgradeRunner) StatusPrefix() string {
	return "POD"
}

func (r *SidecarUpgradeRunner) TargetName() string {
	if r.target.PodName != "" {
		if r.target.ContainerName != "" {
			return r.target.PodName + "/" + r.target.ContainerName
		}
		return r.target.PodName
	}
	return r.target.ContainerName
}

func (r *SidecarUpgradeRunner) LockKey() string {
	return r.target.PodName
}

func (r *SidecarUpgradeRunner) PrepareShutdown(ctx context.Context) (*util.JuiceConf, error) {
	if r.pod == nil {
		pod, err := r.client.GetPod(ctx, r.target.PodName, r.target.Namespace)
		if err != nil {
			return nil, err
		}
		r.pod = pod
	}
	container, err := findSidecarContainer(r.pod, r.target.ContainerName)
	if err != nil {
		return nil, err
	}
	if err := config.LoadFromConfigMap(ctx, r.client); err != nil {
		return nil, err
	}
	r.targetImage, r.isCe, err = config.ResolveSidecarTargetImage(r.pod, container,
		func(name, namespace string) (*corev1.Secret, error) {
			return r.client.GetSecret(ctx, name, namespace)
		},
		func(name, namespace string) (*corev1.PersistentVolumeClaim, error) {
			return r.client.GetPersistentVolumeClaim(ctx, name, namespace)
		},
	)
	if err != nil {
		return nil, err
	}
	if r.targetImage == "" {
		r.sendMessage(fmt.Sprintf("POD-SKIP [%s/%s] target image is empty.", r.pod.Name, r.target.ContainerName))
		return nil, nil
	}
	mntPath, _, err := util.GetMountPathOfSidecar(*r.pod, r.target.ContainerName)
	if err != nil {
		return nil, err
	}
	confName := util.GetJfsInternalFileNameOfContainer(container, ".config")
	r.confPath = path.Join(mntPath, confName)
	confContent, stderr, err := r.client.ExecuteInContainer(ctx, r.pod.Name, r.pod.Namespace, r.target.ContainerName, []string{"cat", r.confPath})
	if err != nil {
		return nil, fmt.Errorf("read %s failed: %v, stderr: %s", r.confPath, err, stderr)
	}
	jfsConf, err := util.ParseConfig([]byte(confContent))
	if err != nil {
		return nil, fmt.Errorf("parse %s failed: %w", r.confPath, err)
	}

	job := builder.NewCanaryJobFromSpec(builder.CanaryJobSpec{
		Name:               sidecarCanaryJobName(r.target),
		Namespace:          r.pod.Namespace,
		Image:              r.targetImage,
		Command:            []string{"sh", "-ec", buildSidecarCanaryCopyCommand(r.isCe, r.pod.Namespace, r.pod.Name, r.target.ContainerName)},
		ServiceAccountName: common.UpgradeJobServiceAccountName(),
	})

	r.sendMessage(fmt.Sprintf("create canary job %s for %s/%s", job.Name, r.target.PodName, r.target.ContainerName))

	if _, err := r.client.CreateJob(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	}
	r.sendMessage(fmt.Sprintf("wait for canary job %s for %s/%s completed", job.Name, r.target.PodName, r.target.ContainerName))
	if err := waitForJobCompleteQuiet(ctx, r.client, job.Name, 5*time.Minute); err != nil {
		return nil, fmt.Errorf("fail to wait for canary job complete: %w", err)
	}
	r.sendMessage(fmt.Sprintf("canary job of sidecar %s/%s completed", r.target.PodName, r.target.ContainerName))

	if err := r.GraceUpgrade.uploadBinary(ctx, r.pod, r.target.ContainerName, r.isCe); err != nil {
		return nil, err
	}
	return jfsConf, nil
}

func (r *SidecarUpgradeRunner) Sighup(ctx context.Context, jfsConf *util.JuiceConf) error {
	r.sendMessage(fmt.Sprintf("send SIGHUP to sidecar %s/%s/%s", r.target.Namespace, r.target.PodName, r.target.ContainerName))
	if err := r.GraceUpgrade.sighup(ctx, r.pod, r.target.ContainerName, jfsConf.Pid); err != nil {
		return err
	}
	if err := updateSidecarUpgradeAnnotation(ctx, r.client, r.target, r.targetImage); err != nil {
		return err
	}
	upgradeEvtMsg := fmt.Sprintf("[%s] Upgrade binary in %s", r.pod.Name, r.target.ContainerName)
	if err := r.client.CreateEvent(ctx, *r.pod, corev1.EventTypeNormal, "Upgrade", upgradeEvtMsg); err != nil {
		log.Error(err, "fail to create event")
	}
	return nil
}

func (r *SidecarUpgradeRunner) OnFail() {
	if r.onFail != nil {
		r.onFail()
	}
}

func findSidecarContainer(pod *corev1.Pod, name string) (*corev1.Container, error) {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return &pod.Spec.Containers[i], nil
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == name {
			return &pod.Spec.InitContainers[i], nil
		}
	}
	return nil, fmt.Errorf("sidecar container %s not found in pod %s/%s", name, pod.Namespace, pod.Name)
}

func sidecarCanaryJobName(target SidecarUpgradeTarget) string {
	base := target.PodName + "-" + target.ContainerName
	base = base + "-" + util.RandStringRunes(6)
	return builder.GenJobNameByVolumeId(base) + "-canary"
}

func buildSidecarCanaryCopyCommand(isCe bool, namespace, podName, containerName string) string {
	dest := fmt.Sprintf("%s/%s:/tmp", namespace, podName)
	if isCe {
		return fmt.Sprintf("set -e; kubectl cp /usr/local/bin/juicefs %s/juicefs -c %s", dest, containerName)
	}
	return fmt.Sprintf("set -e; kubectl cp /usr/bin/juicefs %s/juicefs -c %s; kubectl cp /usr/local/juicefs/mount/jfsmount %s/jfsmount -c %s", dest, containerName, dest, containerName)
}

func updateSidecarUpgradeAnnotation(ctx context.Context, client *k8s.K8sClient, target SidecarUpgradeTarget, targetImage string) error {
	pod, err := client.GetPod(ctx, target.PodName, target.Namespace)
	if err != nil {
		return err
	}
	annotations := config.ParseSidecarBinaryUpgradeAnnotation(pod)
	annotations[target.ContainerName] = config.SidecarBinaryUpgradeInfo{
		Image:      targetImage,
		UpgradedAt: metav1.NewTime(time.Now()),
	}
	raw, err := json.Marshal(annotations)
	if err != nil {
		return err
	}
	return resource.AddPodAnnotation(ctx, client, pod.Name, pod.Namespace, map[string]string{
		common.SidecarBinaryUpgradeAnnotationKey: string(raw),
	})
}

func waitForJobCompleteQuiet(ctx context.Context, client *k8s.K8sClient, name string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	timer := time.NewTicker(2 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-waitCtx.Done():
			job, err := client.GetJob(waitCtx, name, config.Namespace)
			if err != nil {
				return err
			}
			return fmt.Errorf("timeout, last status: %s", resource.GetJobStatus(job))
		case <-timer.C:
			job, err := client.GetJob(waitCtx, name, config.Namespace)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return fmt.Errorf("timeout, last status: %s", resource.GetJobStatus(job))
				}
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			if resource.IsJobFailed(job) {
				return fmt.Errorf("job %s failed, status: %s", name, resource.GetJobStatus(job))
			}
			if resource.IsJobCompleted(job) {
				return nil
			}
		}
	}
}
