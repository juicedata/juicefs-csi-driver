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
			sysNamespace: sysNamespace,
			conf:         conf,
			k8sConfig:    k8sconfig,
			k8sClient:    k8sClient,
			clientset:    clientset,
			lock:         sync.Mutex{},
			podsStatus:   podsStatus,
			status:       config.Running,
		}
		bu.updateStatus(context.TODO())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pauseChan := make(chan bool)
		stopCh := make(chan struct{})
		defer func() {
			close(pauseChan)
			close(stopCh)
		}()

		go handleSignal(pauseChan, stopCh)
		bu.Run(ctx, pauseChan, stopCh)
	},
}

func handleSignal(pauseChan chan bool, stopCh chan struct{}) {
	sigChan := make(chan os.Signal, 10)
	signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGTERM)

	paused := false
	for {
		sig := <-sigChan
		if sig == syscall.SIGUSR1 {
			paused = !paused
			if paused {
				logger("Pause upgrade...")
			} else {
				logger("Resuming upgrade...")
			}
			pauseChan <- paused
		}
		if sig == syscall.SIGTERM {
			logger("Stop upgrade...")
			stopCh <- struct{}{}
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

	lock       sync.Mutex
	podsStatus map[string]config.UpgradeStatus
	status     config.UpgradeStatus
}

func (u *BatchUpgrade) Run(ctx context.Context, pauseChan chan bool, stopCh chan struct{}) {
	if u.conf.Parallel > 50 {
		logger("BATCH-FAIL parallel should not exceed 50")
		u.status = config.Fail
		u.panic(ctx)
	}
	var nextBatch = make(chan struct{})
	for _, batch := range u.conf.Batches {
		go u.processBatch(ctx, batch, nextBatch)

		select {
		case <-ctx.Done():
			logger("BATCH-FAIL upgrade timeout")
			u.status = config.Fail
			u.panic(ctx)
			return
		case <-stopCh:
			u.status = config.Stop
			u.updateStatus(ctx)
			return
		case paused := <-pauseChan:
			if paused {
				u.status = config.Pause
				u.updateStatus(ctx)

				select {
				case <-stopCh:
					u.status = config.Stop
					u.updateStatus(ctx)
					return
				case <-pauseChan:
					u.status = config.Running
					u.updateStatus(ctx)
					<-nextBatch
				}
			}
		case <-nextBatch:
		}
	}
	if u.status == config.Fail {
		// handle upgrade error when ignoreError is true
		logger("BATCH-FAIL some pods upgrade failed")
		u.panic(ctx)
	}
	logger("BATCH-SUCCESS all pods upgraded successfully")
	u.status = config.Success
	u.updateStatus(ctx)
}

func (u *BatchUpgrade) processBatch(ctx context.Context, batch []config.MountPodUpgrade, nextBatch chan struct{}) {
	var (
		wg       sync.WaitGroup
		resultCh = make(chan error, u.conf.Parallel)
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
		isErr := oneErr != nil
		if !isErr {
			for _, s := range u.podsStatus {
				if s == config.Fail {
					u.status = config.Fail
					break
				}
			}
		}
		if isErr && !u.conf.IgnoreError {
			logger("BATCH-FAIL some pods upgrade failed")
			u.panic(ctx)
		}
	}
	nextBatch <- struct{}{}
}

func (u *BatchUpgrade) panic(ctx context.Context) {
	u.updateStatus(ctx)
	os.Exit(1)
}

func (u *BatchUpgrade) updateStatus(ctx context.Context) {
	u.lock.Lock()
	defer u.lock.Unlock()
	conf := u.conf
	for i, batch := range conf.Batches {
		for j, mp := range batch {
			mp.Status = u.podsStatus[mp.Name]
			conf.Batches[i][j] = mp
		}
	}
	conf.Status = u.status
	_, err := config.SaveUpgradeConfig(ctx, u.k8sClient, batchConfigName, conf)
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
