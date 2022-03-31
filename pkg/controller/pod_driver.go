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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/utils/mount"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PodDriver struct {
	Client   *k8sclient.K8sClient
	handlers map[podStatus]podHandler
	mit      *mountInfoTable
	mount.SafeFormatAndMount
}

func NewPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount) *PodDriver {
	return newPodDriver(client, mounter)
}

func newPodDriver(client *k8sclient.K8sClient, mounter mount.SafeFormatAndMount) *PodDriver {
	driver := &PodDriver{
		Client:             client,
		handlers:           map[podStatus]podHandler{},
		mit:                newMountInfoTable(),
		SafeFormatAndMount: mounter,
	}
	driver.handlers[podReady] = driver.podReadyHandler
	driver.handlers[podError] = driver.podErrorHandler
	driver.handlers[podPending] = driver.podPendingHandler
	driver.handlers[podDeleted] = driver.podDeletedHandler
	return driver
}

type podHandler func(ctx context.Context, pod *corev1.Pod) error
type podStatus string

const (
	podReady   podStatus = "podReady"
	podError   podStatus = "podError"
	podDeleted podStatus = "podDeleted"
	podPending podStatus = "podPending"
)

func (p *PodDriver) Run(ctx context.Context, current *corev1.Pod) error {
	// check refs in mount pod annotation first, delete ref that target pod is not found
	err := p.checkAnnotations(current)
	if apierrors.IsConflict(err) {
		current, err = p.Client.GetPod(current.Name, current.Namespace)
		if err != nil {
			return err // temporary
		}
		err = p.checkAnnotations(current)
	}
	if err != nil {
		klog.Errorf("check pod %s annotations err: %v", current.Name, err)
		return err
	}

	podStatus := p.getPodStatus(current)
	if podStatus != podError && podStatus != podDeleted {
		return p.handlers[podStatus](ctx, current)
	}

	// resourceVersion of kubelet may be different from apiserver
	// so we need get latest pod resourceVersion from apiserver
	pod, err := p.Client.GetPod(current.Name, current.Namespace)
	if err != nil {
		return err
	}
	// set mount pod status in mit again, maybe deleted
	p.mit.setPodStatus(pod)
	return p.handlers[p.getPodStatus(pod)](ctx, pod)
}

func (p *PodDriver) getPodStatus(pod *corev1.Pod) podStatus {
	if pod == nil {
		return podError
	}
	if pod.DeletionTimestamp != nil {
		return podDeleted
	}
	if util.IsPodError(pod) {
		return podError
	}
	if util.IsPodReady(pod) {
		return podReady
	}
	return podPending
}

func (p *PodDriver) checkAnnotations(pod *corev1.Pod) error {
	// check refs in mount pod, the corresponding pod exists or not
	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	annotation := make(map[string]string)
	var existTargets int
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			deleted, exists := p.mit.deletedPods[getPodUid(target)]
			if deleted || !exists {
				// target pod is deleted
				continue
			}
			existTargets++
		}
		annotation[k] = target
	}

	if existTargets != 0 {
		delete(annotation, config.DeleteDelayAtKey)
	}
	if len(pod.Annotations) != len(annotation) {
		if err := util.PatchPodAnnotation(p.Client, pod, annotation); err != nil {
			klog.Errorf("Update pod %s error: %v", pod.Name, err)
			return err
		}
	}
	if existTargets == 0 && pod.DeletionTimestamp == nil {
		var shouldDelay bool
		shouldDelay, err := util.ShouldDelay(pod, p.Client)
		if err != nil {
			return err
		}
		if !shouldDelay {
			// if there are no refs or after delay time, delete it
			klog.V(5).Infof("There are no refs in pod %s annotation, delete it", pod.Name)
			if err := p.Client.DeletePod(pod); err != nil {
				klog.Errorf("Delete pod %s error: %v", pod.Name, err)
				return err
			}
			// delete related secret
			secretName := pod.Name + "-secret"
			klog.V(6).Infof("delete related secret of pod: %s", secretName)
			if err := p.Client.DeleteSecret(secretName, pod.Namespace); err != nil {
				klog.V(5).Infof("Delete secret %s error: %v", secretName, err)
			}
		}
	}
	return nil
}

