// Copyright 2025 Juicedata Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events

import (
	"context"

	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

type EventService interface {
	ListEvents(ctx context.Context, namespace string, resource EventResource, uid string) ([]corev1.Event, error)
}

type eventService struct {
	k8sClient *k8sclient.K8sClient
}

func NewEventService(k8sClient *k8sclient.K8sClient) EventService {
	return &eventService{
		k8sClient: k8sClient,
	}
}

type EventResource string

const (
	EventResourcePod EventResource = "Pod"
	EventResourcePVC EventResource = "PersistentVolumeClaim"
	EventResourcePV  EventResource = "PersistentVolume"
	EventResourceJob EventResource = "Job"
	EventResourceSC  EventResource = "StorageClass"
)

func (s *eventService) ListEvents(ctx context.Context, namespace string, resource EventResource, uid string) ([]corev1.Event, error) {
	list, err := s.k8sClient.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{Kind: string(resource)},
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"involvedObject.uid": string(uid),
		}).String(),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}
