/*
 Copyright 2023 Juicedata Inc

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

package utils

import (
	"context"
	"io"
	"strings"

	"golang.org/x/net/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsAppPod(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		// mount pod mode
		if _, ok := pod.Labels[common.UniqueId]; ok {
			return true
		}
		// sidecar mode
		if _, ok := pod.Labels[common.InjectSidecarDone]; ok {
			return true
		}
	}
	return false
}

func IsSysPod(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		return pod.Labels["app.kubernetes.io/name"] == "juicefs-mount" || pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver" || pod.Labels["app.kubernetes.io/name"] == "juicefs-cache-group-worker"
	}
	return false
}

func IsCsiNode(pod *corev1.Pod) bool {
	if pod.Labels != nil {
		return pod.Labels["app.kubernetes.io/name"] == "juicefs-csi-driver" && pod.Labels["app"] == "juicefs-csi-node"
	}
	return false
}

func IsAppPodShouldList(ctx context.Context, client client.Client, pod *corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			var pvc corev1.PersistentVolumeClaim
			if err := client.Get(ctx, types.NamespacedName{Name: volume.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace}, &pvc); err != nil {
				return false
			}

			if pvc.Spec.VolumeName == "" {
				// pvc not bound
				// Can't tell whether it is juicefs pvc, so list it as well.
				return true
			}

			var pv corev1.PersistentVolume
			if err := client.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, &pv); err != nil {
				return false
			}
			if pv.Spec.CSI != nil && pv.Spec.CSI.Driver == config.DriverName {
				return true
			}
		}
	}
	return false
}

func LabelSelectorOfMount(pv corev1.PersistentVolume) labels.Selector {
	values := []string{pv.Spec.CSI.VolumeHandle}
	if pv.Spec.StorageClassName != "" {
		values = append(values, pv.Spec.StorageClassName)
	}
	sl := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      common.PodUniqueIdLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   values,
		}},
	}
	labelMap, _ := metav1.LabelSelectorAsSelector(&sl)
	return labelMap
}

func GetTargetUID(target string) string {
	pair := strings.Split(target, "volumes/kubernetes.io~csi")
	if len(pair) != 2 {
		return ""
	}

	podDir := strings.TrimSuffix(pair[0], "/")
	index := strings.LastIndex(podDir, "/")
	if index <= 0 {
		return ""
	}
	return podDir[index+1:]
}

type LogPipe struct {
	conn   *websocket.Conn
	stream io.ReadCloser
}

func NewLogPipe(ctx context.Context, conn *websocket.Conn, stream io.ReadCloser) *LogPipe {
	l := &LogPipe{
		conn:   conn,
		stream: stream,
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				l.stream.Close()
				return
			default:
				var temp []byte
				err := websocket.Message.Receive(l.conn, &temp)
				if err != nil {
					l.stream.Close()
					return
				}
				if strings.Contains(string(temp), "ping") {
					_ = websocket.Message.Send(l.conn, "pong")
				}
			}
		}
	}()
	return l
}

func (l *LogPipe) Write(p []byte) (int, error) {
	return len(p), websocket.Message.Send(l.conn, string(p))
}

func (l *LogPipe) Read(p []byte) (int, error) {
	return l.stream.Read(p)
}
