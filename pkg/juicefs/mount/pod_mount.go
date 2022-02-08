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

package mount

import (
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/resources"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type PodMount struct {
	k8sMount.SafeFormatAndMount
	K8sClient *k8sclient.K8sClient
}

func NewPodMount(client *k8sclient.K8sClient, mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &PodMount{mounter, client}
}

func (p *PodMount) JMount(jfsSetting *jfsConfig.JfsSetting) error {
	if err := p.createOrAddRef(jfsSetting); err != nil {
		return err
	}
	return p.waitUtilPodReady(resources.GenerateNameByVolumeId(jfsSetting.VolumeId))
}

func (p *PodMount) JUmount(volumeId, target string) error {
	podName := resources.GenerateNameByVolumeId(volumeId)
	lock := jfsConfig.GetPodLock(podName)
	lock.Lock()
	defer lock.Unlock()

	// check mount pod is need to delete
	klog.V(5).Infof("JUmount: Delete target ref [%s] and check mount pod [%s] is need to delete or not.", target, podName)

	pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		klog.Errorf("JUmount: Get pod %s err: %v", podName, err)
		return err
	}

	// if mount pod not exists.
	if pod == nil {
		klog.V(5).Infof("JUmount: Mount pod %v not exists.", podName)
		return nil
	}

	key := util.GetReferenceKey(target)
	klog.V(6).Infof("JUmount: Target %v hash of target %v", target, key)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		po, err := p.K8sClient.GetPod(pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		annotation := po.Annotations
		if _, ok := annotation[key]; !ok {
			klog.V(5).Infof("JUmount: Target ref [%s] in pod [%s] already not exists.", target, pod.Name)
			return nil
		}
		delete(annotation, key)
		po.Annotations = annotation
		return p.K8sClient.UpdatePod(po)
	})
	if err != nil {
		klog.Errorf("JUmount: Remove ref of volumeId %s err: %v", volumeId, err)
		return err
	}

	deleteMountPod := func(podName, namespace string) error {
		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			po, err := p.K8sClient.GetPod(podName, namespace)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				klog.Errorf("JUmount: Get mount pod %s err %v", podName, err)
				return err
			}

			if resources.HasRef(po) {
				klog.V(5).Infof("JUmount: pod %s still has juicefs- refs.", podName)
				return nil
			}

			var shouldDelay bool
			shouldDelay, err = util.ShouldDelay(po, p.K8sClient)
			if err != nil {
				return err
			}
			if !shouldDelay {
				// do not set delay delete, delete it now
				klog.V(5).Infof("JUmount: pod %s has no juicefs- refs. delete it.", podName)
				if err := p.K8sClient.DeletePod(po); err != nil {
					klog.V(5).Infof("JUmount: Delete pod of volumeId %s error: %v", volumeId, err)
					return err
				}
			}
			return nil
		})
	}

	newPod, err := p.K8sClient.GetPod(pod.Name, pod.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		klog.Errorf("JUmount: Get mount pod %s err %v", podName, err)
		return err
	}
	if resources.HasRef(newPod) {
		return nil
	}
	// if pod annotations has no "juicefs-" prefix or no delete delay, delete pod
	return deleteMountPod(pod.Name, pod.Namespace)
}

func (p *PodMount) JCreateVolume(jfsSetting *jfsConfig.JfsSetting) error {
	var exist *batchv1.Job
	job := resources.NewJobForCreateVolume(jfsSetting)
	exist, err := p.K8sClient.GetJob(job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		klog.V(5).Infof("JCreateVolume: create job %s", job.Name)
		exist, err = p.K8sClient.CreateJob(job)
		if err != nil {
			klog.Errorf("JCreateVolume: create job %s err: %v", job.Name, err)
			return err
		}
	}
	if err != nil {
		klog.Errorf("JCreateVolume: get job %s err: %s", job.Name, err)
		return err
	}
	secret := resources.NewSecret(jfsSetting)
	resources.SetJobAsOwner(&secret, *exist)
	if err := p.createOrUpdateSecret(&secret); err != nil {
		return err
	}
	return p.waitUtilJobCompleted(job.Name)
}

