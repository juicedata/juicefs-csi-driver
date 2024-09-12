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
	"os"
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
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	k8sMount "k8s.io/utils/mount"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/security"
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
	hashVal := GenHashOfSetting(p.log, *jfsSetting)
	jfsSetting.HashVal = hashVal
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

	err = p.waitUtilMountReady(ctx, jfsSetting, podName)
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
	log := util.GenLog(ctx, p.log, "")
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

	// check mount pod is need to delete
	log.Info("Delete target ref and check mount pod is need to delete or not.", "target", target, "podName", podName)

	if podName == "" {
		// mount pod not exist
		log.Info("Mount pod of target not exists.", "target", target)
		return nil
	}
	pod, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		log.Error(err, "Get pod err", "podName", podName)
		return err
	}

	// if mount pod not exists.
	if pod == nil {
		log.Info("Mount pod not exists", "podName", podName)
		return nil
	}

	key := util.GetReferenceKey(target)
	log.V(1).Info("Target hash of target", "target", target, "key", key)

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		po, err := p.K8sClient.GetPod(ctx, pod.Name, pod.Namespace)
		if err != nil {
			return err
		}
		annotation := po.Annotations
		if _, ok := annotation[key]; !ok {
			log.Info("Target ref in pod already not exists.", "target", target, "podName", pod.Name)
			return nil
		}
		return resource.DelPodAnnotation(ctx, p.K8sClient, pod, []string{key})
	})
	if err != nil {
		log.Error(err, "Remove ref of target err", "target", target)
		return err
	}
	return nil
}

func (p *PodMount) JUmount(ctx context.Context, target, podName string) error {
	log := util.GenLog(ctx, p.log, "JUmount")
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		po, err := p.K8sClient.GetPod(ctx, podName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			log.Error(err, "Get mount pod err", "podName", podName)
			return err
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
			if util.SupportFusePass(po.Spec.Containers[0].Image) {
				passfd.GlobalFds.StopFd(ctx, po.Labels[jfsConfig.PodJuiceHashLabelKey])
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
	if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
		return err
	}
	err = p.waitUtilJobCompleted(ctx, job.Name)
	if err != nil {
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			log.Error(e, "delete job error", "jobName", job.Name)
		}
	}
	return err
}

func (p *PodMount) JDeleteVolume(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) error {
	log := p.log.WithName("JDeleteVolume")
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
	if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
		return err
	}
	err = p.waitUtilJobCompleted(ctx, job.Name)
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
		jfsConfig.PodTypeKey:           jfsConfig.PodTypeValue,
		jfsConfig.PodUniqueIdLabelKey:  jfsSetting.UniqueId,
		jfsConfig.PodJuiceHashLabelKey: jfsSetting.HashVal,
	}}
	pods, err := p.K8sClient.ListPod(ctx, jfsConfig.Namespace, labelSelector, nil)
	if err != nil {
		log.Error(err, "List pods of uniqueId", "uniqueId", jfsSetting.UniqueId, "hashVal", jfsSetting.HashVal)
		return "", err
	}
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Spec.NodeName == jfsConfig.NodeName || pod.Spec.NodeSelector["kubernetes.io/hostname"] == jfsConfig.NodeName {
			return pod.Name, nil
		}
	}
	return GenPodNameByUniqueId(jfsSetting.UniqueId, true), nil
}

