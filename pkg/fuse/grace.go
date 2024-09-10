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

package fuse

import (
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

func ServeGfShutdown(addr string) error {
	_ = os.RemoveAll(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		fdLog.Error(err, "error listening on socket")
		return err
	}

	fdLog.Info("Serve gracefully shutdown is listening", "addr", addr)

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				fdLog.Error(err, "error accepting connection")
				continue
			}

			fdLog.Info("Start to graceful shutdown")
			go handleShutdown(conn)
		}
	}()
	return nil
}

func handleShutdown(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		fdLog.Error(err, "error reading from connection")
		return
	}

	message := string(buf[:n])

	var restart bool
	ss := strings.Split(message, " ")
	name := ss[0]
	if len(ss) == 2 {
		restart = true
	}

	fdLog.V(1).Info("Received shutdown message", "message", message)

	client, err := k8s.NewClient()
	if err != nil {
		fdLog.Error(err, "failed to create k8s client")
		return
	}

	if err := gracefulShutdown(context.TODO(), client, name, restart); err != nil {
		fdLog.Error(err, "graceful shutdown error")
		return
	}
}

func gracefulShutdown(ctx context.Context, client *k8s.K8sClient, name string, restart bool) error {
	mountPod, err := client.GetPod(ctx, name, config.Namespace)
	if err != nil {
		return err
	}
	if mountPod.Spec.NodeName != config.NodeName {
		return fmt.Errorf("pod %s is not on node %s", mountPod.Name, config.NodeName)
	}
	mntPath, _, err := resource.GetMountPathOfPod(*mountPod)
	if err != nil {
		return err
	}
	ce := util.ContainSubString(mountPod.Spec.Containers[0].Command, "format")

	// get pid and sid from <mountpoint>/.config
	fdLog.V(1).Info("get pid and sid from config", "path", mntPath, "pod", mountPod.Name)
	var conf []byte
	err = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		conf, err = os.ReadFile(path.Join(mntPath, ".config"))
		return err
	})
	jfsConf, err := util.ParseConfig(conf)
	if err != nil {
		return err
	}

	hashVal := mountPod.Labels[config.PodJuiceHashLabelKey]
	if hashVal == "" {
		return fmt.Errorf("pod %s/%s has no hash label", mountPod.Namespace, mountPod.Name)
	}
	fdLog.V(1).Info("get hash val from pod", "pod", mountPod.Name, "hash", hashVal)

	cPod, err := canaryPod(ctx, client, mountPod)
	if err != nil {
		return err
	}

	if err := waitForCanaryReady(ctx, client, cPod.Name, 5*time.Minute); err != nil {
		fdLog.Error(err, "canary pod is not ready, delete it.", "pod", cPod.Name)
		_ = client.DeletePod(ctx, cPod)
		return err
	}

	defer func() {
		fdLog.Info("delete canary pod", "pod", cPod.Name)
		if err := client.DeletePod(ctx, cPod); err != nil {
			fdLog.Error(err, "delete canary pod error", "pod", cPod.Name)
		}
	}()

	if restart {
		// set fuse fd to -1 in mount pod

		// update sid
		if ce {
			GlobalFds.updateSid(hashVal, jfsConf.Meta.Sid)
			fdLog.V(1).Info("update sid", "mountPod", mountPod.Name, "sid", jfsConf.Meta.Sid)
		}

		// close fuse fd in mount pod
		commPath, err := resource.GetCommPath("/tmp", *mountPod)
		if err != nil {
			return err
		}
		fuseFd, _ := getFuseFd(commPath, true)
		for i := 0; i < 100 && fuseFd == 0; i++ {
			time.Sleep(time.Millisecond * 100)
			fuseFd, _ = getFuseFd(commPath, true)
		}
		if fuseFd == 0 {
			return fmt.Errorf("fail to recv FUSE fd from %s", commPath)
		}
		fdLog.Info("recv FUSE fd", "fd", fuseFd)
	} else {
		fdLog.V(1).Info("download binary from canary pod", "pod", cPod.Name)
		// upgrade binary
		if err := downloadBinary(ctx, client, mountPod, cPod.Name); err != nil {
			return err
		}
		fdLog.V(1).Info("upgrade binary to mount pod", "pod", mountPod.Name)
		if err := uploadBinary(ctx, client, mountPod); err != nil {
			return err
		}
	}

	// send SIGHUP to mount pod
	for i := 0; i < 600; i++ {
		fdLog.Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", mountPod.Name)
		if stdout, stderr, err := client.ExecuteInContainer(
			ctx,
			mountPod.Name,
			mountPod.Namespace,
			config.MountContainerName,
			[]string{"kill", "-s", "SIGHUP", strconv.Itoa(jfsConf.Pid)},
		); err != nil {
			fdLog.V(1).Info("kill -s SIGHUP", "pid", jfsConf.Pid, "stdout", stdout, "stderr", stderr, "error", err)
			continue
		}
		return nil
	}
	fdLog.Info("mount point of mount pod is busy, stop upgrade", "podName", mountPod.Name)
	return nil
}

