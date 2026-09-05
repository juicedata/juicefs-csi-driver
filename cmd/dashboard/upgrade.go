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
	"strconv"
	"strings"
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
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/grace"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var batchConfigName string

const batchUpgradeTimeoutEnv = "BATCH_UPGRADE_TIMEOUT_SECONDS"
const defaultPodUpgradeTimeout = 300 * time.Second
const minPodUpgradeTimeout = 5 * time.Second

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "trigger upgrade mount pod smoothly",
	Run: func(cmd *cobra.Command, args []string) {
		config.DisableGraceUpgrade = strings.EqualFold(os.Getenv("DISABLE_GRACE_UPGRADE"), "true")
		if config.DisableGraceUpgrade {
			logger("BATCH-FAIL smooth upgrade is disabled")
			os.Exit(1)
		}
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

		bu := &BatchUpgrade{
			sysNamespace:      sysNamespace,
			conf:              conf,
			k8sConfig:         k8sconfig,
			k8sClient:         k8sClient,
			clientset:         clientset,
			podUpgradeTimeout: defaultPodUpgradeTimeout,
			lock:              sync.Mutex{},
			status:            config.Running,
			crtBatchStatus:    config.Pending,
			nextBatchStatus:   config.Pending,
			crtBatch:          0,
		}
		podsStatus := make(map[string]config.UpgradeStatus)
		for bi := range conf.Batches {
			for ti := range conf.Batches[bi] {
				target := &conf.Batches[bi][ti]
				if target.Status == "" {
					target.Status = config.Pending
				}
				podsStatus[target.Key()] = target.Status
			}
		}
		bu.podsStatus = podsStatus
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
	sysNamespace      string
	conf              *config.BatchConfig
	k8sConfig         *rest.Config
	k8sClient         *k8sclient.K8sClient
	clientset         *kubernetes.Clientset
	podUpgradeTimeout time.Duration

	batches         []map[string][]*PodUpgrade
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
	timeout, err := getBatchUpgradeTimeout()
	if err != nil {
		logger(fmt.Sprintf("BATCH-FAIL invalid %s: %v", batchUpgradeTimeoutEnv, err))
		os.Exit(1)
	}
	u.podUpgradeTimeout = timeout

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
			if u.getCrtBatchStatus() == config.Fail && !u.conf.IgnoreError {
				u.status = config.Fail
				handleFinalStatus()
				return
			}
			if u.crtBatch > len(u.conf.Batches) {
				u.status = u.finalUpgradeStatus()
				handleFinalStatus()
				return
			}
			switch u.getNextBatchStatus() {
			case config.Pending:
				if crtSt := u.getCrtBatchStatus(); crtSt == config.Pending || crtSt == config.Success || (crtSt == config.Fail && u.conf.IgnoreError) {
					u.crtBatch++
					if u.crtBatch > len(u.conf.Batches) {
						u.status = u.finalUpgradeStatus()
						handleFinalStatus()
						return
					}
					// Set Running synchronously so the next ticker tick won't launch
					// a second concurrent processBatch before this one starts.
					u.setCrtBatchStatus(config.Running)
					batchIdx := u.crtBatch
					go u.processBatch(ctx, batchIdx)
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

	u.batches = make([]map[string][]*PodUpgrade, len(u.conf.Batches))
	for i, batch := range u.conf.Batches {
		pods := make(map[string][]*PodUpgrade)
		for _, pu := range batch {
			po := podMap[pu.Name]
			if po == nil {
				continue
			}

			pods[pu.Node] = append(pods[pu.Node], &PodUpgrade{
				pod:         po,
				hashVal:     po.Labels[common.PodJuiceHashLabelKey],
				upgradeUUID: resource.GetUpgradeUUID(po),
			})
		}
		u.batches[i] = pods
	}
	return nil
}

func (u *BatchUpgrade) processBatch(ctx context.Context, batchIdx int) {
	var (
		wg                  sync.WaitGroup
		batch               = u.conf.Batches[batchIdx-1]
		crtBatchFinalStatus = config.Success
	)

	mountCsiNodeNames := make(map[string][]config.UpgradeTarget)
	if u.conf.Kind != config.UpgradeKindSidecar {
		for _, mp := range batch {
			mountCsiNodeNames[mp.CSINodePod] = append(mountCsiNodeNames[mp.CSINodePod], mp)
		}
	}

	resultCh := make(chan error, len(mountCsiNodeNames))
	if u.conf.Kind == config.UpgradeKindSidecar {
		resultCh = make(chan error, len(batch))
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if u.conf.Kind == config.UpgradeKindSidecar {
			u.processSidecarBatch(ctx, batch, resultCh)
			return
		}
		u.processPodBatch(ctx, mountCsiNodeNames, batchIdx, resultCh)
	}()
	go func() {
		wg.Wait()
		close(resultCh)
	}()
	for oneErr := range resultCh {
		if oneErr != nil {
			crtBatchFinalStatus = config.Fail
		}
	}

	// Only check status of targets in the current batch
	u.lock.Lock()
	for _, target := range batch {
		key := target.Key()
		if status, ok := u.podsStatus[key]; ok && status == config.Fail {
			crtBatchFinalStatus = config.Fail
			break
		}
	}
	u.lock.Unlock()
	u.setCrtBatchStatus(crtBatchFinalStatus)
}

func (u *BatchUpgrade) processPodBatch(ctx context.Context, csiNodeNames map[string][]config.UpgradeTarget, batchIdx int, resultCh chan<- error) {
	var (
		wg sync.WaitGroup
	)
	for csiNode, mps := range csiNodeNames {
		csiNode := csiNode
		mps := mps
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, reason := u.precheckNode(ctx, csiNode); !ok {
				for _, p := range mps {
					u.setPodStatus(p.Key(), config.Skip)
					logger(fmt.Sprintf("POD-SKIP [%s] %s", p.Key(), reason))
				}
				resultCh <- nil
				return
			}
			if err := u.triggerUpgrade(ctx, csiNode, batchConfigName, batchIdx); err != nil {
				resultCh <- err
				return
			}
			needWait := false
			node := ""
			for _, p := range mps {
				node = p.Node
				st := u.getPodStatus(p.Key())
				if st != config.Success && st != config.Fail {
					needWait = true
					break
				}
			}
			if needWait {
				u.waitForUpgrade(ctx, batchIdx, node, csiNode)
			}
			resultCh <- nil
		}()
	}
	wg.Wait()
}

func (u *BatchUpgrade) processSidecarBatch(ctx context.Context, targets []config.UpgradeTarget, resultCh chan<- error) {
	var (
		wg sync.WaitGroup
	)
	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := target.Key()
			if ok, reason := u.precheckSidecarTarget(ctx, target); !ok {
				u.setPodStatus(key, config.Skip)
				logger(fmt.Sprintf("POD-SKIP [%s] %s", key, reason))
				resultCh <- nil
				return
			}
			u.setPodStatus(key, config.Running)
			logger(fmt.Sprintf("POD-START [%s] start to upgrade", key))
			err := grace.RunSidecarUpgrade(ctx, u.k8sClient, grace.SidecarUpgradeTarget{
				Namespace:     u.sidecarNamespace(target),
				PodName:       target.Name,
				ContainerName: target.ContainerName,
			})
			if err != nil {
				u.setPodStatus(key, config.Fail)
				logger(fmt.Sprintf("POD-FAIL [%s] upgrade sidecar error: %v", key, err))
				resultCh <- err
				return
			}

			u.setPodStatus(key, config.Success)
			logger(fmt.Sprintf("POD-SUCCESS [%s] upgrade sidecar success", key))
			resultCh <- nil
		}()
	}
	wg.Wait()
}

