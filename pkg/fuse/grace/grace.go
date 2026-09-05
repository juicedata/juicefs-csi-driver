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
	"bufio"
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

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var log = klog.NewKlogr().WithName("grace")

const (
	recreate             = "RECREATE"
	noRecreate           = "NORECREATE"
	singleUpgradeTimeout = 30 * time.Minute
)

func ServeGfShutdown(addr string) error {
	err := util.DoWithTimeout(context.TODO(), 2*time.Second, func(ctx context.Context) error {
		if util.Exists(addr) {
			return os.Remove(addr)
		}
		return nil
	})
	if err != nil {
		return err
	}

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

			go handleShutdown(conn)
		}
	}()
	return nil
}

type upgradeRequest struct {
	action     string
	name       string
	configName string
	batchIndex int
}

// parseRequest parse request from message
// message format: <pod-name> [recreate/noRecreate]
func parseRequest(message string) upgradeRequest {
	req := upgradeRequest{
		action: noRecreate,
	}

	ss := strings.Split(message, " ")
	req.name = ss[0]
	if len(ss) < 2 {
		return req
	}
	req.action = ss[1]
	if ss[0] == "BATCH" && len(ss) > 2 {
		options := strings.Split(ss[2], ",")
		for _, option := range options {
			ops := strings.Split(option, "=")
			if len(ops) < 2 {
				continue
			}
			if ops[0] == "batchIndex" {
				w, err := strconv.Atoi(ops[1])
				if err != nil {
					log.Error(err, "failed to parse options", "option", option)
					continue
				}
				req.batchIndex = w
			}
			if ops[0] == "batchConfig" {
				req.configName = ops[1]
			}
		}
		return req
	}
	return req
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
	req := parseRequest(message)

	if req.name == "list" {
		_, _ = conn.Write(passfd.GlobalFds.PrintFds())
		return
	}

	log.Info("Received shutdown message", "message", message)

	client, err := k8s.NewClient()
	if err != nil {
		log.Error(err, "failed to create k8s client")
		return
	}
	if req.name == "BATCH" {
		NewBatchUpgrade(client, req).BatchUpgrade(context.TODO(), conn)
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), singleUpgradeTimeout)
	defer cancel()
	SinglePodUpgrade(ctx, client, req.name, req.action == recreate, conn)
}

func SinglePodUpgrade(ctx context.Context, client *k8s.K8sClient, name string, recreate bool, conn net.Conn) {
	sendMessage(conn, fmt.Sprintf("POD-START [%s] start to upgrade", name))
	pu, err := NewPodUpgrade(ctx, client, name, recreate, conn)
	if err != nil {
		log.Error(err, "failed to create pod upgrade")
		return
	}

	canUpgrade, reason, err := resource.CanUpgradeWithHash(ctx, client, *pu.pod, pu.recreate)
	if err != nil || !canUpgrade {
		sendMessage(conn, fmt.Sprintf("POD-FAIL [%s] can not upgrade: %s.", pu.pod.Name, reason))
		return
	}

	if err := pu.gracefulShutdown(ctx, conn); err != nil {
		log.Error(err, "graceful shutdown error")
		if pu.recreate {
			if e := resource.DelPodAnnotation(ctx, client, pu.pod.Name, pu.pod.Namespace, []string{common.JfsUpgradeProcess}); e != nil {
				sendMessage(conn, fmt.Sprintf("WARNING delete annotation uprgadeProcess in [%s] error: %s.", pu.pod.Name, e.Error()))
				return
			}
		}
		return
	}
	if pu.recreate {
		pu.waitForUpgrade(ctx, conn)
	}
}

type GraceUpgrade struct {
	client *k8s.K8sClient
	conn   net.Conn
}

type GraceRunner interface {
	StatusPrefix() string
	TargetName() string
	LockKey() string
	PrepareShutdown(context.Context) (*util.JuiceConf, error)
	Sighup(context.Context, *util.JuiceConf) error
	OnFail()
}

func (g *GraceUpgrade) sendMessage(message string) {
	if g == nil {
		return
	}
	if g.conn == nil {
		fmt.Printf("%s %s\n", time.Now().Format(time.DateTime), message)
		return
	}
	sendMessage(g.conn, message)
}

