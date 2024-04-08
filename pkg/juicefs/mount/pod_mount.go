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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	k8sMount "k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

type PodMount struct {
	k8sMount.SafeFormatAndMount
	K8sClient *k8sclient.K8sClient
}

var _ MntInterface = &PodMount{}

func NewPodMount(client *k8sclient.K8sClient, mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &PodMount{mounter, client}
}

func (p *PodMount) JMount(ctx context.Context, appInfo *jfsConfig.AppInfo, jfsSetting *jfsConfig.JfsSetting) error {
	podName, err := p.genMountPodName(ctx, jfsSetting)
	if err != nil {
		return err
	}

	// set mount pod name in app pod
	if appInfo != nil && appInfo.Name != "" && appInfo.Namespace != "" {
		err = p.setMountLabel(ctx, jfsSetting.UniqueId, podName, appInfo.Name, appInfo.Namespace)
		if err != nil {
			return err
		}
	}

	err = p.createOrAddRef(ctx, podName, jfsSetting, appInfo)
	if err != nil {
		return err
	}
	err = p.waitUtilMountReady(ctx, jfsSetting, podName)
	if err != nil {
		return err
	}
	if jfsSetting.CleanCache && jfsSetting.UUID == "" {
		// need set uuid as label in mount pod for clean cache
		uuid, err := p.GetJfsVolUUID(ctx, jfsSetting.Source)
		if err != nil {
			return err
		}
		err = p.setUUIDAnnotation(ctx, podName, uuid)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PodMount) GetMountRef(ctx context.Context, target, podName string) (int, error) {
	pod, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return 0, nil
		}
		klog.Errorf("JUmount: Get mount pod %s err %v", podName, err)
		return 0, err
	}
	return GetRef(pod), nil
}

func (p *PodMount) UmountTarget(ctx context.Context, target, podName string) error {
	// targetPath may be mount bind many times when mount point recovered.
	// umount until it's not mounted.
	klog.V(5).Infof("JfsUnmount: lazy umount %s", target)
	for {
		command := exec.Command("umount", "-l", target)
		out, err := command.CombinedOutput()
		if err == nil {
			continue
		}
		klog.V(6).Infoln(string(out))
		if !strings.Contains(string(out), "not mounted") &&
			!strings.Contains(string(out), "mountpoint not found") &&
			!strings.Contains(string(out), "no mount point specified") {
			klog.Errorf("Could not lazy unmount %q: %v, output: %s", target, err, string(out))
			return err
		}
		break
	}

	// cleanup target path
	if err := k8sMount.CleanupMountPoint(target, p.SafeFormatAndMount.Interface, false); err != nil {
		klog.V(5).Infof("Clean mount point error: %v", err)
		return err
	}

	// check mount pod is need to delete
	klog.V(5).Infof("JUmount: Delete target ref [%s] and check mount pod [%s] is need to delete or not.", target, podName)

	if podName == "" {
		// mount pod not exist
		klog.V(5).Infof("JUmount: Mount pod of target %s not exists.", target)
		return nil
	}
	pod, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
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
		po, err := p.K8sClient.GetPod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		annotation := po.Annotations
		if _, ok := annotation[key]; !ok {
			klog.V(5).Infof("JUmount: Target ref [%s] in pod [%s] already not exists.", target, pod.Name)
			return nil
		}
		return util.DelPodAnnotation(ctx, p.K8sClient, pod, []string{key})
	})
	if err != nil {
		klog.Errorf("JUmount: Remove ref of target %s err: %v", target, err)
		return err
	}
	return nil
}

