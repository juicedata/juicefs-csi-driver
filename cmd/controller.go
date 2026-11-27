/*
 Copyright 2022 Juicedata Inc

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
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/juicedata/juicefs-csi-driver/cmd/app"
	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var (
	scheme = runtime.NewScheme()
)

const csiNodePodRefreshInterval = time.Minute

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
}
func parseControllerConfig() {
	config.ByProcess = process
	config.Webhook = webhook
	config.Provisioner = provisioner
	config.CacheClientConf = cacheConf
	config.ValidatingWebhook = validationWebhook
	if os.Getenv("DRIVER_NAME") != "" {
		config.DriverName = os.Getenv("DRIVER_NAME")
	}
	// enable mount manager by default in csi controller
	config.MountManager = true
	if process {
		config.MountManager = false
		config.Webhook = false
		config.Provisioner = false
		return
	}
	if jfsImmutable := os.Getenv("JUICEFS_IMMUTABLE"); jfsImmutable != "" {
		// check if running in an immutable environment
		if immutable, err := strconv.ParseBool(jfsImmutable); err == nil {
			config.Immutable = immutable
		} else {
			log.Error(err, "cannot parse JUICEFS_IMMUTABLE")
			os.Exit(1)
		}
	}

	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")

	if mountPodImage := os.Getenv("JUICEFS_CE_MOUNT_IMAGE"); mountPodImage != "" {
		config.DefaultCEMountImage = mountPodImage
	}
	if mountPodImage := os.Getenv("JUICEFS_EE_MOUNT_IMAGE"); mountPodImage != "" {
		config.DefaultEEMountImage = mountPodImage
	}
	if mountPodImage := os.Getenv("JUICEFS_MOUNT_IMAGE"); mountPodImage != "" {
		// check if it's CE or EE
		hasCE, hasEE := util.ImageResol(mountPodImage)
		if hasCE {
			config.DefaultCEMountImage = mountPodImage
		}
		if hasEE {
			config.DefaultEEMountImage = mountPodImage
		}
	}
	if os.Getenv("STORAGE_CLASS_SHARE_MOUNT") == "true" {
		config.StorageClassShareMount = true
	}
	if os.Getenv("FS_SHARE_MOUNT") == "true" {
		config.FSShareMount = true
	}

	if v := os.Getenv("PROVISION_WORKER_THREADS"); v != "" {
		if threads, err := strconv.Atoi(v); err == nil {
			config.ProvisionWorkerThreads = threads
		} else {
			log.Error(err, "cannot parse PROVISION_WORKER_THREADS")
			os.Exit(1)
		}
	}
}

// errNoCSINode means the namespace runs no CSI node, which is expected in sidecar mode.
var errNoCSINode = errors.New("no CSI node pod or DaemonSet")

// getCSINodePod returns the CSI node pod whose spec mount pods and jobs inherit. A Ready pod is
// preferred over the DaemonSet template: its affinity keeps the DaemonSet's node pin, so jobs land
// on a node that runs a healthy CSI node, and it carries the image pull secrets that
// ServiceAccount admission added.
func getCSINodePod(ctx context.Context, client *k8s.K8sClient, namespace string) (*corev1.Pod, error) {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{common.CSINodeLabelKey: common.CSINodeLabelValue},
	}
	pods, err := client.ListPod(ctx, namespace, labelSelector, nil)
	if err != nil {
		return nil, fmt.Errorf("list CSI node pods: %w", err)
	}
	for i := range pods {
		if pods[i].DeletionTimestamp == nil && resource.IsPodReady(&pods[i]) {
			return &pods[i], nil
		}
	}

	// No Ready CSI node pod, e.g. its node pool is scaled to zero.
	dsName := "juicefs-csi-node"
	if name := os.Getenv("JUICEFS_CSI_NODE_DS_NAME"); name != "" {
		dsName = name
	}
	ds, err := client.GetDaemonSet(ctx, dsName, namespace)
	if err == nil {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: ds.Name, Namespace: ds.Namespace},
			Spec:       ds.Spec.Template.Spec,
		}, nil
	}
	if !k8serrors.IsNotFound(err) {
		return nil, fmt.Errorf("get CSI node DaemonSet %s: %w", dsName, err)
	}
	// The CSI node may run under another workload kind, e.g. an OpenKruise Advanced DaemonSet.
	if len(pods) > 0 {
		return &pods[0], nil
	}
	return nil, fmt.Errorf("%w: no pod labeled %s=%s and no DaemonSet %s in namespace %s",
		errNoCSINode, common.CSINodeLabelKey, common.CSINodeLabelValue, dsName, namespace)
}

// inheritCSINodePod sets the CSI node pod at startup and refreshes it every interval, so a removed
// node or a changed DaemonSet spec doesn't stick to every job created afterwards.
func inheritCSINodePod(ctx context.Context, client *k8s.K8sClient, interval time.Duration) error {
	pod, err := getCSINodePod(ctx, client, config.Namespace)
	switch {
	case err == nil:
		config.SetCSIPod(pod)
		log.Info("Get CSI node pod successfully", "pod", pod.Name)
	case errors.Is(err, errNoCSINode) && config.Webhook:
		log.Info("No CSI node to inherit attributes from", "reason", err.Error())
	default:
		return err
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			pod, err := getCSINodePod(ctx, client, config.Namespace)
			if err != nil {
				if !errors.Is(err, errNoCSINode) || !config.Webhook {
					log.Error(err, "Can't refresh CSI node pod, keeping the previous one")
				}
				continue
			}
			if pod.Name != config.GetCSIPod().Name {
				log.Info("Switch inherited CSI node pod", "pod", pod.Name)
			}
			config.SetCSIPod(pod)
		}
	}()
	return nil
}

func controllerRun(ctx context.Context) {
	parseControllerConfig()
	if nodeID == "" {
		log.Info("nodeID must be provided")
		os.Exit(1)
	}
	if !config.ByProcess {
		k8sclient, err := k8s.NewClient()
		if err != nil {
			log.Error(err, "Can't get k8s client")
			os.Exit(1)
		}
		if err := inheritCSINodePod(ctx, k8sclient, csiNodePodRefreshInterval); err != nil {
			log.Error(err, "Can't get CSI node pod or DaemonSet")
			os.Exit(1)
		}
	}

	// http server for pprof
	go func() {
		port := 6060
		for {
			if err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil); err != nil {
				log.Error(err, "failed to start pprof server")
				os.Exit(1)
			}
			port++
		}
	}()

	registerer, registry := util.NewPrometheus(config.NodeName)
	// http server for metrics
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				// Opt into OpenMetrics to support exemplars.
				EnableOpenMetrics: true,
			},
		))
		server := &http.Server{
			Addr:    fmt.Sprintf(":%d", config.WebPort),
			Handler: mux,
		}
		if err := server.ListenAndServe(); err != nil {
			log.Error(err, "failed to start metrics server")
		}
	}()

	if config.MountManager || config.Webhook {
		mgr, err := app.NewControllerManager(
			config.MountManager,
			config.Webhook,
			leaderElection,
			leaderElectionNamespace,
			leaderElectionLeaseDuration,
			certDir,
			webhookPort,
		)
		if err != nil {
			log.Error(err, "initialize controller manager failed")
			os.Exit(1)
		}

		go func() {
			if err := mgr.Start(ctx); err != nil {
				log.Error(err, "fail to start controller manager")
				os.Exit(1)
			}
		}()
	}

	drv, err := driver.NewDriver(endpoint, nodeID, leaderElection, leaderElectionNamespace, leaderElectionLeaseDuration, registerer)
	if err != nil {
		log.Error(err, "fail to create driver")
		os.Exit(1)
	}
	go func() {
		<-ctx.Done()
		drv.Stop()
	}()
	if err := drv.Run(); err != nil {
		log.Error(err, "fail to run driver")
		os.Exit(1)
	}
}
