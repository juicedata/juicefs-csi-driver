/*
 Copyright 2023 Juicedata Inc

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
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard"
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

const (
	SysNamespaceKey = "SYS_NAMESPACE"
)

var (
	scheme = runtime.NewScheme()

	port      uint16
	devMode   bool
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
	cmd.PersistentFlags().StringVar(&staticDir, "static-dir", "", "static files to serve")

	goFlag := goflag.CommandLine
	klog.InitFlags(goFlag)
	cmd.PersistentFlags().AddGoFlagSet(goFlag)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run() {
	var config *rest.Config
	var err error
	sysNamespace := "kube-system"
	if devMode {
		config, err = getLocalConfig()
	} else {
		sysNamespace = os.Getenv(SysNamespaceKey)
		gin.SetMode(gin.ReleaseMode)
		config = ctrl.GetConfigOrDie()
	}
	if err != nil {
		log.Fatalf("can't get k8s config: %v", err)
	}
	mgr, err := newManager(config)
	if err != nil {
		log.Fatalf("can't create manager: %v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("can't create k8s client: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	podApi := dashboard.NewAPI(ctx, sysNamespace, mgr.GetClient(), client)
	router := gin.Default()
	if devMode {
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
		router.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/app")
		})
		router.GET("/app/*path", func(c *gin.Context) {
			path := c.Param("path")
			if strings.Contains(path, "..") {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			f, err := os.Stat(filepath.Join(staticDir, path))
			if os.IsNotExist(err) || f.IsDir() {
				c.File(filepath.Join(staticDir, "index.html"))
				return
			}
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
			}
			c.File(filepath.Join(staticDir, path))
		})
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
	go func() {
		if err := podApi.StartManager(ctx, mgr); err != nil {
			klog.Errorf("manager start error: %v", err)
		}
		quit <- syscall.SIGTERM
	}()

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
}

func getLocalConfig() (*rest.Config, error) {
	home := homedir.HomeDir()
	if home == "" {
		home = "/root"
	}
	return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
}

func newManager(conf *rest.Config) (ctrl.Manager, error) {
	return ctrl.NewManager(conf, ctrl.Options{
		Scheme:             scheme,
		Port:               9442,
		MetricsBindAddress: "0.0.0.0:8082",
		LeaderElectionID:   "dashboard.juicefs.com",
		NewCache: cache.BuilderWithOptions(cache.Options{
			Scheme: scheme,
		}),
	})
}
