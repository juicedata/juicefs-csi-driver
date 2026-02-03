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
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	k8sMount "k8s.io/utils/mount"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

type PodMount struct {
	log klog.Logger
	k8sMount.SafeFormatAndMount
	K8sClient *k8sclient.K8sClient
}

var _ MntInterface = &PodMount{}

func NewPodMount(client *k8sclient.K8sClient, mounter k8sMount.SafeFormatAndMount) MntInterface {
	return &PodMount{
		klog.NewKlogr().WithName("pod-mount"),
		mounter, client}
}

func (p *PodMount) JMount(ctx context.Context, appInfo *jfsConfig.AppInfo, jfsSetting *jfsConfig.JfsSetting) error {
	p.log = util.GenLog(ctx, p.log, "JMount")
	hashVal := jfsConfig.GenHashOfSetting(p.log, *jfsSetting)
	jfsSetting.HashVal = hashVal
	jfsSetting.UpgradeUUID = string(uuid.NewUUID())
	var podName string
	var err error

	if err = func() error {
		lock := jfsConfig.GetPodLock(hashVal)
		lock.Lock()
		defer lock.Unlock()

		podName, err = p.genMountPodName(ctx, jfsSetting)
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
		return nil
	}(); err != nil {
		return err
	}

	err = p.waitUntilMountReady(ctx, jfsSetting, podName)
	if err != nil {
		return err
	}
	if jfsSetting.UUID == "" {
		// need set uuid as label in mount pod for clean cache
		uuid, err := p.GetJfsVolUUID(ctx, jfsSetting)
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
	log := util.GenLog(ctx, p.log, "GetMountRef")
	pod, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return 0, nil
		}
		log.Error(err, "Get mount pod error", "podName", podName)
		return 0, err
	}
	return GetRef(pod), nil
}

func (p *PodMount) UmountTarget(ctx context.Context, target, podName string) error {
	// targetPath may be mount bind many times when mount point recovered.
	// umount until it's not mounted.
	log := util.GenLog(ctx, p.log, "UmountTarget")
	log.Info("lazy umount", "target", target)
	for {
		command := exec.Command("umount", "-l", target)
		out, err := command.CombinedOutput()
		if err == nil {
			continue
		}
		log.V(1).Info(string(out))
		if !strings.Contains(string(out), "not mounted") &&
			!strings.Contains(string(out), "mountpoint not found") &&
			!strings.Contains(string(out), "no mount point specified") {
			log.Error(err, "Could not lazy unmount", "target", target, "out", string(out))
			return err
		}
		break
	}

	// cleanup target path
	if err := k8sMount.CleanupMountPoint(target, p.SafeFormatAndMount.Interface, false); err != nil {
		log.Info("Clean mount point error", "error", err)
		return err
	}
	return nil
}

func (p *PodMount) JUmount(ctx context.Context, target, podName string) error {
	log := util.GenLog(ctx, p.log, "JUmount")
	key := util.GetReferenceKey(target)
	log.Info("Delete target ref and check mount pod is need to delete or not.", "target", target, "podName", podName, "key", key)
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		po, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			log.Error(err, "Get mount pod err", "podName", podName)
			return err
		}

		if _, ok := po.Annotations[key]; ok {
			if err := resource.DelPodAnnotation(ctx, p.K8sClient, podName, jfsConfig.Namespace, []string{key}); err != nil {
				log.Error(err, "Remove pod annotation error", "podName", podName, "key", key)
				return err
			}
			delete(po.Annotations, key)
		}
		if GetRef(po) != 0 {
			log.Info("pod still has juicefs- refs.", "podName", podName)
			return nil
		}
		var shouldDelay bool
		shouldDelay, err = resource.ShouldDelay(ctx, po, p.K8sClient)
		if err != nil {
			return err
		}
		if !shouldDelay {
			// do not set delay delete, delete it now
			log.Info("pod has no juicefs- refs. delete it.", "podName", podName)
			if err := p.K8sClient.DeletePod(ctx, po); err != nil {
				log.Info("Delete pod error", "podName", podName, "error", err)
				return err
			}

			// close socket
			if util.SupportFusePass(po) {
				passfd.GlobalFds.StopFd(ctx, po)
			}

			// delete related secret
			secretName := po.Name + "-secret"
			log.V(1).Info("delete related secret of pod", "podName", podName, "secretName", secretName)
			if err := p.K8sClient.DeleteSecret(ctx, secretName, po.Namespace); !k8serrors.IsNotFound(err) && err != nil {
				// do not return err if delete secret failed
				log.V(1).Info("Delete secret error", "secretName", secretName, "error", err)
			}
		}
		return nil
	})

	return err
}