func (p *PodMount) JUmount(ctx context.Context, target, podName string) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		po, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			klog.Errorf("JUmount: Get mount pod %s err %v", podName, err)
			return err
		}

		if GetRef(po) != 0 {
			klog.V(5).Infof("JUmount: pod %s still has juicefs- refs.", podName)
			return nil
		}

		var shouldDelay bool
		shouldDelay, err = util.ShouldDelay(ctx, po, p.K8sClient)
		if err != nil {
			return err
		}
		if !shouldDelay {
			// do not set delay delete, delete it now
			klog.V(5).Infof("JUmount: pod %s has no juicefs- refs. delete it.", podName)
			if err := p.K8sClient.DeletePod(ctx, po); err != nil {
				klog.V(5).Infof("JUmount: Delete pod %s error: %v", podName, err)
				return err
			}

			// delete related secret
			secretName := po.Name + "-secret"
			klog.V(5).Infof("JUmount: delete related secret of pod %s: %s", podName, secretName)
			if err := p.K8sClient.DeleteSecret(ctx, secretName, po.Namespace); err != nil {
				// do not return err if delete secret failed
				klog.V(5).Infof("JUmount: Delete secret %s error: %v", secretName, err)
			}
		}
		return nil
	})

	return err
}

func (p *PodMount) JCreateVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	var exist *batchv1.Job
	r := builder.NewJobBuilder(jfsSetting, 0)
	job := r.NewJobForCreateVolume()
	exist, err := p.K8sClient.GetJob(ctx, job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		klog.V(5).Infof("JCreateVolume: create job %s", job.Name)
		exist, err = p.K8sClient.CreateJob(ctx, job)
		if err != nil {
			klog.Errorf("JCreateVolume: create job %s err: %v", job.Name, err)
			return err
		}
	}
	if err != nil {
		klog.Errorf("JCreateVolume: get job %s err: %s", job.Name, err)
		return err
	}
	secret := r.NewSecret()
	builder.SetJobAsOwner(&secret, *exist)
	if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
		return err
	}
	err = p.waitUtilJobCompleted(ctx, job.Name)
	if err != nil {
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			klog.Errorf("JCreateVolume: delete job %s error: %v", job.Name, e)
		}
	}
	return err
}

func (p *PodMount) JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	var exist *batchv1.Job
	r := builder.NewJobBuilder(jfsSetting, 0)
	job := r.NewJobForDeleteVolume()
	exist, err := p.K8sClient.GetJob(ctx, job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		klog.V(5).Infof("JDeleteVolume: create job %s", job.Name)
		exist, err = p.K8sClient.CreateJob(ctx, job)
		if err != nil {
			klog.Errorf("JDeleteVolume: create job %s err: %v", job.Name, err)
			return err
		}
	}
	if err != nil {
		klog.Errorf("JDeleteVolume: get job %s err: %s", job.Name, err)
		return err
	}
	secret := r.NewSecret()
	builder.SetJobAsOwner(&secret, *exist)
	if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
		return err
	}
	err = p.waitUtilJobCompleted(ctx, job.Name)
	if err != nil {
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			klog.Errorf("JDeleteVolume: delete job %s error: %v", job.Name, e)
		}
	}
	return err
}

func (p *PodMount) genMountPodName(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) (string, error) {
	hashVal, err := GenHashOfSetting(*jfsSetting)
	if err != nil {
		klog.Errorf("Generate hash of jfsSetting error: %v", err)
		return "", err
	}

	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		jfsConfig.PodTypeKey:           jfsConfig.PodTypeValue,
		jfsConfig.PodUniqueIdLabelKey:  jfsSetting.UniqueId,
		jfsConfig.PodJuiceHashLabelKey: hashVal,
	}}
	pods, err := p.K8sClient.ListPod(ctx, jfsConfig.Namespace, labelSelector, nil)
	if err != nil {
		klog.Errorf("List pods of uniqueId %s and hash %s error: %v", jfsSetting.UniqueId, hashVal, err)
		return "", err
	}
	for _, pod := range pods {
		if pod.Spec.NodeName == jfsConfig.NodeName || pod.Spec.NodeSelector["kubernetes.io/hostname"] == jfsConfig.NodeName {
			if pod.DeletionTimestamp != nil {
				continue
			}
			return pod.Name, nil
		}
	}
	return GenPodNameByUniqueId(jfsSetting.UniqueId, true), nil
}

