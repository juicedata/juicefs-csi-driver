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

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

const (
	SysNamespaceKey = "SYS_NAMESPACE"
)

var (
	port    uint16
	devMode bool
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
	if devMode {
		client, err = getLocalConfig()
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
	podApi := newPodApi(ctx, sysNamespace, client)
	router := gin.Default()
	podApi.handle(router.Group("/api/v1"))

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

func (api *podApi) handle(group *gin.RouterGroup) {
	group.GET("/pods", api.listAppPod())
	group.GET("/mountpods", api.listMountPod())
	group.GET("/csi-nodes", api.listCSINodePod())
	group.GET("/controllers", api.listCSIControllerPod())
	podGroup := group.Group("/pod/:namespace/:name", api.getPodMiddileware())
	podGroup.GET("/", api.getPod())
	podGroup.GET("/events", api.getPodEvents())
	podGroup.GET("/logs/:container", api.getPodLogs())
}

func getLocalConfig() (*k8sclient.K8sClient, error) {
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