func (p *PodMount) JCreateVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	log := util.GenLog(ctx, p.log, "JCreateVolume")
	var exist *batchv1.Job
	r := builder.NewJobBuilder(jfsSetting, 0)
	job := r.NewJobForCreateVolume()
	exist, err := p.K8sClient.GetJob(ctx, job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		log.Info("create job", "jobName", job.Name)
		exist, err = p.K8sClient.CreateJob(ctx, job)
		if err != nil {
			log.Error(err, "create job err", "jobName", job.Name)
			return err
		}
	}
	if err != nil {
		log.Error(err, "get job err", "jobName", job.Name)
		return err
	}
	secret := r.NewSecret()
	builder.SetJobAsOwner(&secret, *exist)
	if err := resource.CreateOrUpdateSecret(ctx, p.K8sClient, &secret); err != nil {
		return err
	}
	err = p.waitUntilJobCompleted(ctx, job.Name)
	if err != nil {
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			log.Error(e, "delete job error", "jobName", job.Name)
		}
	}
	return err
}

func (p *PodMount) JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	log := util.GenLog(ctx, p.log, "JDeleteVolume")
	var exist *batchv1.Job
	r := builder.NewJobBuilder(jfsSetting, 0)
	job := r.NewJobForDeleteVolume()
	exist, err := p.K8sClient.GetJob(ctx, job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		log.Info("create job", "jobName", job.Name)
		exist, err = p.K8sClient.CreateJob(ctx, job)
		if err != nil {
			log.Error(err, "create job err", "jobName", job.Name)
			return err
		}
	}
	if err != nil {
		log.Error(err, "get job err", "jobName", job.Name)
		return err
	}
	secret := r.NewSecret()
	builder.SetJobAsOwner(&secret, *exist)
	if err := resource.CreateOrUpdateSecret(ctx, p.K8sClient, &secret); err != nil {
		return err
	}
	err = p.waitUntilJobCompleted(ctx, job.Name)
	if err != nil {
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			log.Error(e, "delete job error", "jobName", job.Name)
		}
	}
	return err
}

func (p *PodMount) genMountPodName(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) (string, error) {
	log := util.GenLog(ctx, p.log, "genMountPodName")
	labelSelector := &metav1.LabelSelector{MatchLabels: map[string]string{
		common.PodTypeKey:          common.PodTypeValue,
		common.PodUniqueIdLabelKey: jfsSetting.UniqueId,
	}}
	var fieldSelector *fields.Set
	if !jfsConfig.GlobalConfig.EnableNodeSelector {
		fieldSelector = &fields.Set{
			"spec.nodeName": jfsConfig.NodeName,
		}
	}
	pods, err := p.K8sClient.ListPod(ctx, jfsConfig.Namespace, labelSelector, fieldSelector)
	if err != nil {
		log.Error(err, "List pods of uniqueId", "uniqueId", jfsSetting.UniqueId, "hashVal", jfsSetting.HashVal)
		return "", err
	}
	var podName string
	for _, pod := range pods {
		po := pod
		if pod.Spec.NodeName != jfsConfig.NodeName && pod.Spec.NodeSelector["kubernetes.io/hostname"] != jfsConfig.NodeName {
			continue
		}
		hashMismatch := po.Labels[common.PodJuiceHashLabelKey] != jfsSetting.HashVal
		beingDeleted := po.DeletionTimestamp != nil
		podComplete := resource.IsPodComplete(&po)

		if hashMismatch || beingDeleted || podComplete {
			if hashMismatch {
				log.V(1).Info("reuse pod check: skipping pod due to hash mismatch", "podName", pod.Name, "expectedHash", jfsSetting.HashVal, "actualHash", po.Labels[common.PodJuiceHashLabelKey])
			}
			if beingDeleted {
				log.V(1).Info("reuse pod check: skipping pod due to deletion in progress", "podName", pod.Name, "deletionTimestamp", po.DeletionTimestamp)
			}
			if podComplete {
				log.V(1).Info("reuse pod check: skipping pod due to completion status", "podName", pod.Name)
			}
			for k, v := range po.Annotations {
				if v == jfsSetting.TargetPath {
					log.Info("Found pod with same target path, delete the reference", "podName", pod.Name, "targetPath", jfsSetting.TargetPath)
					if err := resource.DelPodAnnotation(ctx, p.K8sClient, po.Name, po.Namespace, []string{k}); err != nil {
						return "", err
					}
				}
			}
			continue
		}
		podName = pod.Name
	}
	if podName != "" {
		log.V(1).Info("reuse pod found", "podName", podName, "uniqueId", jfsSetting.UniqueId, "hashVal", jfsSetting.HashVal)
		return podName, nil
	}
	return GenPodNameByUniqueId(jfsSetting.UniqueId, true), nil
}