func (p *PodMount) createOrAddRef(ctx context.Context, podName string, jfsSetting *jfsConfig.JfsSetting, appinfo *jfsConfig.AppInfo) (err error) {
	klog.V(6).Infof("createOrAddRef: mount pod name %s", podName)
	hashVal, err := GenHashOfSetting(*jfsSetting)
	if err != nil {
		klog.Errorf("Generate hash of jfsSetting error: %v", err)
		return err
	}
	jfsSetting.MountPath = jfsSetting.MountPath + podName[len(podName)-7:]

	lock := jfsConfig.GetPodLock(podName)
	lock.Lock()
	defer lock.Unlock()

	jfsSetting.SecretName = podName + "-secret"
	r := builder.NewPodBuilder(jfsSetting, 0)
	secret := r.NewSecret()
	key := util.GetReferenceKey(jfsSetting.TargetPath)

	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	for {
		// wait for old pod deleted
		oldPod, err := p.K8sClient.GetPod(waitCtx, podName, jfsConfig.Namespace)
		if err == nil && oldPod.DeletionTimestamp != nil {
			klog.V(6).Infof("createOrAddRef: wait for old mount pod deleted.")
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			if k8serrors.IsNotFound(err) {
				// pod not exist, create
				klog.V(5).Infof("createOrAddRef: Need to create pod %s.", podName)
				newPod := r.NewMountPod(podName)
				newPod.Annotations[key] = jfsSetting.TargetPath
				newPod.Labels[jfsConfig.PodJuiceHashLabelKey] = hashVal
				if config.EnableNodeSelector {
					nodeSelector := map[string]string{
						"kubernetes.io/hostname": newPod.Spec.NodeName,
					}
					nodes, err := p.K8sClient.ListNode(ctx, &metav1.LabelSelector{MatchLabels: nodeSelector})
					if err != nil || len(nodes) != 1 || nodes[0].Name != newPod.Spec.NodeName {
						klog.Warningf("cannot select node %s by label selector: %v", newPod.Spec.NodeName, err)
					} else {
						newPod.Spec.NodeName = ""
						newPod.Spec.NodeSelector = nodeSelector
						if appinfo != nil && appinfo.Name != "" {
							appPod, err := p.K8sClient.GetPod(ctx, appinfo.Name, appinfo.Namespace)
							if err != nil {
								klog.Warningf("get app pod %s/%s: %v", appinfo.Namespace, appinfo.Name, err)
							} else {
								newPod.Spec.Affinity = appPod.Spec.Affinity
								newPod.Spec.SchedulerName = appPod.Spec.SchedulerName
								newPod.Spec.Tolerations = appPod.Spec.Tolerations
							}
						}
					}
				}

				if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
					return err
				}
				_, err = p.K8sClient.CreatePod(ctx, newPod)
				if err != nil {
					klog.Errorf("createOrAddRef: Create pod %s err: %v", podName, err)
				}
				return err
			} else if k8serrors.IsTimeout(err) {
				return fmt.Errorf("mount %v failed: mount pod %s deleting timeout", jfsSetting.VolumeId, podName)
			}
			// unexpect error
			klog.Errorf("createOrAddRef: Get pod %s err: %v", podName, err)
			return err
		}
		// pod exist, add refs
		if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
			return err
		}
		return p.AddRefOfMount(ctx, jfsSetting.TargetPath, podName)
	}
}

func (p *PodMount) waitUtilMountReady(ctx context.Context, jfsSetting *jfsConfig.JfsSetting, podName string) error {
	err := util.WaitUtilMountReady(ctx, podName, jfsSetting.MountPath, defaultCheckTimeout)
	if err == nil {
		return nil
	}
	// mountpoint not ready, get mount pod log for detail
	log, err := p.getErrContainerLog(ctx, podName)
	if err != nil {
		klog.Errorf("Get pod %s log error %v", podName, err)
		return fmt.Errorf("mount %v at %v failed: mount isn't ready in 30 seconds", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath)
	}
	return fmt.Errorf("mount %v at %v failed, mountpod: %s, failed log: %v", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath, podName, log)
}