func (p *PodDriver) podErrorHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}
	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// check resource err
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("waitUtilMount: Pod is failed because of resource.")
		if util.IsPodHasResource(*pod) {
			// if pod is failed because of resource, delete resource and deploy pod again.
			_ = p.removeFinalizer(pod)
			klog.V(5).Infof("Delete it and deploy again with no resource.")
			if err := p.Client.DeletePod(pod); err != nil {
				klog.Errorf("delete po:%s err:%v", pod.Name, err)
				return nil
			}
			isDeleted := false
			// wait pod delete for 1min
			for i := 0; i < 120; i++ {
				_, err := p.Client.GetPod(pod.Name, pod.Namespace)
				if err == nil {
					klog.V(6).Infof("pod %s %s still exists wait.", pod.Name, pod.Namespace)
					time.Sleep(time.Microsecond * 500)
					continue
				}
				if apierrors.IsNotFound(err) {
					isDeleted = true
					break
				}
				klog.Errorf("get mountPod err:%v", err)
			}
			if !isDeleted {
				klog.Errorf("Old pod %s %s can't be deleted within 1min.", pod.Name, config.Namespace)
				return nil
			}
			var newPod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        pod.Name,
					Namespace:   pod.Namespace,
					Labels:      pod.Labels,
					Annotations: pod.Annotations,
				},
				Spec: pod.Spec,
			}
			controllerutil.AddFinalizer(newPod, config.Finalizer)
			util.DeleteResourceOfPod(newPod)
			_, err := p.Client.CreatePod(newPod)
			if err != nil {
				klog.Errorf("create pod:%s err:%v", pod.Name, err)
			}
		} else {
			klog.V(5).Infof("mountPod PodResourceError, but pod no resource, do nothing.")
		}
	}
	return nil
}

