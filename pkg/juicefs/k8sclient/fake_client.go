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

package k8sclient

import (
	"encoding/json"
	"sync"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

// FakeK8sClient creates a new mock k8s client used for testing
type FakeK8sClient struct {
	PodMap map[string]*corev1.Pod
	lock   sync.RWMutex
}

var FakeClient = &FakeK8sClient{
	PodMap: make(map[string]*corev1.Pod),
	lock:   sync.RWMutex{},
}

func (f *FakeK8sClient) Flush() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.PodMap = make(map[string]*corev1.Pod)
}

func (f *FakeK8sClient) CreatePod(pod *corev1.Pod) (*corev1.Pod, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
			{
				Type:   corev1.ContainersReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
	f.PodMap[pod.Name] = pod
	return pod, nil
}

func (f *FakeK8sClient) GetPod(podName, namespace string) (*corev1.Pod, error) {
	pod, ok := f.PodMap[podName]
	if !ok {
		return nil, k8serrors.NewNotFound(schema.GroupResource{
			Group:    corev1.GroupName,
			Resource: "Pod",
		}, podName)
	}
	return pod, nil
}

func (f *FakeK8sClient) PatchPod(pod *corev1.Pod, data []byte) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	po := f.PodMap[pod.Name]
	if po == nil {
		return k8serrors.NewNotFound(schema.GroupResource{
			Group:    corev1.GroupName,
			Resource: "Pod",
		}, pod.Name)
	}
	originalObjJS, err := runtime.Encode(unstructured.UnstructuredJSONScheme, po)
	if err != nil {
		return err
	}
	originalPatchedObjJS, err := strategicpatch.StrategicMergePatch(originalObjJS, data, po)
	if err != nil {
		return err
	}
	newPod := &corev1.Pod{}
	err = json.Unmarshal(originalPatchedObjJS, newPod)
	if err != nil {
		return err
	}

	f.PodMap[pod.Name] = newPod
	return nil
}

func (f *FakeK8sClient) UpdatePod(pod *corev1.Pod) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	po := f.PodMap[pod.Name]
	if po == nil {
		return k8serrors.NewNotFound(schema.GroupResource{
			Group:    corev1.GroupName,
			Resource: "Pod",
		}, pod.Name)
	}
	f.PodMap[pod.Name] = pod
	return nil
}

func (f *FakeK8sClient) DeletePod(pod *corev1.Pod) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	po := f.PodMap[pod.Name]
	if po == nil {
		return k8serrors.NewNotFound(schema.GroupResource{
			Group:    corev1.GroupName,
			Resource: "Pod",
		}, pod.Name)
	}
	delete(f.PodMap, pod.Name)
	return nil
}