func (p *PodMount) waitUtilJobCompleted(ctx context.Context, jobName string) error {
	// Wait until the job is completed
	waitCtx, waitCancel := context.WithTimeout(ctx, 40*time.Second)
	defer waitCancel()
	for {
		job, err := p.K8sClient.GetJob(waitCtx, jobName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				klog.Infof("waitUtilJobCompleted: Job %s is completed and been recycled", jobName)
				return nil
			}
			if waitCtx.Err() == context.DeadlineExceeded || waitCtx.Err() == context.Canceled {
				klog.V(6).Infof("job %s timeout", jobName)
				break
			}
			return fmt.Errorf("waitUtilJobCompleted: Get job %v failed: %v", jobName, err)
		}
		if util.IsJobCompleted(job) {
			klog.V(5).Infof("waitUtilJobCompleted: Job %s is completed", jobName)
			if util.IsJobShouldBeRecycled(job) {
				// try to delete job
				klog.Infof("job %s completed but not be recycled automatically, delete it", jobName)
				if err := p.K8sClient.DeleteJob(ctx, jobName, jfsConfig.Namespace); err != nil {
					klog.Errorf("delete job %s error %v", jobName, err)
				}
			}
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}

	pods, err := p.K8sClient.ListPod(ctx, jfsConfig.Namespace, &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"job-name": jobName,
		},
	}, nil)
	if err != nil || len(pods) == 0 {
		return fmt.Errorf("waitUtilJobCompleted: get pod from job %s error %v", jobName, err)
	}
	log, err := p.getNotCompleteCnLog(ctx, pods[0].Name)
	if err != nil {
		return fmt.Errorf("waitUtilJobCompleted: get pod %s log error %v", pods[0].Name, err)
	}
	return fmt.Errorf("waitUtilJobCompleted: job %s isn't completed: %v", jobName, log)
}

func (p *PodMount) AddRefOfMount(ctx context.Context, target string, podName string) error {
	klog.V(5).Infof("addRefOfMount: Add target ref in mount pod. mount pod: [%s], target: [%s]", podName, target)
	// add volumeId ref in pod annotation
	key := util.GetReferenceKey(target)

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		exist, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
		if err != nil {
			return err
		}
		if exist.DeletionTimestamp != nil {
			return fmt.Errorf("addRefOfMount: Mount pod [%s] has been deleted", podName)
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
		return util.ReplacePodAnnotation(ctx, p.K8sClient, exist, annotation)
	})
	if err != nil {
		klog.Errorf("addRefOfMount: Add target ref in mount pod %s error: %v", podName, err)
		return err
	}
	return nil
}

func (p *PodMount) setUUIDAnnotation(ctx context.Context, podName string, uuid string) (err error) {
	var pod *corev1.Pod
	pod, err = p.K8sClient.GetPod(context.Background(), podName, jfsConfig.Namespace)
	if err != nil {
		return err
	}
	klog.Infof("setUUIDAnnotation: set pod %s annotation %s=%s", podName, jfsConfig.JuiceFSUUID, uuid)
	return util.AddPodAnnotation(ctx, p.K8sClient, pod, map[string]string{jfsConfig.JuiceFSUUID: uuid})
}

func (p *PodMount) setMountLabel(ctx context.Context, uniqueId, mountPodName string, podName, podNamespace string) (err error) {
	var pod *corev1.Pod
	pod, err = p.K8sClient.GetPod(context.Background(), podName, podNamespace)
	if err != nil {
		return err
	}
	klog.Infof("setMountLabel: set mount info in pod %s", podName)
	if err := util.AddPodLabel(ctx, p.K8sClient, pod, map[string]string{jfsConfig.UniqueId: ""}); err != nil {
		return err
	}

	return nil
}

// GetJfsVolUUID get UUID from result of `juicefs status <volumeName>`
func (p *PodMount) GetJfsVolUUID(ctx context.Context, name string) (string, error) {
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	stdout, err := p.Exec.CommandContext(cmdCtx, jfsConfig.CeCliPath, "status", name).CombinedOutput()
	if err != nil {
		re := string(stdout)
		klog.Infof("juicefs status error: %v, output: '%s'", err, re)
		if cmdCtx.Err() == context.DeadlineExceeded {
			re = fmt.Sprintf("juicefs status %s timed out", 8*defaultCheckTimeout)
			return "", errors.New(re)
		}
		return "", errors.Wrap(err, re)
	}

	matchExp := regexp.MustCompile(`"UUID": "(.*)"`)
	idStr := matchExp.FindString(string(stdout))
	idStrs := strings.Split(idStr, "\"")
	if len(idStrs) < 4 {
		return "", fmt.Errorf("get uuid of %s error", name)
	}

	return idStrs[3], nil
}

