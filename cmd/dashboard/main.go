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
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	juicefsiov1 "github.com/juicedata/juicefs-cache-group-operator/api/v1"

	jfsConfig "github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard"
)

func init() {
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(juicefsiov1.AddToScheme(scheme))
	// Initialize a logger for the controller runtime
	ctrllog.SetLogger(klog.NewKlogr())
	// To disable controller runtime logging, instead set the null logger:
	//log.SetLogger(logr.New(log.NullLogSink{}))

}

const (
	SysNamespaceKey = "SYS_NAMESPACE"
)

var (
	scheme = runtime.NewScheme()
	log    = klog.NewKlogr().WithName("main")

	port      uint16
	devMode   bool
	staticDir string

	leaderElection              bool
	leaderElectionNamespace     string
	leaderElectionLeaseDuration time.Duration

	enableManager bool

	// for basic auth
	USERNAME string
	PASSWORD string
)

func main() {
	var cmd = &cobra.Command{
		Use:   "juicefs-csi-dashboard",
		Short: "dashboard of juicefs csi driver",
		Run: func(cmd *cobra.Command, args []string) {
			run()
		},
	}

	cmd.AddCommand(upgradeCmd)

	if v := os.Getenv("USERNAME"); v != "" {
		USERNAME = v
	}
	if v := os.Getenv("PASSWORD"); v != "" {
		PASSWORD = v
	}
	cmd.PersistentFlags().Uint16Var(&port, "port", 8088, "port to listen on")
	cmd.PersistentFlags().BoolVar(&devMode, "dev", false, "enable dev mode")
	cmd.PersistentFlags().StringVar(&staticDir, "static-dir", "", "static files to serve")
	cmd.PersistentFlags().BoolVar(&leaderElection, "leader-election", false, "Enables leader election. If leader election is enabled, additional RBAC rules are required. ")
	cmd.PersistentFlags().StringVar(&leaderElectionNamespace, "leader-election-namespace", "", "Namespace where the leader election resource lives. Defaults to the pod namespace if not set.")
	cmd.PersistentFlags().DurationVar(&leaderElectionLeaseDuration, "leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
	cmd.PersistentFlags().BoolVar(&enableManager, "enable-manager", true, "enable manager for cache/index resource")

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
	if v := os.Getenv(SysNamespaceKey); v != "" {
		sysNamespace = v
	}
	jfsConfig.Namespace = sysNamespace
	if devMode {
		config, err = getLocalConfig()
	} else {
		gin.SetMode(gin.ReleaseMode)
		config = ctrl.GetConfigOrDie()
	}
	if err != nil {
		log.Error(err, "can't get k8s config")
		os.Exit(1)
	}

	var mgrClient client.Client
	var mgr ctrl.Manager
	if enableManager {
		mgr, err = newManager(config)
		if err != nil {
			log.Error(err, "can't create manager")
			os.Exit(1)
		}
		mgrClient = mgr.GetClient()
	} else {
		mgrClient, err = client.New(config, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			log.Error(err, "can't create client")
			os.Exit(1)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	podApi := dashboard.NewAPI(ctx, sysNamespace, mgrClient, config, enableManager)
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
	if USERNAME != "" && PASSWORD != "" {
		router.Use(gin.BasicAuth(gin.Accounts{
			USERNAME: PASSWORD,
		}))
	}
	podApi.Handle(router.Group("/api/v1"))
	if staticDir != "" {
		router.NoRoute(func(c *gin.Context) {
			if strings.Contains(c.Request.RequestURI, "assets/") {
				assertPath := strings.Split(c.Request.RequestURI, "assets/")[1]
				c.File(filepath.Join(staticDir, "assets", assertPath))
			}
			if !strings.HasPrefix(c.Request.RequestURI, "/api") {
				c.File(filepath.Join(staticDir, "index.html"))
			}
		})
	}

	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		log.Info("listen and serve", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(err, "listen error")
			os.Exit(1)
		}
	}()
	go func() {
		// pprof server
		err = http.ListenAndServe("localhost:8089", nil)
		if err != nil {
			log.Error(err, "pprof server error")
		}
	}()
	quit := make(chan os.Signal, 2)
	if enableManager {
		go func() {
			if err := podApi.StartManager(ctx, mgr); err != nil {
				log.Error(err, "manager start error")
			}
			quit <- syscall.SIGTERM
		}()
	}
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	go func() {
		log.Info("Shutdown Server ...")
		if err := srv.Shutdown(ctx); err != nil {
			log.Error(err, "Server Shutdown")
			os.Exit(1)
		}
		os.Exit(0)
	}()
	<-quit
	os.Exit(1) // second signal. Exit directly.
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
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0.0.0.0:8082",
		},
		LeaderElection:          leaderElection,
		LeaderElectionID:        "dashboard.juicefs.com",
		LeaderElectionNamespace: leaderElectionNamespace,
		LeaseDuration:           &leaderElectionLeaseDuration,
		Cache: cache.Options{
			Scheme: scheme,
		},
	})
}
