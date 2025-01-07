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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	k8scheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
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
		config.Namespace = sysNamespace
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
		if err := bu.fetchPods(ctx); err != nil {
			logger("BATCH-FAIL failed to fetch pods")
			os.Exit(1)
		}
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

	batches         [][]*PodUpgrade
	lock            sync.Mutex
	podsStatus      map[string]config.UpgradeStatus
	status          config.UpgradeStatus
	crtBatchStatus  config.UpgradeStatus
	nextBatchStatus config.UpgradeStatus
	crtBatch        int
}

type PodUpgrade struct {
	pod         *corev1.Pod
	hashVal     string
	upgradeUUID string
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

func (u *BatchUpgrade) fetchPods(ctx context.Context) error {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.PodTypeValue,
		},
	}
	podLists, err := u.k8sClient.ListPod(ctx, config.Namespace, labelSelector, nil)
	if err != nil {
		log.Error(err, "reconcile ListPod error")
		return err
	}
	podMap := make(map[string]*corev1.Pod)
	for _, pod := range podLists {
		po := pod
		podMap[pod.Name] = &po
	}

	u.batches = make([][]*PodUpgrade, len(u.conf.Batches))
	for i, batch := range u.conf.Batches {
		pods := make([]*PodUpgrade, 0)
		for _, pu := range batch {
			po := podMap[pu.Name]
			if po == nil {
				continue
			}
			pods = append(pods, &PodUpgrade{
				pod:         po,
				hashVal:     po.Labels[common.PodJuiceHashLabelKey],
				upgradeUUID: resource.GetUpgradeUUID(po),
			})
		}
		u.batches[i] = pods
	}
	return nil
}