func (p *PodMount) CleanCache(ctx context.Context, image string, id string, volumeId string, cacheDirs []string) error {
	jfsSetting, err := jfsConfig.ParseSetting(map[string]string{"name": id}, nil, []string{}, true)
	if err != nil {
		klog.Errorf("CleanCache: parse jfs setting err: %v", err)
		return err
	}
	jfsSetting.Attr.Image = image
	jfsSetting.VolumeId = volumeId
	jfsSetting.CacheDirs = cacheDirs
	jfsSetting.UUID = id
	r := builder.NewJobBuilder(jfsSetting, 0)
	job := r.NewJobForCleanCache()
	klog.V(6).Infof("Clean cache job: %v", job)
	_, err = p.K8sClient.GetJob(ctx, job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		klog.V(5).Infof("CleanCache: create job %s", job.Name)
		_, err = p.K8sClient.CreateJob(ctx, job)
	}
	if err != nil {
		klog.Errorf("CleanCache: get or create job %s err: %s", job.Name, err)
		return err
	}
	err = p.waitUtilJobCompleted(ctx, job.Name)
	if err != nil {
		klog.Errorf("CleanCache: wait for job completed err and fall back to delete job\n %v", err)
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			klog.Errorf("CleanCache: delete job %s error: %v", job.Name, e)
		}
	}
	return nil
}

func (p *PodMount) createOrUpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	klog.V(5).Infof("createOrUpdateSecret: %s, %s", secret.Name, secret.Namespace)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		oldSecret, err := p.K8sClient.GetSecret(ctx, secret.Name, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				// secret not exist, create
				_, err := p.K8sClient.CreateSecret(ctx, secret)
				return err
			}
			// unexpected err
			return err
		}

		oldSecret.StringData = secret.StringData
		return p.K8sClient.UpdateSecret(ctx, oldSecret)
	})
	if err != nil {
		klog.Errorf("createOrUpdateSecret: secret %s: %v", secret.Name, err)
		return err
	}
	return nil
}

func (p *PodMount) getErrContainerLog(ctx context.Context, podName string) (log string, err error) {
	pod, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
	if err != nil {
		return
	}
	for _, cn := range pod.Status.InitContainerStatuses {
		if !cn.Ready {
			log, err = p.K8sClient.GetPodLog(ctx, pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	for _, cn := range pod.Status.ContainerStatuses {
		if !cn.Ready {
			log, err = p.K8sClient.GetPodLog(ctx, pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	return
}

func (p *PodMount) getNotCompleteCnLog(ctx context.Context, podName string) (log string, err error) {
	pod, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
	if err != nil {
		return
	}
	for _, cn := range pod.Status.InitContainerStatuses {
		if cn.State.Terminated == nil || cn.State.Terminated.Reason != "Completed" {
			log, err = p.K8sClient.GetPodLog(ctx, pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	for _, cn := range pod.Status.ContainerStatuses {
		if cn.State.Terminated == nil || cn.State.Terminated.Reason != "Completed" {
			log, err = p.K8sClient.GetPodLog(ctx, pod.Name, pod.Namespace, cn.Name)
			return
		}
	}
	return
}

func GetRef(pod *corev1.Pod) int {
	res := 0
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			res++
		}
	}
	return res
}

func GenPodNameByUniqueId(uniqueId string, withRandom bool) string {
	if !withRandom {
		return fmt.Sprintf("juicefs-%s-%s", jfsConfig.NodeName, uniqueId)
	}
	return fmt.Sprintf("juicefs-%s-%s-%s", jfsConfig.NodeName, uniqueId, util.RandStringRunes(6))
}

func GenHashOfSetting(setting jfsConfig.JfsSetting) (string, error) {
	// target path should not affect hash val
	setting.TargetPath = ""
	setting.VolumeId = ""
	setting.SubPath = ""
	settingStr, err := json.Marshal(setting)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(settingStr)
	val := hex.EncodeToString(h.Sum(nil))[:63]
	klog.V(6).Infof("jfsSetting hash: %s", val)
	return val, nil
}
