/*
Copyright 2026 Juicedata Inc

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

package ddschedulerextender

import (
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultAnnotationPrefix = "headroom.datadoghq.com"
	legacyAnnotationPrefix  = "headroom.datadog.com"
)

type Headroom struct {
	MilliCPU int64
	Memory   int64
	Pods     int64
}

func (h Headroom) Empty() bool {
	return h.MilliCPU == 0 && h.Memory == 0 && h.Pods == 0
}

func (h Headroom) Add(other Headroom) Headroom {
	return Headroom{
		MilliCPU: h.MilliCPU + other.MilliCPU,
		Memory:   h.Memory + other.Memory,
		Pods:     h.Pods + other.Pods,
	}
}

func parseHeadroom(cpu, memory string, pods int64) (Headroom, error) {
	headroom := Headroom{Pods: pods}
	if cpu != "" {
		q, err := resource.ParseQuantity(cpu)
		if err != nil {
			return Headroom{}, fmt.Errorf("parse cpu headroom %q: %w", cpu, err)
		}
		headroom.MilliCPU = q.MilliValue()
	}
	if memory != "" {
		q, err := resource.ParseQuantity(memory)
		if err != nil {
			return Headroom{}, fmt.Errorf("parse memory headroom %q: %w", memory, err)
		}
		headroom.Memory = q.Value()
	}
	return headroom, nil
}

func headroomFromPod(pod *corev1.Pod, defaults Headroom, prefixes []string) (Headroom, error) {
	if pod == nil {
		return defaults, nil
	}
	if len(prefixes) == 0 {
		prefixes = []string{defaultAnnotationPrefix, legacyAnnotationPrefix}
	}
	headroom := defaults
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if err := applyHeadroomAnnotation(&headroom, pod.Annotations, prefix, "cpu"); err != nil {
			return Headroom{}, err
		}
		if err := applyHeadroomAnnotation(&headroom, pod.Annotations, prefix, "memory"); err != nil {
			return Headroom{}, err
		}
		if err := applyHeadroomAnnotation(&headroom, pod.Annotations, prefix, "pods"); err != nil {
			return Headroom{}, err
		}
	}
	return headroom, nil
}

func applyHeadroomAnnotation(headroom *Headroom, annotations map[string]string, prefix, resourceName string) error {
	if annotations == nil {
		return nil
	}
	value, ok := annotations[prefix+"/"+resourceName]
	if !ok || value == "" {
		return nil
	}
	switch resourceName {
	case "cpu":
		q, err := resource.ParseQuantity(value)
		if err != nil {
			return fmt.Errorf("parse %s/%s annotation %q: %w", prefix, resourceName, value, err)
		}
		headroom.MilliCPU = q.MilliValue()
	case "memory":
		q, err := resource.ParseQuantity(value)
		if err != nil {
			return fmt.Errorf("parse %s/%s annotation %q: %w", prefix, resourceName, value, err)
		}
		headroom.Memory = q.Value()
	case "pods":
		pods, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("parse %s/%s annotation %q: %w", prefix, resourceName, value, err)
		}
		headroom.Pods = pods
	}
	return nil
}

func podRequest(pod *corev1.Pod) Headroom {
	if pod == nil {
		return Headroom{}
	}

	request := Headroom{Pods: 1}
	for _, container := range pod.Spec.Containers {
		request.MilliCPU += container.Resources.Requests.Cpu().MilliValue()
		request.Memory += container.Resources.Requests.Memory().Value()
	}

	var initRequest Headroom
	for _, container := range pod.Spec.InitContainers {
		initRequest.MilliCPU = maxInt64(initRequest.MilliCPU, container.Resources.Requests.Cpu().MilliValue())
		initRequest.Memory = maxInt64(initRequest.Memory, container.Resources.Requests.Memory().Value())
	}
	request.MilliCPU = maxInt64(request.MilliCPU, initRequest.MilliCPU)
	request.Memory = maxInt64(request.Memory, initRequest.Memory)

	if pod.Spec.Overhead != nil {
		request.MilliCPU += pod.Spec.Overhead.Cpu().MilliValue()
		request.Memory += pod.Spec.Overhead.Memory().Value()
	}

	return request
}

func nodeAllocatable(node *corev1.Node) Headroom {
	if node == nil {
		return Headroom{}
	}
	return Headroom{
		MilliCPU: node.Status.Allocatable.Cpu().MilliValue(),
		Memory:   node.Status.Allocatable.Memory().Value(),
		Pods:     node.Status.Allocatable.Pods().Value(),
	}
}

func podConsumesResources(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Spec.NodeName == "" {
		return false
	}
	return pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
