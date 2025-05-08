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
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/juicedata/juicefs-csi-driver/cmd/app"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var (
	scheme = runtime.NewScheme()
)

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
	if !config.Webhook {
		// When not in sidecar mode, we should inherit attributes from CSI Node pod.
		k8sclient, err := k8s.NewClient()
		if err != nil {
			log.Error(err, "Can't get k8s client")
			os.Exit(1)
		}

		CSINodeDsName := "juicefs-csi-node"
		if name := os.Getenv("JUICEFS_CSI_NODE_DS_NAME"); name != "" {
			CSINodeDsName = name
		}
		labelSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app": "juicefs-csi-node",
			},
		}

		pods, err := k8sclient.ListPod(context.TODO(), config.Namespace, labelSelector, nil)
		if err != nil {
			log.Error(err, "Can't get CSI pods")
			os.Exit(1)
		}
		var csiPod *corev1.Pod
		for i := range pods {
			isCSINodePod := false
			// Ensure the pod is managed by the expected DaemonSet
			for _, ownerRef := range pods[i].OwnerReferences {
				if ownerRef.Kind == "DaemonSet" && ownerRef.Name == CSINodeDsName {
					isCSINodePod = true
					csiPod = &pods[i]
					break
				}
			}
			if isCSINodePod {
				break
			}
		}

		if csiPod != nil {
			config.CSIPod = corev1.Pod{
				Spec: csiPod.Spec,
			}
			log.Info("Get CSI pod successfully", "pod", csiPod.Name)
		} else {
			log.Error(nil, "Can't get CSI pod managed by DaemonSet", "ds", CSINodeDsName)
			os.Exit(1)
		}
	}
}

func controllerRun(ctx context.Context) {
	parseControllerConfig()
	if nodeID == "" {
		log.Info("nodeID must be provided")
		os.Exit(1)
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