func (g *GraceUpgrade) runGracefulUpgrade(ctx context.Context, runner GraceRunner) error {
	unlock, err := config.LockPod(ctx, runner.LockKey())
	if err != nil {
		return err
	}
	defer unlock()

	jfsConf, err := runner.PrepareShutdown(ctx)
	if err != nil {
		g.sendMessage(fmt.Sprintf("%s-FAIL [%s] %s.", runner.StatusPrefix(), runner.TargetName(), err.Error()))
		runner.OnFail()
		return err
	}
	if jfsConf == nil {
		return nil
	}

	if err := runner.Sighup(ctx, jfsConf); err != nil {
		g.sendMessage(fmt.Sprintf("%s-FAIL [%s] %s.", runner.StatusPrefix(), runner.TargetName(), err.Error()))
		runner.OnFail()
		return err
	}
	return nil
}

func (g *GraceUpgrade) uploadBinary(ctx context.Context, pod *corev1.Pod, containerName string, isCe bool) error {
	if g == nil || g.client == nil {
		return fmt.Errorf("grace upgrade client is nil")
	}
	script := "cp /usr/bin/juicefs /tmp/juicefs.bak && cp /usr/local/juicefs/mount/jfsmount /tmp/jfsmount.bak && mv /tmp/juicefs /usr/bin/juicefs && mv /tmp/jfsmount /usr/local/juicefs/mount/jfsmount"
	if isCe {
		script = "cp /usr/local/bin/juicefs /tmp/juicefs.bak && mv /tmp/juicefs /usr/local/bin/juicefs"
	}
	stdout, stderr, err := g.client.ExecuteInContainer(
		ctx,
		pod.Name,
		pod.Namespace,
		containerName,
		[]string{"sh", "-c", script},
	)
	if err != nil {
		return fmt.Errorf("upload binary error: %v, stderr: %s", err, stderr)
	}
	_ = stdout
	return nil
}

func (g *GraceUpgrade) sighup(ctx context.Context, pod *corev1.Pod, containerName string, pid int) error {
	if g == nil || g.client == nil {
		return fmt.Errorf("grace upgrade client is nil")
	}
	if _, stderr, err := g.client.ExecuteInContainer(
		ctx,
		pod.Name,
		pod.Namespace,
		containerName,
		[]string{"kill", "-s", "SIGHUP", fmt.Sprintf("%d", pid)},
	); err != nil {
		return fmt.Errorf("send SIGHUP failed: %v, stderr: %s", err, stderr)
	}
	return nil
}

func TriggerShutdown(socketPath string, name string, recreateFlag bool) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Error(err, "error connecting to socket")
		return err
	}
	defer conn.Close()

	var message string
	if recreateFlag {
		message = fmt.Sprintf("%s %s", name, recreate)
	} else {
		message = fmt.Sprintf("%s %s", name, noRecreate)
	}
	if name == "list" {
		message = "list"
	}

	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Error(err, "error sending message")
		return err
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message = scanner.Text()
		fmt.Printf("%s %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
		if strings.HasPrefix(message, "POD-SUCCESS") || strings.HasPrefix(message, "POD-FAIL") {
			break
		}
	}

	return scanner.Err()
}

func sendMessage(conn net.Conn, message string) {
	_, err := conn.Write([]byte(message + "\n"))
	if err != nil {
		log.V(1).Info("error sending message", "message", message, "error", err)
	}
}

// resolvePpid determines the PID used to locate the fuse_fd_comm socket.
// Resolution order:
//  1. jfsConf.PPid if non-zero
//  2. suffix of path.Base(jfsConf.CommPath) after the last "." (e.g. "fuse_fd_comm.3551359" → 3551359)
//  3. 1 for non-HostPID pods (mount process is PID 1 in its own namespace)
//  4. error for HostPID pods where the real host PID cannot be determined
func resolvePpid(jfsConf *util.JuiceConf, hostPID bool) (int, error) {
	if jfsConf.PPid != 0 {
		return jfsConf.PPid, nil
	}
	base := path.Base(jfsConf.CommPath)
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		if ppid, err := strconv.Atoi(base[idx+1:]); err == nil && ppid > 0 {
			return ppid, nil
		}
	}
	if !hostPID {
		return 1, nil
	}
	return 0, fmt.Errorf("unable to determine ppid (PPid=%d, CommPath=%q)", jfsConf.PPid, jfsConf.CommPath)
}
