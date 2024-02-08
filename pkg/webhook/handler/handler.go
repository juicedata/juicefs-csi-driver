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
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler/mutate"
	"github.com/juicedata/juicefs-csi-driver/pkg/webhook/handler/validator"
)

type SidecarHandler struct {
	Client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder *admission.Decoder
	// is in serverless environment
	serverless bool
}

func NewSidecarHandler(client *k8sclient.K8sClient, serverless bool) *SidecarHandler {
	return &SidecarHandler{
		Client:     client,
		serverless: serverless,
	}
}

func (s *SidecarHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	pod := &corev1.Pod{}
	raw := request.Object.Raw
	klog.V(6).Infof("[SidecarHandler] get pod: %s", string(raw))
	err := s.decoder.Decode(request, pod)
	if err != nil {
		klog.Error(err, "unable to decoder pod from req")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// check if pod has done label
	if util.CheckExpectValue(pod.Labels, config.InjectSidecarDone, config.True) {
		klog.Infof("[SidecarHandler] skip mutating the pod because injection is done. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because injection is done")
	}

	// check if pod has disable label
	if util.CheckExpectValue(pod.Labels, config.InjectSidecarDisable, config.True) {
		klog.Infof("[SidecarHandler] skip mutating the pod because injection is disabled. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because injection is disabled")
	}

	// check if pod use JuiceFS Volume
	used, pair, err := util.GetVolumes(ctx, s.Client, pod)
	if err != nil {
		klog.Errorf("[SidecarHandler] get pv from pod %s namespace %s err: %v", pod.Name, pod.Namespace, err)
		return admission.Errored(http.StatusBadRequest, err)
	} else if !used {
		klog.Infof("[SidecarHandler] skip mutating the pod because it doesn't use JuiceFS Volume. Pod %s namespace %s", pod.Name, pod.Namespace)
		return admission.Allowed("skip mutating the pod because it doesn't use JuiceFS Volume")
	}

	jfs := juicefs.NewJfsProvider(nil, s.Client)
	sidecarMutate := mutate.NewSidecarMutate(s.Client, jfs, s.serverless, pair)
	klog.Infof("[SidecarHandler] start injecting juicefs client as sidecar in pod [%s] namespace [%s].", pod.Name, pod.Namespace)
	out, err := sidecarMutate.Mutate(ctx, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	pod = out

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		klog.Error(err, "unable to marshal pod")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	klog.V(6).Infof("[SidecarHandler] mutated pod: %s", string(marshaledPod))
	resp := admission.PatchResponseFromRaw(raw, marshaledPod)
	return resp
}

// InjectDecoder injects the decoder.
func (s *SidecarHandler) InjectDecoder(d *admission.Decoder) error {
	s.decoder = d
	return nil
}

type SecretHandler struct {
	Client *k8sclient.K8sClient
	// A decoder will be automatically injected
	decoder *admission.Decoder
}

func NewSecretHandler(client *k8sclient.K8sClient) *SecretHandler {
	return &SecretHandler{
		Client: client,
	}
}

// InjectDecoder injects the decoder.
func (s *SecretHandler) InjectDecoder(d *admission.Decoder) error {
	s.decoder = d
	return nil
}

func (s *SecretHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	secret := &corev1.Secret{}
	err := s.decoder.Decode(request, secret)
	if err != nil {
		klog.Errorf("unable to decoder secret from req, %v", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	jfs := juicefs.NewJfsProvider(nil, nil)
	secretValidateor := validator.NewSecretValidator(jfs)
	if err := secretValidateor.Validate(ctx, *secret); err != nil {
		klog.Errorf("secret validation failed, secret: %s, err: %v", secret.Name, err)
		return admission.Errored(http.StatusBadRequest, err)
	}
	return admission.Allowed("")
}