func (u *BatchUpgrade) finalUpgradeStatus() config.UpgradeStatus {
	u.lock.Lock()
	defer u.lock.Unlock()
	for _, batch := range u.conf.Batches {
		for _, target := range batch {
			if u.podsStatus[target.Key()] == config.Fail {
				return config.Fail
			}
		}
	}
	return config.Success
}

func (u *BatchUpgrade) precheckSidecarTarget(ctx context.Context, target config.UpgradeTarget) (bool, string) {
	if target.Name == "" {
		return false, "skip upgrade: empty pod name"
	}
	namespace := u.sidecarNamespace(target)
	if namespace == "" {
		return false, "skip upgrade: empty pod namespace"
	}

	pod, err := u.k8sClient.GetPod(ctx, target.Name, namespace)
	if err != nil {
		return false, fmt.Sprintf("skip upgrade: can not get pod %s/%s: %s", namespace, target.Name, err.Error())
	}
	if pod.DeletionTimestamp != nil {
		return false, fmt.Sprintf("skip upgrade: pod %s/%s is terminating", namespace, target.Name)
	}
	if !resource.IsPodReady(pod) {
		return false, fmt.Sprintf("skip upgrade: pod %s/%s is not ready", namespace, target.Name)
	}
	return true, ""
}

// precheckNode verifies that the node and its CSI node pod are in a state that
// allows a smooth upgrade. It returns false (with a reason) when the upgrade of
// all pods on the node should be skipped:
//  1. the node is marked SchedulingDisabled (spec.unschedulable == true)
//  2. the CSI node pod on the node is not ready
//
// When the node or CSI node pod state cannot be confirmed due to an API error,
// it conservatively returns false so the node is skipped rather than upgraded.
func (u *BatchUpgrade) precheckNode(ctx context.Context, csiNode string) (bool, string) {
	if csiNode == "" {
		return false, "skip upgrade: no csi node pod found"
	}

	po, err := u.k8sClient.GetPod(ctx, csiNode, u.sysNamespace)
	if err != nil {
		return false, fmt.Sprintf("skip upgrade: can not get csi node pod %s: %s", csiNode, err.Error())
	}
	nodeName := po.Spec.NodeName
	if nodeName == "" {
		return false, fmt.Sprintf("skip upgrade: csi node pod %s has no node name", csiNode)
	}

	node, err := u.k8sClient.GetNode(ctx, nodeName)
	if err != nil {
		return false, fmt.Sprintf("skip upgrade: can not get node %s: %s", nodeName, err.Error())
	}
	if node.Spec.Unschedulable {
		return false, fmt.Sprintf("skip upgrade: node %s is marked SchedulingDisabled", nodeName)
	}

	if !resource.IsPodReady(po) {
		return false, fmt.Sprintf("skip upgrade: csi node pod %s on node %s is not ready", csiNode, nodeName)
	}

	return true, ""
}