func (p *PodDriver) podDeletedHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		klog.Errorf("get nil pod")
		return nil
	}
	klog.V(5).Infof("Pod %s in namespace %s is to be deleted.", pod.Name, pod.Namespace)

	// pod with no finalizer
	if !util.ContainsString(pod.GetFinalizers(), config.Finalizer) {
		// do nothing
		return nil
	}

	// pod with resource error
	if util.IsPodResourceError(pod) {
		klog.V(5).Infof("The pod is PodResourceError, podDeletedHandler skip delete the pod:%s", pod.Name)
		return nil
	}

	// remove finalizer of pod
	if err := p.removeFinalizer(pod); err != nil {
		klog.Errorf("remove pod finalizer err:%v", err)
		return err
	}

	// get mount point
	sourcePath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return nil
	}

	// check if it needs to create new one
	klog.V(6).Infof("Annotations:%v", pod.Annotations)
	if pod.Annotations == nil {
		return nil
	}
	annotation := pod.Annotations
	existTargets := make(map[string]string)

	for k, v := range pod.Annotations {
		// annotation is checked in beginning, don't double-check here
		if k == util.GetReferenceKey(v) {
			existTargets[k] = v
		}
	}

	if len(existTargets) == 0 {
		// do not need to create new one, umount
		umountPath(ctx, sourcePath)
		// clean mount point
		_, err = doWithinTime(ctx, nil, func() error {
			klog.V(5).Infof("Clean mount point : %s", sourcePath)
			return mount.CleanupMountPoint(sourcePath, p.SafeFormatAndMount.Interface, false)
		})
		if err != nil {
			klog.Errorf("Clean mount point %s error: %v", sourcePath, err)
		}
		return nil
	}

	lock := config.GetPodLock(pod.Name)
	lock.Lock()
	defer lock.Unlock()

	// create
	klog.V(5).Infof("pod targetPath not empty, need create pod:%s", pod.Name)

	// check pod delete
	for i := 0; i < 120; i++ {
		po, err := p.Client.GetPod(pod.Name, pod.Namespace)
		if err == nil && po.DeletionTimestamp != nil {
			klog.V(6).Infof("pod %s %s is being deleted, waiting", pod.Name, pod.Namespace)
			time.Sleep(time.Millisecond * 500)
			continue
		}
		if err != nil {
			if apierrors.IsNotFound(err) {
				// umount mount point before recreate mount pod
				_, err := doWithinTime(ctx, nil, func() error {
					exist, _ := mount.PathExists(sourcePath)
					if !exist {
						return fmt.Errorf("%s not exist", sourcePath)
					}
					return nil
				})
				if err != nil {
					klog.Infof("start to umount: %s", sourcePath)
					umountPath(ctx, sourcePath)
				}
				// create pod
				var newPod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        pod.Name,
						Namespace:   pod.Namespace,
						Labels:      pod.Labels,
						Annotations: annotation,
					},
					Spec: pod.Spec,
				}
				controllerutil.AddFinalizer(newPod, config.Finalizer)
				klog.Infof("Need to create pod %s %s", pod.Name, pod.Namespace)
				_, err = p.Client.CreatePod(newPod)
				if err != nil {
					klog.Errorf("Create pod:%s err:%v", pod.Name, err)
				}
				return nil
			}
			klog.Errorf("Get pod err:%v", err)
			return nil
		}

		// pod is created elsewhere
		if po.Annotations == nil {
			po.Annotations = make(map[string]string)
		}
		for k, v := range existTargets {
			// add exist target in annotation
			po.Annotations[k] = v
		}
		if err := util.PatchPodAnnotation(p.Client, pod, annotation); err != nil {
			klog.Errorf("Update pod %s %s error: %v", po.Name, po.Namespace, err)
		}
		return err
	}

	klog.V(5).Infof("Old pod %s %s can't be deleted within 1min.", pod.Name, config.Namespace)
	return fmt.Errorf("old pod %s %s can't be deleted within 1min", pod.Name, config.Namespace)
}

func (p *PodDriver) podPendingHandler(ctx context.Context, pod *corev1.Pod) error {
	// do nothing
	return nil
}

func (p *PodDriver) podReadyHandler(ctx context.Context, pod *corev1.Pod) error {
	if pod == nil {
		klog.Errorf("[podReadyHandler] get nil pod")
		return nil
	}

	if pod.Annotations == nil {
		return nil
	}
	// get mount point
	mntPath, _, err := util.GetMountPathOfPod(*pod)
	if err != nil {
		klog.Error(err)
		return nil
	}

	_, e := doWithinTime(ctx, nil, func() error {
		_, e := os.Stat(mntPath)
		return e
	})

	if e != nil {
		klog.Errorf("[podReadyHandler] stat mntPath:%s err:%v, don't do recovery", mntPath, err)
		return nil
	}

	// recovery for each target
	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			mi := p.mit.resolveTarget(target)
			if mi == nil {
				klog.Errorf("pod %s target %s resolve fail", pod.Name, target)
				continue
			}

			p.recoverTarget(pod.Name, mntPath, mi.baseTarget, mi)
			for _, ti := range mi.subPathTarget {
				p.recoverTarget(pod.Name, mntPath, ti, mi)
			}
		}
	}

	return nil
}