func downloadBinary(ctx context.Context, client *k8s.K8sClient, pod *corev1.Pod, canaryPod string) error {
	// download binary
	ce := util.ContainSubString(pod.Spec.Containers[0].Command, "format")
	if ce {
		stdout, stderr, err := client.ExecuteInContainer(
			ctx,
			canaryPod,
			config.Namespace,
			"canary",
			[]string{"sh", "-c", "cp /usr/local/bin/juicefs /tmp/juicefs"},
		)
		if err != nil {
			fdLog.Error(err, "download binary error", "pod", canaryPod, "stdout", stdout, "stderr", stderr)
			return nil
		}
		return nil
	}

	stdout, stderr, err := client.ExecuteInContainer(
		ctx,
		canaryPod,
		config.Namespace,
		"canary",
		[]string{"sh", "-c", "cp /usr/bin/juicefs /tmp/juicefs && cp /usr/local/juicefs/mount/jfsmount /tmp/jfsmount"},
	)
	if err != nil {
		fdLog.Error(err, "download binary error", "pod", canaryPod, "stdout", stdout, "stderr", stderr)
		return nil
	}
	return nil
}

func uploadBinary(ctx context.Context, client *k8s.K8sClient, pod *corev1.Pod) error {
	ce := util.ContainSubString(pod.Spec.Containers[0].Command, "format")
	if ce {
		stdout, stderr, err := client.ExecuteInContainer(
			ctx,
			pod.Name,
			pod.Namespace,
			config.MountContainerName,
			[]string{"sh", "-c", "rm -rf /usr/local/bin/juicefs && cp /tmp/juicefs /usr/local/bin/juicefs"},
		)
		if err != nil {
			fdLog.Error(err, "upload binary error", "pod", pod.Name, "stdout", stdout, "stderr", stderr)
			return nil
		}
		return nil
	}

	stdout, stderr, err := client.ExecuteInContainer(
		ctx,
		pod.Name,
		pod.Namespace,
		config.MountContainerName,
		[]string{"sh", "-c", "rm -rf /usr/bin/juicefs && cp /tmp/juicefs /usr/bin/juicefs  && rm -rf /usr/local/juicefs/mount/jfsmount && cp /tmp/jfsmount /usr/local/juicefs/mount/jfsmount"},
	)
	if err != nil {
		fdLog.Error(err, "upload binary error", "pod", pod.Name, "stdout", stdout, "stderr", stderr)
		return nil
	}
	return nil

}

// canaryPod:
// restart: pull image ahead
// !restart: for download binary
func canaryPod(ctx context.Context, client *k8s.K8sClient, mountPod *corev1.Pod) (*corev1.Pod, error) {
	attr, err := config.GenPodAttrWithMountPod(ctx, client, mountPod)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%s-canary", mountPod.Name)
	if po, err := client.GetPod(ctx, name, config.Namespace); err == nil {
		fdLog.Info("canary pod already exists, delete it first", "name", name)
		if err := client.DeletePod(ctx, po); err != nil {
			fdLog.Error(err, "delete canary pod error", "name", name)
			return nil, err
		}
	}
	fdLog.Info("create canary pod", "image", attr.Image, "name", name)
	var (
		mounts  []corev1.VolumeMount
		volumes []corev1.Volume
	)
	for _, v := range mountPod.Spec.Volumes {
		if v.Name == config.JfsFuseFdPathName {
			volumes = append(volumes, v)
		}
	}
	for _, c := range mountPod.Spec.Containers[0].VolumeMounts {
		if c.Name == config.JfsFuseFdPathName {
			mounts = append(mounts, c)
		}
	}
	cPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Image:        attr.Image,
				Name:         "canary",
				Command:      []string{"sleep", "infinity"},
				VolumeMounts: mounts,
			}},
			NodeName:      mountPod.Spec.NodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes:       volumes,
		},
	}
	if _, err := client.CreatePod(ctx, &cPod); err != nil {
		fdLog.Error(err, "create canary pod error", "name", name)
		return nil, err
	}
	return &cPod, nil
}

func waitForCanaryReady(ctx context.Context, client *k8s.K8sClient, name string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Wait until the mount point is ready
	fdLog.Info("waiting for canary pod ready", "name", name)
	for {
		pod, err := client.GetPod(waitCtx, name, config.Namespace)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				break
			}
			time.Sleep(time.Millisecond * 500)
			continue
		}
		if resource.IsPodReady(pod) {
			return nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	return fmt.Errorf("canary pod %s is not complete eventually", name)
}

func TriggerShutdown(socketPath string, name string, restart bool) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fdLog.Error(err, "error connecting to socket")
		return err
	}
	defer conn.Close()

	message := name
	if restart {
		message = fmt.Sprintf("%s RESTART", name)
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		fdLog.Error(err, "error sending message")
		return err
	}

	fdLog.Info("trigger gracefully shutdown successfully", "name", name)
	return nil
}
