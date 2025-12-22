/*
Copyright 2025 Juicedata Inc

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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestMergeTolerations(t *testing.T) {
	testCases := []struct {
		name              string
		baseTolerations   []corev1.Toleration
		appTolerations    []corev1.Toleration
		expectedCount     int
		expectedHasNoSchedule bool
		expectedHasGPU    bool
	}{
		{
			name:            "both empty",
			baseTolerations: []corev1.Toleration{},
			appTolerations:  []corev1.Toleration{},
			expectedCount:   0,
		},
		{
			name: "only base tolerations",
			baseTolerations: []corev1.Toleration{
				{
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpExists,
				},
			},
			appTolerations:        []corev1.Toleration{},
			expectedCount:         1,
			expectedHasNoSchedule: true,
		},
		{
			name:            "only app tolerations",
			baseTolerations: []corev1.Toleration{},
			appTolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			expectedCount:  1,
			expectedHasGPU: true,
		},
		{
			name: "merge different tolerations",
			baseTolerations: []corev1.Toleration{
				{
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpExists,
				},
			},
			appTolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			expectedCount:         2,
			expectedHasNoSchedule: true,
			expectedHasGPU:        true,
		},
		{
			name: "remove duplicates",
			baseTolerations: []corev1.Toleration{
				{
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpExists,
				},
			},
			appTolerations: []corev1.Toleration{
				{
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpExists,
				},
			},
			expectedCount:         1,
			expectedHasNoSchedule: true,
		},
		{
			name: "merge multiple tolerations",
			baseTolerations: []corev1.Toleration{
				{
					Effect:   corev1.TaintEffectNoSchedule,
					Operator: corev1.TolerationOpExists,
				},
				{
					Key:      "dedicated",
					Operator: corev1.TolerationOpEqual,
					Value:    "juicefs",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			appTolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
				{
					Key:      "node.kubernetes.io/not-ready",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoExecute,
				},
			},
			expectedCount:         4,
			expectedHasNoSchedule: true,
			expectedHasGPU:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MergeTolerations(tc.baseTolerations, tc.appTolerations)
			assert.Equal(t, tc.expectedCount, len(result), "unexpected number of tolerations")

			if tc.expectedHasNoSchedule {
				found := false
				for _, t := range result {
					if t.Effect == corev1.TaintEffectNoSchedule && t.Operator == corev1.TolerationOpExists && t.Key == "" {
						found = true
						break
					}
				}
				assert.True(t, found, "expected to find NoSchedule Exists toleration")
			}

			if tc.expectedHasGPU {
				found := false
				for _, t := range result {
					if t.Key == "gpu" {
						found = true
						break
					}
				}
				assert.True(t, found, "expected to find gpu toleration")
			}
		})
	}
}