func (p *PodMount) createOrAddRef(ctx context.Context, podName string, jfsSetting *jfsConfig.JfsSetting, appinfo *jfsConfig.AppInfo) (err error) {
	log := util.GenLog(ctx, p.log, "createOrAddRef")
	log.V(1).Info("mount pod", "podName", podName)
	jfsSetting.MountPath = jfsSetting.MountPath + podName[len(podName)-7:]
	jfsSetting.SecretName = fmt.Sprintf("juicefs-%s-secret", jfsSetting.UniqueId)

	r := builder.NewPodBuilder(jfsSetting, 0)
	secret := r.NewSecret()
	builder.SetPVAsOwner(&secret, jfsSetting.PV)
	key := util.GetReferenceKey(jfsSetting.TargetPath)

	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	for {
		var (
			oldPod *corev1.Pod
		)
		// wait for old pod deleted
		oldPod, err = p.K8sClient.GetPod(waitCtx, podName, jfsConfig.Namespace)
		if err == nil && oldPod.DeletionTimestamp != nil {
			log.V(1).Info("wait for old mount pod deleted.", "podName", podName)
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			if k8serrors.IsNotFound(err) {
				// mkdir mountpath
				if err = util.MkdirIfNotExist(ctx, jfsSetting.MountPath); err != nil {
					return
				}
				// pod not exist, create
				log.Info("Need to create pod", "podName", podName)
				newPod, err := r.NewMountPod(podName)
				if err != nil {
					log.Error(err, "Make new mount pod error", "podName", podName)
					return err
				}
				newPod.Annotations[key] = jfsSetting.TargetPath
				if jfsConfig.GlobalConfig.EnableNodeSelector {
					nodeSelector := map[string]string{
						"kubernetes.io/hostname": newPod.Spec.NodeName,
					}
					nodes, err := p.K8sClient.ListNode(ctx, &metav1.LabelSelector{MatchLabels: nodeSelector})
					if err != nil || len(nodes) != 1 || nodes[0].Name != newPod.Spec.NodeName {
						log.Info("cannot select node by label selector", "nodeName", newPod.Spec.NodeName, "error", err)
					} else {
						newPod.Spec.NodeName = ""
						newPod.Spec.NodeSelector = nodeSelector
						if appinfo != nil && appinfo.Name != "" {
							appPod, err := p.K8sClient.GetPod(ctx, appinfo.Name, appinfo.Namespace)
							if err != nil {
								log.Info("get app pod", "namespace", appinfo.Namespace, "name", appinfo.Name, "error", err)
							} else {
								newPod.Spec.Affinity = appPod.Spec.Affinity
								newPod.Spec.SchedulerName = appPod.Spec.SchedulerName
								newPod.Spec.Tolerations = util.CopySlice(appPod.Spec.Tolerations)
							}
						}
					}
				}

				if err := resource.CreateOrUpdateSecret(ctx, p.K8sClient, &secret); err != nil {
					return err
				}

				supportFusePass := util.SupportFusePass(newPod)
				if supportFusePass {
					if err := passfd.GlobalFds.ServeFuseFd(ctx, newPod); err != nil {
						log.Error(err, "serve fuse fd error", "podName", podName)
					}
				} else {
					log.Info("mount pod cannot be smoothly upgraded. do not serve the FUSE fd.", "podName", podName)
				}

				_, err = p.K8sClient.CreatePod(ctx, newPod)
				if err != nil {
					log.Error(err, "Create pod err, stop fuse fd server", "podName", podName)
					if supportFusePass {
						passfd.GlobalFds.StopFd(ctx, newPod)
					}
				}
				return err
			} else if k8serrors.IsTimeout(err) {
				return fmt.Errorf("mount %v failed: mount pod %s deleting timeout", jfsSetting.VolumeId, podName)
			}
			// unexpect error
			log.Error(err, "Get pod err", "podName", podName)
			return err
		}
		// pod exist, add refs
		if err = resource.CreateOrUpdateSecret(ctx, p.K8sClient, &secret); err != nil {
			return err
		}
		// update mount path
		jfsSetting.MountPath, _, err = util.GetMountPathOfPod(*oldPod)
		if err != nil {
			log.Error(err, "Get mount path of pod error", "podName", podName)
			return err
		}

		// mkdir mountpath
		if err = util.MkdirIfNotExist(ctx, jfsSetting.MountPath); err != nil {
			return
		}
		return p.AddRefOfMount(ctx, jfsSetting.TargetPath, podName)
	}
}

