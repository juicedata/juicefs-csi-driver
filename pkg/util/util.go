/*
Copyright 2018 The Kubernetes Authors.

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
	"crypto/sha256"
	"fmt"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/juicefs/k8sclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func ParseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	addr := path.Join(u.Host, filepath.FromSlash(u.Path))

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "tcp":
	case "unix":
		addr = path.Join("/", addr)
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
	default:
		return "", "", fmt.Errorf("unsupported protocol: %s", scheme)
	}

	return scheme, addr, nil
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func GetReferenceKey(target string) string {
	h := sha256.New()
	h.Write([]byte(target))
	return fmt.Sprintf("juicefs-%x", h.Sum(nil))[:63]
}

// ParseMntPath return mntPath, volumeId (/jfs/volumeId, volumeId err)
func ParseMntPath(cmd string) (string, string, error) {
	args := strings.Fields(cmd)
	if len(args) < 3 || !strings.HasPrefix(args[2], config.PodMountBase) {
		return "", "", fmt.Errorf("err cmd:%s", cmd)
	}
	argSlice := strings.Split(args[2], "/")
	if len(argSlice) < 3 {
		return "", "", fmt.Errorf("err mntPath:%s", args[2])
	}
	return args[2], argSlice[2], nil
}

// GetTimeAfterDelay get time which after delay
func GetTimeAfterDelay(delayStr string) (string, error) {
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		return "", err
	}
	delayAt := time.Now().Add(delay)
	return delayAt.Format("2006-01-02 15:04:05"), nil
}

func GetTime(str string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", str)
}

func ShouldDelay(pod *corev1.Pod, Client *k8s.K8sClient) (shouldDelay bool) {
	delayStr, delayExist := pod.Annotations[config.DeleteDelayTimeKey]
	if !delayExist {
		// not set delete delay
		return false
	}
	delayAtStr, delayAtExist := pod.Annotations[config.DeleteDelayAtKey]
	if !delayAtExist {
		// need to add delayAt annotation
		d, err := GetTimeAfterDelay(delayStr)
		if err != nil {
			klog.Errorf("delayDelete: can't parse delay time %s: %v", d, err)
			return false
		}
		pod.Annotations[config.DeleteDelayAtKey] = d
		if err := Client.UpdatePod(pod); err != nil {
			klog.Errorf("delayDelete: Update pod %s error: %v", pod.Name, err)
		}
		return true
	}
	delayAt, err := GetTime(delayAtStr)
	if err != nil {
		klog.Errorf("delayDelete: can't parse delayAt %s: %v", delayAtStr, err)
		return false
	}
	return time.Now().After(delayAt)
}