func (p *PodMount) JDeleteVolume(jfsSetting *jfsConfig.JfsSetting) error {
	var exist *batchv1.Job
	job := resources.NewJobForDeleteVolume(jfsSetting)
	exist, err := p.K8sClient.GetJob(job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		klog.V(5).Infof("JDeleteVolume: create job %s", job.Name)
		exist, err = p.K8sClient.CreateJob(job)
		if err != nil {
			klog.Errorf("JDeleteVolume: create job %s err: %v", job.Name, err)
			return err
		}
	}
	if err != nil {
		klog.Errorf("JDeleteVolume: get job %s err: %s", job.Name, err)
		return err
	}
	secret := resources.NewSecret(jfsSetting)
	resources.SetJobAsOwner(&secret, *exist)
	if err := p.createOrUpdateSecret(&secret); err != nil {
		return err
	}
	return p.waitUtilJobCompleted(job.Name)
}

func (p *PodMount) createOrAddRef(jfsSetting *jfsConfig.JfsSetting) error {
	podName := resources.GenerateNameByVolumeId(jfsSetting.VolumeId)
	secretName := podName + "-secret"
	jfsSetting.SecretName = secretName
	lock := jfsConfig.GetPodLock(podName)
	lock.Lock()
	defer lock.Unlock()

	secret := resources.NewSecret(jfsSetting)
	key := util.GetReferenceKey(jfsSetting.TargetPath)
	for i := 0; i < 120; i++ {
		// wait for old pod deleted
		oldPod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err == nil && oldPod.DeletionTimestamp != nil {
			klog.V(6).Infof("createOrAddRef: wait for old mount pod deleted.")
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			if k8serrors.IsNotFound(err) {
				// pod not exist, create
				klog.V(5).Infof("createOrAddRef: Need to create pod %s.", podName)
				newPod := resources.NewMountPod(jfsSetting)
				if newPod.Annotations == nil {
					newPod.Annotations = make(map[string]string)
				}
				newPod.Annotations[key] = jfsSetting.TargetPath
				po, err := p.K8sClient.CreatePod(newPod)
				if err != nil {
					klog.Errorf("createOrAddRef: Create pod %s err: %v", podName, err)
				}
				resources.SetPodAsOwner(&secret, *po)
				if err := p.createOrUpdateSecret(&secret); err != nil {
					return err
				}
				return err
			}
			// unexpect error
			klog.Errorf("createOrAddRef: Get pod %s err: %v", podName, err)
			return err
		}
		// pod exist, add refs
		resources.SetPodAsOwner(&secret, *oldPod)
		if err := p.createOrUpdateSecret(&secret); err != nil {
			return err
		}
		return p.AddRefOfMount(jfsSetting.TargetPath, podName)
	}
	return status.Errorf(codes.Internal, "Mount %v failed: mount pod %s has been deleting for 1 min", jfsSetting.VolumeId, podName)
}

