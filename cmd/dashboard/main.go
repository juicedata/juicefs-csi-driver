/*
Copyright 2023 The Kubernetes Authors.

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
	goflag "flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

const (
	SysNamespaceKey = "SYS_NAMESPACE"
)

var (
	port      uint16
	devMode   bool
	mockMode  bool
	staticDir string
)

func main() {
	var cmd = &cobra.Command{
		Use:   "juicefs-csi-dashboard",
		Short: "dashboard of juicefs csi driver",
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}

	cmd.PersistentFlags().Uint16Var(&port, "port", 8088, "port to listen on")
	cmd.PersistentFlags().BoolVar(&devMode, "dev", false, "enable dev mode")
	cmd.PersistentFlags().BoolVar(&mockMode, "mock", false, "enable mock mode")
	cmd.PersistentFlags().StringVar(&staticDir, "static-dir", "", "static files to serve")

	goFlag := goflag.CommandLine
	klog.InitFlags(goFlag)
	cmd.PersistentFlags().AddGoFlagSet(goFlag)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run() {
	var client *k8sclient.K8sClient
	var err error
	sysNamespace := "kube-system"
	if mockMode {
		client = getMockClient()
	} else if devMode {
		client, err = getLocalClient()
	} else {
		sysNamespace = os.Getenv(SysNamespaceKey)
		gin.SetMode(gin.ReleaseMode)
		client, err = k8sclient.NewClient()
	}
	if err != nil {
		log.Fatalf("can't get k8s client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	podApi := dashboard.NewAPI(ctx, sysNamespace, client)
	router := gin.Default()
	if mockMode || devMode {
		router.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"*"},
			AllowHeaders:     []string{"*"},
			ExposeHeaders:    []string{"*"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}
	if staticDir != "" {
		router.StaticFile("/", filepath.Join(staticDir, "index.html"))
		router.Static("/static", staticDir)
	}
	podApi.Handle(router.Group("/api/v1"))
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Printf("listen on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	go func() {
		// pprof server
		log.Println(http.ListenAndServe("localhost:8089", nil))
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
}

func getMockClient() *k8sclient.K8sClient {
	client := &k8sclient.K8sClient{Interface: fake.NewSimpleClientset()}
	for _, pod := range dashboard.MockPods {
		_, _ = client.CreatePod(context.TODO(), &pod)
	}
	return client
}

func getLocalClient() (*k8sclient.K8sClient, error) {
	home := homedir.HomeDir()
	if home == "" {
		home = "/root"
	}
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to build config from flags")
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}
	return &k8sclient.K8sClient{Interface: client}, nil
}