func (p *PodMount) createOrAddRef(ctx context.Context, podName string, jfsSetting *jfsConfig.JfsSetting, appinfo *jfsConfig.AppInfo) (err error) {
	log := p.log.WithName("createOrAddRef")
	log.V(1).Info("mount pod", "podName", podName)
	jfsSetting.MountPath = jfsSetting.MountPath + podName[len(podName)-7:]
	jfsSetting.SecretName = fmt.Sprintf("juicefs-%s-secret", jfsSetting.UniqueId)
	// mkdir mountpath
	err = util.DoWithTimeout(ctx, 3*time.Second, func() error {
		exist, _ := k8sMount.PathExists(jfsSetting.MountPath)
		if !exist {
			return os.MkdirAll(jfsSetting.MountPath, 0777)
		}
		return nil
	})
	if err != nil {
		return
	}

	r := builder.NewPodBuilder(jfsSetting, 0)
	secret := r.NewSecret()
	builder.SetPVAsOwner(&secret, jfsSetting.PV)
	key := util.GetReferenceKey(jfsSetting.TargetPath)

	waitCtx, waitCancel := context.WithTimeout(ctx, 60*time.Second)
	defer waitCancel()
	for {
		// wait for old pod deleted
		oldPod, err := p.K8sClient.GetPod(waitCtx, podName, jfsConfig.Namespace)
		if err == nil && oldPod.DeletionTimestamp != nil {
			log.V(1).Info("wait for old mount pod deleted.", "podName", podName)
			time.Sleep(time.Millisecond * 500)
			continue
		} else if err != nil {
			if k8serrors.IsNotFound(err) {
				// pod not exist, create
				log.Info("Need to create pod", "podName", podName)
				newPod, err := r.NewMountPod(podName)
				if err != nil {
					log.Error(err, "Make new mount pod error", "podName", podName)
					return err
				}
				newPod.Annotations[key] = jfsSetting.TargetPath
				newPod.Labels[jfsConfig.PodJuiceHashLabelKey] = jfsSetting.HashVal
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
								newPod.Spec.Tolerations = appPod.Spec.Tolerations
							}
						}
					}
				}

				if util.SupportFusePass(jfsSetting.Attr.Image) {
					if err := passfd.GlobalFds.ServeFuseFd(ctx, newPod.Labels[jfsConfig.PodJuiceHashLabelKey]); err != nil {
						log.Error(err, "serve fuse fd error")
					}
				}

				if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
					return err
				}
				_, err = p.K8sClient.CreatePod(ctx, newPod)
				if err != nil {
					log.Error(err, "Create pod err", "podName", podName)
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
		if err := p.createOrUpdateSecret(ctx, &secret); err != nil {
			return err
		}
		// update mount path
		jfsSetting.MountPath, _, err = util.GetMountPathOfPod(*oldPod)
		if err != nil {
			log.Error(err, "Get mount path of pod error", "podName", podName)
			return err
		}
		return p.AddRefOfMount(ctx, jfsSetting.TargetPath, podName)
	}
}

func (p *PodMount) waitUtilMountReady(ctx context.Context, jfsSetting *jfsConfig.JfsSetting, podName string) error {
	logger := util.GenLog(ctx, p.log, "")
	err := resource.WaitUtilMountReady(ctx, podName, jfsSetting.MountPath, defaultCheckTimeout)
	if err == nil {
		return nil
	}
	if util.SupportFusePass(jfsSetting.Attr.Image) {
		logger.Error(err, "pod is not ready within 60s")
		// mount pod hang probably, close fd
		logger.Info("close fuse fd")
		passfd.GlobalFds.CloseFd(jfsSetting.HashVal)
		// umount it
		_ = util.DoWithTimeout(ctx, defaultCheckTimeout, func() error {
			util.UmountPath(ctx, jfsSetting.MountPath)
			return nil
		})
	}
	// mountpoint not ready, get mount pod log for detail
	log, err := p.getErrContainerLog(ctx, podName)
	if err != nil {
		logger.Error(err, "Get pod log error", "podName", podName)
		return fmt.Errorf("mount %v at %v failed: mount isn't ready in 30 seconds", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath)
	}
	return fmt.Errorf("mount %v at %v failed, mountpod: %s, failed log: %v", util.StripPasswd(jfsSetting.Source), jfsSetting.MountPath, podName, log)
}

func (p *PodMount) waitUtilJobCompleted(ctx context.Context, jobName string) error {
	log := p.log.WithName("waitUtilJobCompleted")
	// Wait until the job is completed
	waitCtx, waitCancel := context.WithTimeout(ctx, 40*time.Second)
	defer waitCancel()
	for {
		job, err := p.K8sClient.GetJob(waitCtx, jobName, jfsConfig.Namespace)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				log.Info("waitUtilJobCompleted: Job is completed and been recycled", "jobName", jobName)
				return nil
			}
			if waitCtx.Err() == context.DeadlineExceeded || waitCtx.Err() == context.Canceled {
				log.V(1).Info("job timeout", "jobName", jobName)
				break
			}
			return fmt.Errorf("waitUtilJobCompleted: Get job %v failed: %v", jobName, err)
		}
		if resource.IsJobCompleted(job) {
			log.Info("waitUtilJobCompleted: Job is completed", "jobName", jobName)
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
		return fmt.Errorf("waitUtilJobCompleted: get pod from job %s error %v", jobName, err)
	}
	cnlog, err := p.getNotCompleteCnLog(ctx, pods[0].Name)
	if err != nil {
		return fmt.Errorf("waitUtilJobCompleted: get pod %s log error %v", pods[0].Name, err)
	}
	return fmt.Errorf("waitUtilJobCompleted: job %s isn't completed: %v", jobName, cnlog)
}