func (p *PodMount) waitUtilPodReady(podName string) error {
	// Wait until the mount pod is ready
	for i := 0; i < 60; i++ {
		pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return status.Errorf(codes.Internal, "waitUtilPodReady: Get pod %v failed: %v", podName, err)
		}
		if util.IsPodReady(pod) {
			klog.V(5).Infof("waitUtilPodReady: Pod %v is successful", podName)
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	log, err := p.getErrContainerLog(podName)
	if err != nil {
		klog.Errorf("waitUtilPodReady: get pod %s log error %v", podName, err)
	}
	return status.Errorf(codes.Internal, "waitUtilPodReady: mount pod %s isn't ready in 30 seconds: %v", podName, log)
}

func (p *PodMount) waitUtilJobCompleted(jobName string) error {
	// Wait until the job is completed
	for i := 0; i < 60; i++ {
		job, err := p.K8sClient.GetJob(jobName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				klog.Infof("waitUtilJobCompleted: Job %s is completed and been recycled", jobName)
				return nil
			}
			return status.Errorf(codes.Internal, "waitUtilJobCompleted: Get job %v failed: %v", jobName, err)
		}
		if util.IsJobCompleted(job) {
			klog.V(5).Infof("waitUtilJobCompleted: Job %s is completed", jobName)
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	pods, err := p.K8sClient.ListPod(jfsConfig.Namespace, metav1.LabelSelector{
		MatchLabels: map[string]string{
			"job-name": jobName,
		},
	})
	if err != nil || len(pods) != 1 {
		klog.Errorf("waitUtilJobCompleted: get pod from job %s error %v", jobName, err)
	}
	log, err := p.getNotCompleteCnLog(pods[0].Name)
	if err != nil {
		klog.Errorf("waitUtilJobCompleted: get pod %s log error %v", pods[0].Name, err)
	}
	return status.Errorf(codes.Internal, "waitUtilJobCompleted: job %s isn't completed in 30 seconds: %v", jobName, log)
}

func (p *PodMount) AddRefOfMount(target string, podName string) error {
	klog.V(5).Infof("addRefOfMount: Add target ref in mount pod. mount pod: [%s], target: [%s]", podName, target)
	// add volumeId ref in pod annotation
	key := util.GetReferenceKey(target)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		exist, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
		if err != nil {
			return err
		}
		if exist.DeletionTimestamp != nil {
			return status.Errorf(codes.Internal, "addRefOfMount: Mount pod [%s] has been deleted.", podName)
		}
		annotation := exist.Annotations
		if _, ok := annotation[key]; ok {
			klog.V(5).Infof("addRefOfMount: Target ref [%s] in pod [%s] already exists.", target, podName)
			return nil
		}
		if annotation == nil {
			annotation = make(map[string]string)
		}
		annotation[key] = target
		// delete deleteDelayAt when there ars refs
		delete(annotation, jfsConfig.DeleteDelayAtKey)
		exist.Annotations = annotation
		return p.K8sClient.UpdatePod(exist)
	})
	if err != nil {
		klog.Errorf("addRefOfMount: Add target ref in mount pod %s error: %v", podName, err)
		return err
	}
	return nil
}

func (p *PodMount) createOrUpdateSecret(secret *corev1.Secret) error {
	klog.V(5).Infof("createOrUpdateSecret: %s, %s", secret.Name, secret.Namespace)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		oldSecret, err := p.K8sClient.GetSecret(secret.Name, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// secret not exist, create
				_, err := p.K8sClient.CreateSecret(secret)
				return err
			}
			// unexpected err
			return err
		}

		oldSecret.StringData = secret.StringData
		return p.K8sClient.UpdateSecret(oldSecret)
	})
	if err != nil {
		klog.Errorf("createOrUpdateSecret: secret %s: %v", secret.Name, err)
		return err
	}
	return nil
}

func (p *PodMount) getErrContainerLog(podName string) (log string, err error) {
	pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
	if err != nil {
		return
	}
	for _, cn := range pod.Status.InitContainerStatuses {
		if !cn.Ready {
			log, err = p.K8sClient.GetPodLog(pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	for _, cn := range pod.Status.ContainerStatuses {
		if !cn.Ready {
			log, err = p.K8sClient.GetPodLog(pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	return
}

func (p *PodMount) getNotCompleteCnLog(podName string) (log string, err error) {
	pod, err := p.K8sClient.GetPod(podName, jfsConfig.Namespace)
	if err != nil {
		return
	}
	for _, cn := range pod.Status.InitContainerStatuses {
		if cn.State.Terminated == nil || cn.State.Terminated.Reason != "Completed" {
			log, err = p.K8sClient.GetPodLog(pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	for _, cn := range pod.Status.ContainerStatuses {
		if cn.State.Terminated == nil || cn.State.Terminated.Reason != "Completed" {
			log, err = p.K8sClient.GetPodLog(pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	return
}
