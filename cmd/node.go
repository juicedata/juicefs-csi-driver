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
	"time"

	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/util/retry"

	"github.com/juicedata/juicefs-csi-driver/cmd/app"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/controller"
	"github.com/juicedata/juicefs-csi-driver/pkg/driver"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/grace"
	"github.com/juicedata/juicefs-csi-driver/pkg/fuse/passfd"
	k8s "github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

func parseNodeConfig() {
	config.ByProcess = process
	if os.Getenv("DRIVER_NAME") != "" {
		config.DriverName = os.Getenv("DRIVER_NAME")
	}

	if jfsImmutable := os.Getenv("JUICEFS_IMMUTABLE"); jfsImmutable != "" {
		if immutable, err := strconv.ParseBool(jfsImmutable); err == nil {
			config.Immutable = immutable
		} else {
			log.Error(err, "cannot parse JUICEFS_IMMUTABLE")
		}
	}
	config.NodeName = os.Getenv("NODE_NAME")
	config.Namespace = os.Getenv("JUICEFS_MOUNT_NAMESPACE")
	config.PodName = os.Getenv("POD_NAME")
	config.MountPointPath = os.Getenv("JUICEFS_MOUNT_PATH")
	config.JFSConfigPath = os.Getenv("JUICEFS_CONFIG_PATH")
	config.HostIp = os.Getenv("HOST_IP")
	config.KubeletPort = os.Getenv("KUBELET_PORT")
	jfsMountPriorityName := os.Getenv("JUICEFS_MOUNT_PRIORITY_NAME")
	jfsMountPreemptionPolicy := os.Getenv("JUICEFS_MOUNT_PREEMPTION_POLICY")
	if timeout := os.Getenv("JUICEFS_RECONCILE_TIMEOUT"); timeout != "" {
		duration, _ := time.ParseDuration(timeout)
		if duration > config.ReconcileTimeout {
			config.ReconcileTimeout = duration
		}
	}
	if interval := os.Getenv("JUICEFS_CONFIG_UPDATE_INTERVAL"); interval != "" {
		duration, _ := time.ParseDuration(interval)
		if duration > config.SecretReconcilerInterval {
			config.SecretReconcilerInterval = duration
		}
	}

	if jfsMountPriorityName != "" {
		config.JFSMountPriorityName = jfsMountPriorityName
	}

	if jfsMountPreemptionPolicy != "" {
		config.JFSMountPreemptionPolicy = jfsMountPreemptionPolicy
	}

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

	if config.PodName == "" || config.Namespace == "" {
		log.Info("Pod name & namespace can't be null.")
		os.Exit(1)
	}
	config.ReconcilerInterval = reconcilerInterval
	if config.ReconcilerInterval < 5 {
		config.ReconcilerInterval = 5
	}

	k8sclient, err := k8s.NewClient()
	if err != nil {
		log.Error(err, "Can't get k8s client")
		os.Exit(1)
	}
	pod, err := k8sclient.GetPod(context.TODO(), config.PodName, config.Namespace)
	if err != nil {
		log.Error(err, "Can't get pod", "pod", config.PodName)
		os.Exit(1)
	}
	config.CSIPod = *pod

	passfd.InitGlobalFds(context.TODO(), k8sclient, "/tmp")

	err = grace.ServeGfShutdown(config.ShutdownSockPath)
	if err != nil {
		log.Error(err, "Serve graceful shutdown error")
		os.Exit(1)
	}
}

func nodeRun(ctx context.Context) {
	parseNodeConfig()
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

	// enable pod manager in csi node
	if !process && podManager {
		if config.KubeletPort != "" && config.HostIp != "" {
			err := retry.OnError(retry.DefaultBackoff, func(err error) bool {
				log.Error(err, "Could not Start Reconciler of polling kubelet, retrying...")
				return true
			}, func() error {
				return controller.StartReconciler()
			})
			if err != nil {
				log.Error(err, "Could not Start Reconciler of polling kubelet and fallback to watch ApiServer.")
			} else {
				config.AccessToKubelet = true
			}
		}

		if !config.AccessToKubelet {
			go func() {
				mgr, err := app.NewPodManager()
				if err != nil {
					log.Error(err, "fail to create pod manager")
					os.Exit(1)
				}

				if err := mgr.Start(ctx); err != nil {
					log.Error(err, "fail to start pod manager")
					os.Exit(1)
				}
			}()
		}
		log.Info("Pod Reconciler Started")
	}

	registerer.MustRegister(collectors.NewGoCollector())
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
