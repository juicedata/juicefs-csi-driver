/*
 Copyright 2022 Juicedata Inc

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

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler/mutate"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler/validator"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type SidecarHandler struct {
	Client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder admission.Decoder
	// is in serverless environment
	serverless bool
}

var (
	handlerLog = klog.NewKlogr().WithName("sidecar-handler")
)

func NewSidecarHandler(client *k8sclient.K8sClient, serverless bool, scheme *runtime.Scheme) *SidecarHandler {
	return &SidecarHandler{
		Client:     client,
		serverless: serverless,
		decoder:    admission.NewDecoder(scheme),
	}
}

func (s *SidecarHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pod := &corev1.Pod{}
	raw := request.Object.Raw
	reqNamespace := request.Namespace
	handlerLog.V(1).Info("get pod", "reqNamespace", reqNamespace, "pod", string(raw))
	err := s.decoder.Decode(request, pod)
	if err != nil {
		handlerLog.Error(err, "unable to decoder pod from req")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// check if pod has done label
	if util.CheckExpectValue(pod.Labels, common.InjectSidecarDone, common.True) {
		handlerLog.Info("skip mutating the pod because injection is done.", "name", pod.Name, "namespace", pod.Namespace)
		return admission.Allowed("skip mutating the pod because injection is done")
	}

	// check if pod has disable label
	if util.CheckExpectValue(pod.Labels, common.InjectSidecarDisable, common.True) {
		handlerLog.Info("skip mutating the pod because injection is disabled.", "name", pod.Name, "namespace", pod.Namespace)
		return admission.Allowed("skip mutating the pod because injection is disabled")
	}

	// check if pod use JuiceFS Volume
	used, pair, err := resource.GetVolumes(ctx, s.Client, pod, reqNamespace)
	if err != nil {
		handlerLog.Error(err, "get pv from pod", "name", pod.Name, "namespace", pod.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	} else if !used {
		handlerLog.Info("skip mutating the pod because it doesn't use JuiceFS Volume.", "name", pod.Name, "namespace", pod.Namespace)
		return admission.Allowed("skip mutating the pod because it doesn't use JuiceFS Volume")
	}

	jfs := juicefs.NewJfsProvider(nil, s.Client)
	sidecarMutate := mutate.NewSidecarMutate(s.Client, jfs, s.serverless, pair)
	handlerLog.Info("start injecting juicefs client as sidecar in pod", "name", pod.Name, "namespace", pod.Namespace)
	out, err := sidecarMutate.Mutate(ctx, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	pod = out

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		handlerLog.Error(err, "unable to marshal pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	handlerLog.V(1).Info("mutated pod", "pod", string(marshaledPod))
	resp := admission.PatchResponseFromRaw(raw, marshaledPod)
	return resp
}

type SecretHandler struct {
	Client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder admission.Decoder
}

func NewSecretHandler(client *k8sclient.K8sClient, scheme *runtime.Scheme) *SecretHandler {
	return &SecretHandler{
		Client:  client,
		decoder: admission.NewDecoder(scheme),
	}
}

// InjectDecoder injects the decoder.
func (s *SecretHandler) InjectDecoder(d admission.Decoder) error {
	s.decoder = d
	return nil
}

func (s *SecretHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	secret := &corev1.Secret{}
	err := s.decoder.Decode(request, secret)
	if err != nil {
		handlerLog.Error(err, "unable to decoder secret from req")
		return admission.Errored(http.StatusBadRequest, err)
	}

	jfs := juicefs.NewJfsProvider(nil, nil)
	secretValidateor := validator.NewSecretValidator(jfs)
	if err := secretValidateor.Validate(ctx, *secret); err != nil {
		handlerLog.Error(err, "secret validation failed", "name", secret.Name, "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}
	return admission.Allowed("")
}

type PVHandler struct {
	Client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder admission.Decoder
}

func NewPVHandler(client *k8sclient.K8sClient, scheme *runtime.Scheme) *PVHandler {
	return &PVHandler{
		Client:  client,
		decoder: admission.NewDecoder(scheme),
	}
}

func (s *PVHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pv := &corev1.PersistentVolume{}
	err := s.decoder.Decode(request, pv)
	if err != nil {
		handlerLog.Error(err, "unable to decoder pv from req")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != config.DriverName {
		return admission.Allowed("")
	}
	if pv.Spec.StorageClassName != "" {
		return admission.Allowed("")
	}

	volumeHandle := pv.Spec.CSI.VolumeHandle
	existPvs, err := s.Client.ListPersistentVolumesByVolumeHandle(ctx, volumeHandle)
	if err != nil {
		handlerLog.Error(err, "list pv by volume handle failed", "volume handle", volumeHandle)
		return admission.Errored(http.StatusBadRequest, err)
	}
	if len(existPvs) > 0 {
		return admission.Denied(fmt.Sprintf("pv %s with volume handle %s already exists", pv.Name, volumeHandle))
	}
	return admission.Allowed("")
}

var (
	evictLog = klog.NewKlogr().WithName("evict-pod-handler")
)

type EvictPodHandler struct {
	client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder admission.Decoder
}

func NewEvictPodHandler(client *k8sclient.K8sClient, scheme *runtime.Scheme) *EvictPodHandler {
	return &EvictPodHandler{
		client:  client,
		decoder: admission.NewDecoder(scheme),
	}
}

func (s *EvictPodHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	if request.Namespace != config.Namespace {
		return admission.Allowed("")
	}
	if request.SubResource != "eviction" || request.Operation != "CREATE" {
		evictLog.Info("skip evict pod", "request", request)
		return admission.Allowed("")
	}
	evictLog.Info("receive evict pod request", "pod", request.Name, "namespace", request.Namespace)
	pod, err := s.client.GetPod(ctx, request.Name, request.Namespace)
	if err != nil {
		// if pod not found, allow the eviction
		// maybe the pod has been deleted by juicefs-csi-driver
		if k8serrors.IsNotFound(err) {
			return admission.Allowed("")
		}
		evictLog.Error(err, "get pod failed", "name", request.Name, "namespace", request.Namespace)
		return admission.Errored(http.StatusBadRequest, err)
	}
	if value, ok := pod.Labels[common.PodTypeKey]; !ok || value != common.PodTypeValue {
		evictLog.Info("skip evict pod because it's not a mount pod", "name", request.Name, "namespace", request.Namespace)
		return admission.Allowed("")
	}

	for k, target := range pod.Annotations {
		if k == util.GetReferenceKey(target) {
			evictLog.Info("deny evict mount pod because it has juicefs reference", "name", request.Name, "namespace", request.Namespace)
			return admission.Denied("deny evict mount pod because it has juicefs reference")
		}
	}

	return admission.Allowed("")
}