func (p *PodMount) AddRefOfMount(ctx context.Context, target string, podName string) error {
	log := p.log.WithName("AddRefOfMount")
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
		delete(annotation, jfsConfig.DeleteDelayAtKey)
		return resource.ReplacePodAnnotation(ctx, p.K8sClient, exist, annotation)
	})
	if err != nil {
		log.Error(err, "Add target ref in mount pod error", "podName", podName)
		return err
	}
	return nil
}

func (p *PodMount) setUUIDAnnotation(ctx context.Context, podName string, uuid string) (err error) {
	logger := util.GenLog(ctx, p.log, "")
	var pod *corev1.Pod
	pod, err = p.K8sClient.GetPod(context.Background(), podName, jfsConfig.Namespace)
	if err != nil {
		return err
	}
	logger.Info("set pod annotation", "podName", podName, "key", jfsConfig.JuiceFSUUID, "uuid", uuid)
	return resource.AddPodAnnotation(ctx, p.K8sClient, pod, map[string]string{jfsConfig.JuiceFSUUID: uuid})
}

func (p *PodMount) setMountLabel(ctx context.Context, uniqueId, mountPodName string, podName, podNamespace string) (err error) {
	logger := util.GenLog(ctx, p.log, "")
	var pod *corev1.Pod
	pod, err = p.K8sClient.GetPod(context.Background(), podName, podNamespace)
	if err != nil {
		return err
	}
	logger.Info("set mount info in pod", "podName", podName)
	if err := resource.AddPodLabel(ctx, p.K8sClient, pod, map[string]string{jfsConfig.UniqueId: ""}); err != nil {
		return err
	}

	return nil
}

// GetJfsVolUUID get UUID from result of `juicefs status <volumeName>`
func (p *PodMount) GetJfsVolUUID(ctx context.Context, jfsSetting *jfsConfig.JfsSetting) (string, error) {
	log := util.GenLog(ctx, p.log, "")
	cmdCtx, cmdCancel := context.WithTimeout(ctx, 8*defaultCheckTimeout)
	defer cmdCancel()
	statusCmd := p.Exec.CommandContext(cmdCtx, jfsConfig.CeCliPath, "status", jfsSetting.Source)
	envs := syscall.Environ()
	for key, val := range jfsSetting.Envs {
		envs = append(envs, fmt.Sprintf("%s=%s", security.EscapeBashStr(key), security.EscapeBashStr(val)))
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
	log := p.log.WithName("CleanCache")
	jfsSetting, err := jfsConfig.ParseSetting(map[string]string{"name": id}, nil, []string{}, true, nil, nil)
	if err != nil {
		log.Error(err, "parse jfs setting err")
		return err
	}
	jfsSetting.Attr.Image = image
	jfsSetting.VolumeId = volumeId
	jfsSetting.CacheDirs = cacheDirs
	jfsSetting.UUID = id
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
	err = p.waitUtilJobCompleted(ctx, job.Name)
	if err != nil {
		log.Error(err, "wait for job completed err and fall back to delete job")
		// fall back if err
		if e := p.K8sClient.DeleteJob(ctx, job.Name, job.Namespace); e != nil {
			log.Error(e, "delete job %s error: %v", "jobName", job.Name)
		}
	}
	return nil
}

func (p *PodMount) createOrUpdateSecret(ctx context.Context, secret *corev1.Secret) error {
	log := p.log.WithName("createOrUpdateSecret")
	log.Info("secret", "name", secret.Name, "namespace", secret.Namespace)
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
		oldSecret.Data = nil
		oldSecret.StringData = secret.StringData
		// merge owner reference
		if len(secret.OwnerReferences) != 0 {
			newOwner := secret.OwnerReferences[0]
			exist := false
			for _, ref := range oldSecret.OwnerReferences {
				if ref.UID == newOwner.UID {
					exist = true
					break
				}
			}
			if !exist {
				oldSecret.OwnerReferences = append(oldSecret.OwnerReferences, newOwner)
			}
		}
		return p.K8sClient.UpdateSecret(ctx, oldSecret)
	})
	if err != nil {
		log.Error(err, "create or update secret error", "secretName", secret.Name)
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

func GenHashOfSetting(log klog.Logger, setting jfsConfig.JfsSetting) string {
	// target path should not affect hash val
	setting.TargetPath = ""
	setting.VolumeId = ""
	setting.SubPath = ""
	settingStr, _ := json.Marshal(setting)
	h := sha256.New()
	h.Write(settingStr)
	val := hex.EncodeToString(h.Sum(nil))[:63]
	log.V(1).Info("get jfsSetting hash", "hashVal", val)
	return val
}
