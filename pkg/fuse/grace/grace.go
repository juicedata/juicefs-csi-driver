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

package grace

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
	"k8s.io/klog/v2"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	"github.com/juicedata/juicefs-csi-driver/pkg/juicefs/mount/builder"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var log = klog.NewKlogr().WithName("grace")

func ServeGfShutdown(addr string) error {
	_ = os.RemoveAll(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		log.Error(err, "error listening on socket")
		return err
	}

	log.Info("Serve gracefully shutdown is listening", "addr", addr)

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Error(err, "error accepting connection")
				continue
			}

			log.Info("Start to graceful shutdown")
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
		log.Error(err, "error reading from connection")
		return
	}

	message := string(buf[:n])

	var restart bool
	ss := strings.Split(message, " ")
	name := ss[0]
	if len(ss) == 2 {
		restart = true
	}

	log.V(1).Info("Received shutdown message", "message", message)

	client, err := k8s.NewClient()
	if err != nil {
		log.Error(err, "failed to create k8s client")
		return
	}

	if err := gracefulShutdown(context.TODO(), client, name, restart); err != nil {
		log.Error(err, "graceful shutdown error")
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
	hashVal := mountPod.Labels[config.PodJuiceHashLabelKey]
	if hashVal == "" {
		return fmt.Errorf("pod %s/%s has no hash label", mountPod.Namespace, mountPod.Name)
	}
	log.V(1).Info("get hash val from pod", "pod", mountPod.Name, "hash", hashVal)
	lock := config.GetPodLock(hashVal)
	lock.Lock()
	defer lock.Unlock()

	mntPath, _, err := util.GetMountPathOfPod(*mountPod)
	if err != nil {
		return err
	}
	ce := util.ContainSubString(mountPod.Spec.Containers[0].Command, "format")

	// get pid and sid from <mountpoint>/.config
	log.V(1).Info("get pid and sid from config", "path", mntPath, "pod", mountPod.Name)
	var conf []byte
	err = util.DoWithTimeout(ctx, 2*time.Second, func() error {
		conf, err = os.ReadFile(path.Join(mntPath, ".config"))
		return err
	})
	jfsConf, err := util.ParseConfig(conf)
	if err != nil {
		return err
	}

	cJob, err := builder.NewCanaryJob(ctx, client, mountPod, restart)
	if err != nil {
		return err
	}

	if err := resource.WaitForJobComplete(ctx, client, cJob.Name, 5*time.Minute); err != nil {
		log.Error(err, "canary job is not complete, delete it.", "job", cJob.Name)
		_ = client.DeleteJob(ctx, cJob.Name, cJob.Namespace)
		return err
	}

	if restart {
		// set fuse fd to -1 in mount pod

		// update sid
		if ce {
			passfd.GlobalFds.UpdateSid(hashVal, jfsConf.Meta.Sid)
			log.V(1).Info("update sid", "mountPod", mountPod.Name, "sid", jfsConf.Meta.Sid)
		}

		// close fuse fd in mount pod
		commPath, err := resource.GetCommPath("/tmp", *mountPod)
		if err != nil {
			return err
		}
		fuseFd, _ := passfd.GetFuseFd(commPath, true)
		for i := 0; i < 100 && fuseFd == 0; i++ {
			time.Sleep(time.Millisecond * 100)
			fuseFd, _ = passfd.GetFuseFd(commPath, true)
		}
		if fuseFd == 0 {
			return fmt.Errorf("fail to recv FUSE fd from %s", commPath)
		}
		log.Info("recv FUSE fd", "fd", fuseFd)
	} else {
		// upgrade binary
		log.V(1).Info("upgrade binary to mount pod", "pod", mountPod.Name)
		if err := uploadBinary(ctx, client, mountPod); err != nil {
			return err
		}
	}

	// send SIGHUP to mount pod
	for i := 0; i < 600; i++ {
		log.Info("kill -s SIGHUP", "pid", jfsConf.Pid, "pod", mountPod.Name)
		if stdout, stderr, err := client.ExecuteInContainer(
			ctx,
			mountPod.Name,
			mountPod.Namespace,
			config.MountContainerName,
			[]string{"kill", "-s", "SIGHUP", strconv.Itoa(jfsConf.Pid)},
		); err != nil {
			log.V(1).Info("kill -s SIGHUP", "pid", jfsConf.Pid, "stdout", stdout, "stderr", stderr, "error", err)
			continue
		}
		return nil
	}
	log.Info("mount point of mount pod is busy, stop upgrade", "podName", mountPod.Name)
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
			log.Error(err, "download binary error", "pod", canaryPod, "stdout", stdout, "stderr", stderr)
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
		log.Error(err, "download binary error", "pod", canaryPod, "stdout", stdout, "stderr", stderr)
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
			[]string{"sh", "-c", "rm -rf /usr/local/bin/juicefs && mv /tmp/juicefs /usr/local/bin/juicefs"},
		)
		if err != nil {
			log.Error(err, "upload binary error", "pod", pod.Name, "stdout", stdout, "stderr", stderr)
			return nil
		}
		return nil
	}

	stdout, stderr, err := client.ExecuteInContainer(
		ctx,
		pod.Name,
		pod.Namespace,
		config.MountContainerName,
		[]string{"sh", "-c", "rm -rf /usr/bin/juicefs && mv /tmp/juicefs /usr/bin/juicefs  && rm -rf /usr/local/juicefs/mount/jfsmount && mv /tmp/jfsmount /usr/local/juicefs/mount/jfsmount"},
	)
	if err != nil {
		log.Error(err, "upload binary error", "pod", pod.Name, "stdout", stdout, "stderr", stderr)
		return nil
	}
	return nil

}

func TriggerShutdown(socketPath string, name string, restart bool) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Error(err, "error connecting to socket")
		return err
	}
	defer conn.Close()

	message := name
	if restart {
		message = fmt.Sprintf("%s RESTART", name)
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error(err, "error sending message")
		return err
	}

	log.Info("trigger gracefully shutdown successfully", "name", name)
	return nil
}