func (u *BatchUpgrade) processBatch(ctx context.Context) {
	if u.crtBatch > len(u.conf.Batches) {
		return
	}
	u.setCrtBatchStatus(config.Running)
	var (
		wg                  sync.WaitGroup
		batch               = u.conf.Batches[u.crtBatch-1]
		crtBatchFinalStatus = config.Success
		csiNodeNames        = make(map[string]string)
	)
	// trigger upgrade in each csi node only one time
	for _, mp := range batch {
		csiNodeNames[mp.CSINodePod] = mp.Node
	}
	resultCh := make(chan error, len(csiNodeNames))
	go func() {
		defer func() {
			wg.Wait()
			close(resultCh)
		}()
		for csiNode, node := range csiNodeNames {
			wg.Add(1)
			go func() {
				resultCh <- u.triggerUpgrade(ctx, csiNode, batchConfigName, u.crtBatch)
				needWait := false
				for _, p := range u.batches[u.crtBatch-1] {
					if u.podsStatus[p.pod.Name] != config.Success && u.podsStatus[p.pod.Name] != config.Fail {
						needWait = true
						break
					}
				}
				if needWait {
					u.waitForUpgrade(ctx, u.crtBatch, node)
				}

				wg.Done()
			}()
		}
	}()
	for oneErr := range resultCh {
		// pod upgrade error:
		// 1. trigger upgrade failed
		// 2. pod upgrade failed which is parsed in log stream
		if oneErr != nil {
			crtBatchFinalStatus = config.Fail
		}
		for _, s := range u.podsStatus {
			if s == config.Fail {
				crtBatchFinalStatus = config.Fail
			}
		}
	}
	u.setCrtBatchStatus(crtBatchFinalStatus)
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

func (u *BatchUpgrade) triggerUpgrade(ctx context.Context, csiNode string, configName string, crtBatchIndex int) error {
	cmds := []string{"juicefs-csi-driver", "upgrade", "BATCH", "--batchConfig", configName, "--batchIndex", fmt.Sprintf("%d", crtBatchIndex)}
	if !u.conf.NoRecreate {
		cmds = append(cmds, "--recreate")
	}
	req := u.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(csiNode).
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

func (u *BatchUpgrade) waitForUpgrade(ctx context.Context, index int, nodeName string) {
	ctx, cancel := context.WithTimeout(ctx, 1200*time.Second)
	defer cancel()

	var (
		successSum = make(map[string]bool)
		failSum    = make(map[string]bool)
		crtBatch   = u.batches[index-1]
	)

	for _, p := range crtBatch {
		if u.podsStatus[p.pod.Name] == config.Fail {
			failSum[p.pod.Name] = true
		}
		if u.podsStatus[p.pod.Name] == config.Success {
			successSum[p.pod.Name] = true
		}
	}

	stop := make(chan struct{})
	defer close(stop)

	labelSelector, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.PodTypeValue,
		},
	})
	watchlist := cache.NewFilteredListWatchFromClient(
		u.clientset.CoreV1().RESTClient(),
		"pods",
		config.Namespace,
		func(options *metav1.ListOptions) {
			options.ResourceVersion = "0"
			options.FieldSelector = fields.Set{"spec.nodeName": nodeName}.String()
			options.LabelSelector = labelSelector.String()
		},
	)
	handle := func(obj interface{}) {
		if obj == nil {
			return
		}
		po, ok := obj.(*corev1.Pod)
		if !ok {
			return
		}
		var pu *PodUpgrade
		for _, p := range crtBatch {
			if p.pod.Labels[common.PodUpgradeUUIDLabelKey] == po.Labels[common.PodUpgradeUUIDLabelKey] {
				pu = p
				break
			}
		}
		if pu == nil {
			return
		}
		if po.Name != pu.pod.Name {
			if po.DeletionTimestamp == nil && !resource.IsPodComplete(po) {
				if resource.IsPodReady(po) && !successSum[pu.pod.Name] {
					u.lock.Lock()
					u.podsStatus[pu.pod.Name] = config.Success
					u.lock.Unlock()
					successSum[pu.pod.Name] = true
					logger(fmt.Sprintf("POD-SUCCESS [%s] Upgrade mount pod and recreate one: %s !", pu.pod.Name, po.Name))
					return
				}
			}
		}
		if po.Name == pu.pod.Name {
			if resource.IsPodComplete(po) {
				logger(fmt.Sprintf("Mount pod %s received signal and completed", pu.pod.Name))
				return
			}
			if po.DeletionTimestamp != nil {
				logger(fmt.Sprintf("Mount pod %s is deleted", pu.pod.Name))
				return
			}
		}
	}
	_, controller := cache.NewInformerWithOptions(cache.InformerOptions{
		ListerWatcher: watchlist,
		ObjectType:    &corev1.Pod{},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				handle(obj)
			},
			DeleteFunc: func(obj interface{}) {
				handle(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				handle(newObj)
			},
		},
	})
	go controller.Run(stop)

	for {
		select {
		case <-ctx.Done():
			if len(successSum) == len(crtBatch) {
				logger(fmt.Sprintf("CRT-BATCH-SUCCESS all pods of current batch upgrade success in node %s", nodeName))
				return
			}
			for _, p := range crtBatch {
				if u.podsStatus[p.pod.Name] != config.Success {
					u.lock.Lock()
					u.podsStatus[p.pod.Name] = config.Fail
					u.lock.Unlock()
					logger(fmt.Sprintf("POD-FAIL [%s] node may be busy, upgrade mount pod timeout, please check it later manually.", p.pod.Name))
				}
			}
			logger(fmt.Sprintf("CRT-BATCH-FAIL pods of current batch upgrade timeout in node %s", nodeName))
			return
		default:
			if len(successSum) == len(crtBatch) {
				logger(fmt.Sprintf("CRT-BATCH-SUCCESS all pods of current batch upgrade success in node %s", nodeName))
				return
			}
			if len(failSum) > 0 && len(failSum)+len(successSum) == len(crtBatch) {
				logger(fmt.Sprintf("CRT-BATCH-FAIL some pods of current batch upgrade failed in node %s", nodeName))
				return
			}
		}
	}
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
