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

	if !config.Webhook {
		// When not in sidecar mode, we should inherit attributes from CSI Node.
		k8sclient, err := k8s.NewClient()
		if err != nil {
			log.Error(err, "Can't get k8s client")
			os.Exit(1)
		}
		csiPod, err := getCSINodePodTemplate(context.TODO(), k8sclient, config.Namespace)
		if err != nil {
			log.Error(err, "Can't get CSI DaemonSet or pods")
			os.Exit(1)
		}
		config.CSIPod = *csiPod
	}
}

// getCSINodePodTemplate returns the pod spec that jobs created by the controller inherit
// (nodeSelector, affinity, tolerations, ...). The DaemonSet template is preferred over a running
// CSI node pod: the DaemonSet controller pins each pod to its node, and a job copying that pin
// stays Pending forever once the node is removed from the cluster.
func getCSINodePodTemplate(ctx context.Context, client *k8s.K8sClient, namespace string) (*corev1.Pod, error) {
	csiNodeDSName := "juicefs-csi-node"
	if name := os.Getenv("JUICEFS_CSI_NODE_DS_NAME"); name != "" {
		csiNodeDSName = name
	}
	ds, dsErr := client.GetDaemonSet(ctx, csiNodeDSName, namespace)
	if dsErr == nil {
		log.Info("Get CSI DaemonSet successfully", "ds", csiNodeDSName)
		return &corev1.Pod{Spec: *ds.Spec.Template.Spec.DeepCopy()}, nil
	}
	log.Error(dsErr, "Can't get CSI DaemonSet, try CSI pods", "ds", csiNodeDSName)

	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "juicefs-csi-node",
		},
	}
	pods, err := client.ListPod(ctx, namespace, labelSelector, nil)
	if err != nil {
		return nil, err
	}
	if len(pods) == 0 {
		return nil, fmt.Errorf("no DaemonSet %s and no CSI node pods in namespace %s", csiNodeDSName, namespace)
	}
	csiPod := pods[0].DeepCopy()
	csiPod.Spec.NodeName = ""
	csiPod.Spec.Affinity = removeNodeNamePin(csiPod.Spec.Affinity)
	log.Info("Get CSI pod successfully", "pod", csiPod.Name)
	return csiPod, nil
}

// removeNodeNamePin drops the `metadata.name` field selector that the DaemonSet controller
// injects into each pod's required node affinity.
func removeNodeNamePin(affinity *corev1.Affinity) *corev1.Affinity {
	if affinity == nil || affinity.NodeAffinity == nil || affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return affinity
	}
	required := affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	var terms []corev1.NodeSelectorTerm
	for _, term := range required.NodeSelectorTerms {
		var fields []corev1.NodeSelectorRequirement
		for _, f := range term.MatchFields {
			if f.Key != metav1.ObjectNameField {
				fields = append(fields, f)
			}
		}
		term.MatchFields = fields
		if len(term.MatchExpressions) > 0 || len(term.MatchFields) > 0 {
			terms = append(terms, term)
		}
	}
	if len(terms) == 0 {
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = nil
	} else {
		required.NodeSelectorTerms = terms
	}
	return affinity
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