func (p *PodMount) waitUntilMountReady(ctx context.Context, jfsSetting *jfsConfig.JfsSetting, podName string) error {
	logger := util.GenLog(ctx, p.log, "waitUntilMountReady")

	err := resource.WaitUntilPodRunning(ctx, p.K8sClient, podName, 1*time.Second)
	if err != nil {
		// if pod is not running until timeout, return error
		return err
	}

	err = resource.WaitUntilMountReady(ctx, podName, jfsSetting.MountPath, defaultCheckTimeout)
	if err == nil {
		return nil
	}
	logger.Error(err, "wait for mount error", "podName", podName)
	msg := fmt.Sprintf("wait for mount %s ready failed, err: %s", util.StripPasswd(jfsSetting.Source), err)
	// mountpoint not ready, get mount pod log for detail
	log, lerr := p.getErrContainerLog(ctx, podName)
	if lerr != nil {
		logger.Error(lerr, "Get pod log error", "podName", podName)
		return errors.New(msg)
	}
	if log != "" {
		msg += fmt.Sprintf(", log: %s", log)
	}
	return errors.New(msg)
}

func (p *PodMount) waitUntilJobCompleted(ctx context.Context, jobName string) error {
	log := util.GenLog(ctx, p.log, "waitUntilJobCompleted")
	// Wait until the job is completed
	waitCtx, waitCancel := context.WithTimeout(ctx, 40*time.Second)
	defer waitCancel()
	for {
		job, err := p.K8sClient.GetJob(waitCtx, jobName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("waitUntilJobCompleted: Job is completed and been recycled", "jobName", jobName)
				return nil
			}
			if waitCtx.Err() == context.DeadlineExceeded || waitCtx.Err() == context.Canceled {
				log.V(1).Info("job timeout", "jobName", jobName)
				break
			}
			return fmt.Errorf("waitUntilJobCompleted: Get job %v failed: %v", jobName, err)
		}
		if resource.IsJobCompleted(job) {
			log.Info("waitUntilJobCompleted: Job is completed", "jobName", jobName)
			if resource.IsJobShouldBeRecycled(job) {
				// try to delete job
				log.Info("job completed but not be recycled automatically, delete it", "jobName", jobName)
				if err := p.K8sClient.DeleteJob(ctx, jobName, jfsConfig.Namespace); err != nil {
					log.Error(err, "delete job error", "jobName", jobName)
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
		return fmt.Errorf("waitUntilJobCompleted: get pod from job %s error %v", jobName, err)
	}
	cnlog, err := p.getNotCompleteCnLog(ctx, pods[0].Name)
	if err != nil {
		return fmt.Errorf("waitUntilJobCompleted: get pod %s log error %v", pods[0].Name, err)
	}
	return fmt.Errorf("waitUntilJobCompleted: job %s isn't completed: %v", jobName, cnlog)
}

func (p *PodMount) AddRefOfMount(ctx context.Context, target string, podName string) error {
	log := util.GenLog(ctx, p.log, "AddRefOfMount")
	log.Info("Add target ref in mount pod.", "podName", podName, "target", target)
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
			log.Info("Target ref in pod already exists.", "target", target, "podName", podName)
			return nil
		}
		if annotation == nil {
			annotation = make(map[string]string)
		}
		annotation[key] = target
		// delete deleteDelayAt when there ars refs
		delete(annotation, common.DeleteDelayAtKey)
		return resource.ReplacePodAnnotation(ctx, p.K8sClient, podName, jfsConfig.Namespace, annotation)
	})
	if err != nil {
		log.Error(err, "Add target ref in mount pod error", "podName", podName)
		return err
	}
	return nil
}

