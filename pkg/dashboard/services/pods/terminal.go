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

package pods

import (
	"context"
	"io"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
	"golang.org/x/net/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (s *podService) WatchPodLogs(c *gin.Context, namespace, name, container string) error {
	var lines int64 = 100
	previousStr := c.Query("previous")
	previous := false
	if previousStr == "true" {
		previous = true
	}

	podLogOpts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &lines,
		Follow:    true,
		Previous:  previous,
	}

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		req := s.k8sClient.CoreV1().Pods(namespace).GetLogs(name, podLogOpts)
		stream, err := req.Stream(c.Request.Context())
		if err != nil {
			return
		}
		wr := utils.NewLogPipe(c.Request.Context(), ws, stream)
		_, err = io.Copy(wr, wr)
		if err != nil {
			return
		}
	}).ServeHTTP(c.Writer, c.Request)

	return nil
}

func (s *podService) ExecPod(c *gin.Context, namespace, name, container string) {
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		terminal := resource.NewTerminalSession(ctx, ws, resource.EndOfTransmission)
		if err := resource.ExecInPod(
			ctx,
			s.k8sClient, s.kubeconfig, terminal, namespace, name, container,
			[]string{"sh", "-c", "bash || sh"}); err != nil {
			podLog.Error(err, "Failed to exec in pod")
			return
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func (s *podService) WatchMountPodAccessLog(c *gin.Context, namespace, name, container string) {
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		terminal := resource.NewTerminalSession(ctx, ws, resource.EndOfText)
		mountpod, err := s.k8sClient.CoreV1().Pods(namespace).Get(c, name, metav1.GetOptions{})
		if err != nil {
			podLog.Error(err, "Failed to get mount pod")
			return
		}
		mntPath, _, err := util.GetMountPathOfPod(*mountpod)
		if err != nil || mntPath == "" {
			podLog.Error(err, "Failed to get mount path")
			return
		}
		if err := resource.ExecInPod(
			ctx,
			s.k8sClient, s.kubeconfig, terminal, namespace, name, container,
			[]string{"sh", "-c", "cat " + mntPath + "/.accesslog"}); err != nil {
			podLog.Error(err, "Failed to exec in pod")
			return
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func (s *podService) DebugPod(c *gin.Context, namespace, name, container string) {
	statsSec := c.Query("statsSec")
	traceSec := c.Query("traceSec")
	profileSec := c.Query("profileSec")
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		terminal := resource.NewTerminalSession(ctx, ws, resource.EndOfText)
		mountpod, err := s.k8sClient.CoreV1().Pods(namespace).Get(c, name, metav1.GetOptions{})
		if err != nil {
			podLog.Error(err, "Failed to get mount pod")
			return
		}
		mntPath, _, err := util.GetMountPathOfPod(*mountpod)
		if err != nil || mntPath == "" {
			podLog.Error(err, "Failed to get mount path")
			return
		}
		if err := resource.ExecInPod(
			ctx,
			s.k8sClient, s.kubeconfig, terminal, namespace, name, container,
			[]string{
				"juicefs", "debug",
				"--no-color",
				"--profile-sec", profileSec,
				"--trace-sec", traceSec,
				"--stats-sec", statsSec,
				"--out-dir", "/debug",
				mntPath}); err != nil {
			podLog.Error(err, "Failed to start process")
			return
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func (s *podService) WarmupPod(c *gin.Context, namespace, name, container string) {
	threads := c.Query("threads")
	ioRetries := c.Query("ioRetries")
	maxFailure := c.Query("maxFailure")
	background := c.Query("background")
	check := c.Query("check")
	customSubPath := c.Query("subPath")

	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		terminal := resource.NewTerminalSession(ctx, ws, resource.EndOfText)
		mountpod, err := s.k8sClient.CoreV1().Pods(namespace).Get(c, name, metav1.GetOptions{})
		if err != nil {
			klog.Error("Failed to get mount pod: ", err)
			return
		}

		mntPath, _, err := util.GetMountPathOfPod(*mountpod)
		if err != nil || mntPath == "" {
			klog.Error("Failed to get mount path: ", err)
			return
		}
		cmds := []string{
			"juicefs", "warmup",
			"--threads=" + threads,
			"--background=" + background,
			"--check=" + check,
			"--no-color",
		}
		if !config.IsCEMountPod(mountpod) {
			cmds = append(cmds, "--io-retries="+ioRetries)
			cmds = append(cmds, "--max-failure="+maxFailure)
		}
		cmds = append(cmds, path.Join(mntPath, customSubPath))
		if err := resource.ExecInPod(
			ctx,
			s.k8sClient, s.kubeconfig, terminal, namespace, name, container,
			cmds); err != nil {
			klog.Error("Failed to start process: ", err)
			return
		}
	}).ServeHTTP(c.Writer, c.Request)
}

func (s *podService) DownloadDebugFile(c *gin.Context, namespace, name, container string) error {
	err := resource.DownloadPodFile(
		c.Request.Context(),
		s.k8sClient, s.kubeconfig, c.Writer, namespace, name, container,
		[]string{"sh", "-c", "cat $(ls -t /debug/*.zip | head -n 1) && exit 0"})
	if err != nil {
		podLog.Error(err, "Failed to create SPDY executor")
		return err
	}
	return nil
}
