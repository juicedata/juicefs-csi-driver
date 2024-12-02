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
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
)

var (
	batchConfigName = "juicefs-upgrade-batch"
)

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
			log.Error(err, "Failed to create kubernetes clientset")
			os.Exit(1)
		}

		k8sClient, err := k8sclient.NewClientWithConfig(k8sconfig)
		if err != nil {
			log.Error(err, "Could not create k8s client")
			os.Exit(1)
		}

		conf, err := config.LoadUpgradeConfig(context.Background(), k8sClient, batchConfigName)
		if err != nil {
			log.Error(err, "Failed to load upgrade config")
			os.Exit(1)
		}

		podsStatus := make(map[string]config.UpgradeStatus)
		for _, batch := range conf.Batches {
			for _, pods := range batch {
				podsStatus[pods.Name] = config.Pending
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
		}
		bu.Run(context.Background())
	},
}

func init() {
	upgradeCmd.Flags().StringVar(&batchConfigName, "batchConfigName", "juicefs-upgrade-batch", "configmap name for batch upgrade")
}

type BatchUpgrade struct {
	sysNamespace string
	conf         *config.BatchConfig
	k8sConfig    *rest.Config
	k8sClient    *k8sclient.K8sClient
	clientset    *kubernetes.Clientset

	lock       sync.Mutex
	podsStatus map[string]config.UpgradeStatus
}

func (u *BatchUpgrade) Run(ctx context.Context) {
	for _, batch := range u.conf.Batches {
		var (
			limiter  = make(chan struct{}, u.conf.Parallel)
			resultCh = make(chan error)
			wg       sync.WaitGroup
		)
		go func() {
			defer func() {
				wg.Wait()
				close(resultCh)
				close(limiter)
			}()
			for _, mp := range batch {
				mp2 := mp
				select {
				case <-ctx.Done():
					return
				case limiter <- struct{}{}:
				}

				wg.Add(1)
				go func() {
					defer func() {
						wg.Done()
						<-limiter
					}()
					resultCh <- u.triggerUpgrade(ctx, &mp2)
				}()
			}
		}()

		for oneErr := range resultCh {
			if oneErr != nil {
				if !u.conf.IgnoreError {
					fmt.Printf("%s BATCH-FAIL some pods upgrade failed\n", time.Now().Format("2006-01-02 15:04:05"))
					return
				}
			}
		}

		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			log.Error(ctx.Err(), "upgrade timeout")
			fmt.Printf("%s BATCH-FAIL upgrade timeout\n", time.Now().Format("2006-01-02 15:04:05"))
			return
		}

		for _, status := range u.podsStatus {
			if status == config.Fail && !u.conf.IgnoreError {
				fmt.Printf("%s BATCH-FAIL some pods upgrade failed\n", time.Now().Format("2006-01-02 15:04:05"))
				return
			}
		}
	}
	for _, status := range u.podsStatus {
		if status == config.Fail && !u.conf.IgnoreError {
			fmt.Printf("%s BATCH-FAIL some pods upgrade failed\n", time.Now().Format("2006-01-02 15:04:05"))
			return
		}
	}
	fmt.Printf("%s BATCH-SUCCESS all pods upgraded successfully\n", time.Now().Format("2006-01-02 15:04:05"))
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
		log.Error(err, "Failed to create SPDY executor")
		return err
	}
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: u,
		Stderr: u,
		Tty:    true,
	}); err != nil {
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
