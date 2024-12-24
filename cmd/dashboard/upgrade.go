/*
 Copyright 2024 Juicedata Inc

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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var batchConfigName string

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "trigger upgrade mount pod smoothly",
	Run: func(cmd *cobra.Command, args []string) {
		var k8sconfig *rest.Config
		var err error
		sysNamespace := os.Getenv(SysNamespaceKey)
		if sysNamespace == "" {
			sysNamespace = "kube-system"
		}
		if devMode {
			k8sconfig, _ = getLocalConfig()
		} else {
			gin.SetMode(gin.ReleaseMode)
			k8sconfig = ctrl.GetConfigOrDie()
		}
		clientset, err := kubernetes.NewForConfig(k8sconfig)
		if err != nil {
			logger("BATCH-FAIL failed to create kubernetes clientset")
			os.Exit(1)
		}

		k8sClient, err := k8sclient.NewClientWithConfig(*k8sconfig)
		if err != nil {
			logger("BATCH-FAIL could not create k8s client")
			os.Exit(1)
		}

		batchConfigName = os.Getenv(common.JfsUpgradeConfig)
		conf, err := config.LoadUpgradeConfig(context.Background(), k8sClient, batchConfigName)
		if err != nil {
			logger("BATCH-FAIL failed to load upgrade config")
			os.Exit(1)
		}

		podsStatus := make(map[string]config.UpgradeStatus)
		for _, batch := range conf.Batches {
			for _, pods := range batch {
				podsStatus[pods.Name] = config.Running
			}
		}
		bu := &BatchUpgrade{
			sysNamespace:    sysNamespace,
			conf:            conf,
			k8sConfig:       k8sconfig,
			k8sClient:       k8sClient,
			clientset:       clientset,
			lock:            sync.Mutex{},
			podsStatus:      podsStatus,
			status:          config.Running,
			crtBatchStatus:  config.Pending,
			nextBatchStatus: config.Pending,
			crtBatch:        0,
		}
		bu.flushStatus(context.TODO())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go bu.handleSignal()
		bu.Run(ctx)
	},
}

func (u *BatchUpgrade) handleSignal() {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGTERM)

	paused := false
	for sig := range sigChan {
		if sig == syscall.SIGUSR1 {
			paused = !paused
			if paused {
				logger("Pause upgrade...")
				u.setNextBatchStatus(config.Pause)
				u.status = config.Pause
				u.flushStatus(context.TODO())
			} else {
				logger("Resuming upgrade...")
				u.setNextBatchStatus(config.Pending)
				u.status = config.Running
				u.flushStatus(context.TODO())
			}
		}
		if sig == syscall.SIGTERM {
			logger("Stop upgrade...")
			u.setNextBatchStatus(config.Stop)
			u.status = config.Stop
			u.flushStatus(context.TODO())
			return
		}
	}
}

type BatchUpgrade struct {
	sysNamespace string
	conf         *config.BatchConfig
	k8sConfig    *rest.Config
	k8sClient    *k8sclient.K8sClient
	clientset    *kubernetes.Clientset

	lock            sync.Mutex
	podsStatus      map[string]config.UpgradeStatus
	status          config.UpgradeStatus
	crtBatchStatus  config.UpgradeStatus
	nextBatchStatus config.UpgradeStatus
	crtBatch        int
}

func (u *BatchUpgrade) Run(ctx context.Context) {
	if len(u.conf.Batches) == 0 {
		logger("BATCH-SUCCESS no batch found")
		u.status = config.Success
		u.flushStatus(ctx)
		return
	}
	if u.conf.Parallel > 50 {
		logger("BATCH-FAIL parallel should not exceed 50")
		u.panic(ctx)
	}

	handleFinalStatus := func() {
		if u.status == config.Fail {
			logger("BATCH-FAIL some pods upgrade failed")
			u.panic(ctx)
		}
		if u.status == config.Success {
			logger("BATCH-SUCCESS all pods upgraded successfully")
		}
		u.flushStatus(ctx)
	}

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			logger("BATCH-FAIL upgrade timeout")
			u.panic(ctx)
			return
		case <-t.C:
			if u.crtBatchStatus == config.Fail && !u.conf.IgnoreError {
				u.status = config.Fail
				handleFinalStatus()
				return
			}
			if u.crtBatch > len(u.conf.Batches) {
				u.status = u.crtBatchStatus
				handleFinalStatus()
				return
			}
			switch u.nextBatchStatus {
			case config.Pending:
				if u.crtBatchStatus == config.Pending || u.crtBatchStatus == config.Success || (u.crtBatchStatus == config.Fail && u.conf.IgnoreError) {
					u.crtBatch++
					go u.processBatch(ctx)
				}
			case config.Pause:
			case config.Stop:
				u.status = config.Stop
				handleFinalStatus()
				return
			}
		}
	}
}

func (u *BatchUpgrade) processBatch(ctx context.Context) {
	if u.crtBatch > len(u.conf.Batches) {
		return
	}
	u.setCrtBatchStatus(config.Running)
	var (
		wg       sync.WaitGroup
		batch    = u.conf.Batches[u.crtBatch-1]
		resultCh = make(chan error, len(batch))
	)
	go func() {
		defer func() {
			wg.Wait()
			close(resultCh)
		}()
		for _, mp := range batch {
			wg.Add(1)
			go func(mp config.MountPodUpgrade) {
				resultCh <- u.triggerUpgrade(ctx, &mp)
				wg.Done()
			}(mp)
		}
	}()
	for oneErr := range resultCh {
		// pod upgrade error:
		// 1. trigger upgrade failed
		// 2. pod upgrade failed which is parsed in log stream
		if oneErr != nil {
			u.setCrtBatchStatus(config.Fail)
			return
		}
		for _, s := range u.podsStatus {
			if s == config.Fail {
				u.setCrtBatchStatus(config.Fail)
				return
			}
		}
	}
	u.setCrtBatchStatus(config.Success)
}

func (u *BatchUpgrade) setCrtBatchStatus(s config.UpgradeStatus) {
	u.lock.Lock()
	u.crtBatchStatus = s
	u.lock.Unlock()
}

func (u *BatchUpgrade) setNextBatchStatus(s config.UpgradeStatus) {
	u.nextBatchStatus = s
}

func (u *BatchUpgrade) panic(ctx context.Context) {
	u.status = config.Fail
	u.flushStatus(ctx)
	os.Exit(1)
}

func (u *BatchUpgrade) flushStatus(ctx context.Context) {
	u.lock.Lock()
	defer u.lock.Unlock()
	conf := u.conf
	for i, batch := range conf.Batches {
		for j, mp := range batch {
			mp.Status = u.podsStatus[mp.Name]
			if u.status == config.Stop && mp.Status == config.Running {
				mp.Status = config.Stop
			}
			if u.status == config.Fail && mp.Status == config.Running {
				mp.Status = config.Fail
			}
			conf.Batches[i][j] = mp
		}
	}
	conf.Status = u.status
	_, err := config.UpdateUpgradeConfig(ctx, u.k8sClient, batchConfigName, conf)
	if err != nil {
		logger(fmt.Sprintf("failed to update upgrade status in config: %v\n", err))
	}
}

func (u *BatchUpgrade) triggerUpgrade(ctx context.Context, mp *config.MountPodUpgrade) error {
	cmds := []string{"juicefs-csi-driver", "upgrade", mp.Name}
	if !u.conf.NoRecreate {
		cmds = append(cmds, "--recreate")
	}
	req := u.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(mp.CSINodePod).
		Namespace(u.sysNamespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   cmds,
		Container: "juicefs-plugin",
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, k8scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(u.k8sConfig, "POST", req.URL())
	if err != nil {
		logger(fmt.Sprintf("failed to create SPDY executor: %v", err))
		return err
	}
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: u,
		Stderr: u,
		Tty:    true,
	}); err != nil {
		logger(fmt.Sprintf("failed to stream: %v", err))
		return err
	}
	return nil
}

func (u *BatchUpgrade) Write(p []byte) (n int, err error) {
	msg := string(p)
	fmt.Print(msg)

	successRegex := `POD-SUCCESS \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]`
	successRe := regexp.MustCompile(successRegex)

	successMatches := successRe.FindStringSubmatch(msg)
	if len(successMatches) > 1 {
		podName := successMatches[1]
		u.lock.Lock()
		u.podsStatus[podName] = config.Success
		u.lock.Unlock()
	}

	failRegex := `POD-FAIL \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]`
	failRe := regexp.MustCompile(failRegex)

	failMatches := failRe.FindStringSubmatch(msg)
	if len(failMatches) > 1 {
		podName := failMatches[1]
		u.lock.Lock()
		u.podsStatus[podName] = config.Fail
		u.lock.Unlock()
	}

	return len(p), nil
}

func logger(msg string) {
	fmt.Printf("%s %s\n", time.Now().Format(time.DateTime), msg)
}