func (p *PodDriver) recoverTarget(podName, sourcePath string, ti *targetItem, mi *mountItem) {
	switch ti.status {
	case targetStatusNotExist:
		klog.Errorf("pod %s target %s not exists, item count:%d", podName, ti.target, ti.count)
		if ti.count > 0 {
			// target exist in /proc/self/mountinfo file
			// refer to this case: local target exist, but source which target binded has beed deleted
			// if target is for pod subpath (volumeMount.subpath), this will cause user pod delete failed, so we help kubelet umount it
			if mi.podDeleted {
				p.umountTarget(ti.target, ti.count)
			}
		}

	case targetStatusMounted:
		// normal, most likely happen
		klog.V(6).Infof("pod %s target %s is normal mounted", podName, ti.target)

	case targetStatusNotMount:
		klog.V(5).Infof("pod %s target %s is not mounted", podName, ti.target)

	case targetStatusCorrupt:
		if ti.inconsistent {
			// source paths (found in /proc/self/mountinfo) which target binded is inconsistent
			// some unexpected things happened
			klog.Errorf("pod %s target %s, source inconsistent", podName, ti.target)
			break
		}
		if mi.podDeleted {
			klog.V(6).Infof("pod %s target %s, user pod has been deleted, don't do recovery", podName, ti.target)
			break
		}
		// if not umountTarget, mountinfo file will increase unlimited
		// if we umount all the target items, `mountPropagation` will lose efficacy
		p.umountTarget(ti.target, ti.count-1)
		if ti.subpath != "" {
			sourcePath += "/" + ti.subpath
			_, err := os.Stat(sourcePath)
			if err != nil {
				klog.Errorf("pod %s target %s, stat volPath:%s err:%v, don't do recovery", podName, ti.target, sourcePath, err)
				break
			}
		}
		klog.V(5).Infof("pod %s target %s recover volPath:%s", podName, ti.target, sourcePath)
		mountOption := []string{"bind"}
		if err := p.Mount(sourcePath, ti.target, "none", mountOption); err != nil {
			klog.Errorf("exec cmd: mount -o bind %s %s err:%v", sourcePath, ti.target, err)
		}

	case targetStatusUnexpect:
		klog.Errorf("pod %s target %s reslove err:%v", podName, ti.target, ti.err)
	}
}

func (p *PodDriver) umountTarget(target string, count int) {
	for i := 0; i < count; i++ {
		// ignore error
		p.Unmount(target)
	}
}

func umountPath(ctx context.Context, sourcePath string) {
	cmd := exec.Command("umount", sourcePath)
	out, err := doWithinTime(ctx, cmd, nil)
	if err != nil {
		if !strings.Contains(out, "not mounted") &&
			!strings.Contains(out, "mountpoint not found") &&
			!strings.Contains(out, "no mount point specified") {
			klog.V(5).Infof("Unmount %s failed: %q, try to lazy unmount", sourcePath, err)
			cmd2 := exec.Command("umount", "-l", sourcePath)
			output, err1 := doWithinTime(ctx, cmd2, nil)
			if err1 != nil {
				klog.Errorf("could not lazy unmount %q: %v, output: %s", sourcePath, err1, output)
			}
		}
	}
}

func doWithinTime(ctx context.Context, cmd *exec.Cmd, f func() error) (out string, err error) {
	doneCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	doneCh := make(chan error)
	go func() {
		if cmd != nil {
			outByte, e := cmd.CombinedOutput()
			out = string(outByte)
			doneCh <- e
		} else {
			doneCh <- f()
		}
	}()

	select {
	case <-doneCtx.Done():
		err = status.Error(codes.Internal, "context timeout")
		if cmd != nil {
			go func() {
				cmd.Process.Kill()
			}()
		}
		return
	case err = <-doneCh:
		return
	}
}

func (p *PodDriver) removeFinalizer(pod *corev1.Pod) error {
	f := pod.GetFinalizers()
	for i := 0; i < len(f); i++ {
		if f[i] == config.Finalizer {
			f = append(f[:i], f[i+1:]...)
			i--
		}
	}
	payload := []k8sclient.PatchListValue{{
		Op:    "replace",
		Path:  "/metadata/finalizers",
		Value: f,
	}}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Parse json error: %v", err)
		return err
	}
	if err := p.Client.PatchPod(pod, payloadBytes); err != nil {
		klog.Errorf("Patch pod err:%v", err)
		return err
	}
	return nil
}