func (u *BatchUpgrade) setCrtBatchStatus(s config.UpgradeStatus) {
	u.lock.Lock()
	u.crtBatchStatus = s
	u.lock.Unlock()
}

func (u *BatchUpgrade) getCrtBatchStatus() config.UpgradeStatus {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.crtBatchStatus
}

func (u *BatchUpgrade) setNextBatchStatus(s config.UpgradeStatus) {
	u.lock.Lock()
	u.nextBatchStatus = s
	u.lock.Unlock()
}

func (u *BatchUpgrade) getNextBatchStatus() config.UpgradeStatus {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.nextBatchStatus
}

func (u *BatchUpgrade) getPodStatus(name string) config.UpgradeStatus {
	u.lock.Lock()
	defer u.lock.Unlock()
	return u.podsStatus[name]
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
	// Overall status is maintained in conf.Status, and per-target status in target.Status/podsStatus.
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

func (u *BatchUpgrade) setPodStatus(key string, status config.UpgradeStatus) {
	u.lock.Lock()
	defer u.lock.Unlock()
	u.podsStatus[key] = status
	for bi := range u.conf.Batches {
		for ti := range u.conf.Batches[bi] {
			target := &u.conf.Batches[bi][ti]
			if target.Key() == key {
				target.Status = status
			}
		}
	}
}

func (u *BatchUpgrade) sidecarNamespace(target config.UpgradeTarget) string {
	if u.conf.Namespace != "" {
		return u.conf.Namespace
	}
	return target.Namespace
}

func (u *BatchUpgrade) waitForUpgrade(ctx context.Context, index int, nodeName, csiNode string) {
	ctx, cancel := context.WithTimeout(ctx, u.podUpgradeTimeout)
	defer cancel()
	timer := time.NewTicker(5 * time.Second)
	defer timer.Stop()
	var (
		successSum = make(map[string]bool)
		failSum    = make(map[string]bool)
		crtBatch   = u.batches[index-1][nodeName]
	)

	for _, p := range crtBatch {
		if u.getPodStatus(p.pod.Name) == config.Fail {
			failSum[p.pod.Name] = true
		}
		if u.getPodStatus(p.pod.Name) == config.Success {
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
			if resource.GetUpgradeUUID(p.pod) == resource.GetUpgradeUUID(po) {
				pu = p
				break
			}
		}
		if pu == nil {
			return
		}
		if po.Name != pu.pod.Name {
			if po.DeletionTimestamp == nil && !resource.IsPodComplete(po) && resource.IsPodReady(po) && !successSum[pu.pod.Name] {
				u.setPodStatus(pu.pod.Name, config.Success)
				successSum[pu.pod.Name] = true
				logger(fmt.Sprintf("POD-SUCCESS [%s] Upgrade mount pod and the new one is ready: %s !", pu.pod.Name, po.Name))
				return
			} else {
				logger(fmt.Sprintf("[%s] is upgraded and the new one is created: %s .", pu.pod.Name, po.Name))
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
				if u.getPodStatus(p.pod.Name) != config.Success {
					u.setPodStatus(p.pod.Name, config.Fail)
					failSum[p.pod.Name] = true
					logger(fmt.Sprintf("POD-FAIL [%s] node may be busy, upgrade mount pod timeout, please check it later manually.", p.pod.Name))
				}
			}
			logger(fmt.Sprintf("CRT-BATCH-FAIL %d pods of current batch upgrade failed in node %s, please check log of csi node: %s", len(failSum), nodeName, csiNode))
			return
		case <-timer.C:
			if len(successSum) == len(crtBatch) {
				logger(fmt.Sprintf("CRT-BATCH-SUCCESS all pods of current batch upgrade success in node %s", nodeName))
				return
			}
			if len(failSum) > 0 && len(failSum)+len(successSum) == len(crtBatch) {
				logger(fmt.Sprintf("CRT-BATCH-FAIL %d pods of current batch upgrade failed in node %s, please check log of csi node: %s", len(failSum), nodeName, csiNode))
				return
			}
			logger(fmt.Sprintf("CRT-BATCH-WAITING wait for %d pods of current batch upgrade in node %s", len(crtBatch)-len(failSum)-len(successSum), nodeName))
		}
	}
}

func getBatchUpgradeTimeout() (time.Duration, error) {
	value := os.Getenv(batchUpgradeTimeoutEnv)
	if value == "" {
		return defaultPodUpgradeTimeout, nil
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: must be an integer (seconds), got %q", batchUpgradeTimeoutEnv, value)
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout < minPodUpgradeTimeout {
		return 0, fmt.Errorf("%s must be at least %v", batchUpgradeTimeoutEnv, minPodUpgradeTimeout)
	}
	return timeout, nil
}

func (u *BatchUpgrade) Write(p []byte) (n int, err error) {
	msg := string(p)
	fmt.Print(msg)

	runningRegex := `POD-START \[([^\]]+)\]`
	runningRe := regexp.MustCompile(runningRegex)

	runningMatches := runningRe.FindAllStringSubmatch(msg, -1)
	for _, match := range runningMatches {
		podName := match[1]
		u.setPodStatus(podName, config.Running)
	}

	successRegex := `POD-SUCCESS \[([^\]]+)\]`
	successRe := regexp.MustCompile(successRegex)

	successMatches := successRe.FindAllStringSubmatch(msg, -1)
	for _, match := range successMatches {
		podName := match[1]
		u.setPodStatus(podName, config.Success)
	}

	failRegex := `POD-FAIL \[([^\]]+)\]`
	failRe := regexp.MustCompile(failRegex)

	failMatches := failRe.FindAllStringSubmatch(msg, -1)
	for _, match := range failMatches {
		podName := match[1]
		u.setPodStatus(podName, config.Fail)
	}

	skipRegex := `POD-SKIP \[([^\]]+)\]`
	skipRe := regexp.MustCompile(skipRegex)

	skipMatches := skipRe.FindAllStringSubmatch(msg, -1)
	for _, match := range skipMatches {
		podName := match[1]
		u.setPodStatus(podName, config.Skip)
	}

	return len(p), nil
}

func logger(msg string) {
	fmt.Printf("%s %s\n", time.Now().Format(time.DateTime), msg)
}