func (p *PodMount) setUUIDAnnotation(ctx context.Context, podName string, uuid string) (err error) {
	logger := util.GenLog(ctx, p.log, "")
	logger.Info("set pod annotation", "podName", podName, "key", common.JuiceFSUUID, "uuid", uuid)
	return resource.AddPodAnnotation(ctx, p.K8sClient, podName, jfsConfig.Namespace, map[string]string{common.JuiceFSUUID: uuid})
}

func (p *PodMount) setMountLabel(ctx context.Context, uniqueId, mountPodName string, podName, podNamespace string) (err error) {
	logger := util.GenLog(ctx, p.log, "")
	logger.Info("set mount info in pod", "podName", podName)
	if err := resource.AddPodLabel(ctx, p.K8sClient, podName, podNamespace, map[string]string{common.UniqueId: ""}); err != nil {
		return err
	}

	return nil
}

// GetJfsVolUUID get UUID from result of `juicefs status <volumeName>`
func (p *PodMount) GetJfsVolUUID(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) (string, error) {
	log := util.GenLog(ctx, p.log, "GetJfsVolUUID")
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	statusCmd := p.Exec.CommandContext(cmdCtx, jfsConfig.CeCliPath, "status", jfsSetting.Source)
	envs := syscall.Environ()
	for key, val := range jfsSetting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", key, val))
	}
	statusCmd.SetEnv(envs)
	stdout, err := statusCmd.CombinedOutput()
	if err != nil {
		re := string(stdout)
		log.Info("juicefs status error", "error", err, "output", re)
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
		return "", fmt.Errorf("get uuid of %s error", jfsSetting.Source)
	}

	return idStrs[3], nil
}

func (p *PodMount) CleanCache(ctx context.Context, image string, id string, volumeId string, cacheDirs []string) error {
	log := util.GenLog(ctx, p.log, "CleanCache")
	jfsSetting, err := jfsConfig.ParseSetting(ctx, map[string]string{"name": id}, nil, []string{}, volumeId, volumeId, id, nil, nil)
	if err != nil {
		log.Error(err, "parse jfs setting err")
		return err
	}
	jfsSetting.Attr.Image = image
	jfsSetting.CacheDirs = cacheDirs
	r := builder.NewJobBuilder(jfsSetting, 0)
	job := r.NewJobForCleanCache()
	log.V(1).Info("Clean cache job", "jobName", job)
	_, err = p.K8sClient.GetJob(ctx, job.Name, job.Namespace)
	if err != nil && k8serrors.IsNotFound(err) {
		log.Info("create job", "jobName", job.Name)
		_, err = p.K8sClient.CreateJob(ctx, job)
	}
	if err != nil {
		log.Error(err, "get or create job err", "jobName", job.Name)
		return err
	}
	err = p.waitUntilJobCompleted(ctx, job.Name)
	if err != nil {
		log.Error(err, "wait for job completed err and fall back to delete job")
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			log.Error(e, "delete job %s error: %v", "jobName", job.Name)
		}
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
			if log != "" || err != nil {
				return
			}
		}
	}
	for _, cn := range pod.Status.ContainerStatuses {
		if !cn.Ready {
			log, err = p.K8sClient.GetPodLog(ctx, pod.Name, pod.Namespace, cn.Name)
			if log != "" || err != nil {
				return
			}
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
