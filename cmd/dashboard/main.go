package main

import (
	goflag "flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/juicedata/juicefs-csi-driver/pkg/k8sclient"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
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
	if devMode {
		client, err = getLocalConfig()
	} else {
		gin.SetMode(gin.ReleaseMode)
		client, err = k8sclient.NewClient()
	}
	if err != nil {
		log.Fatalf("can't get k8s client: %v", err)
	}
	api := newApi(client)
	r := gin.Default()
	api.handle(r.Group("/api/v1"))
	r.Run(fmt.Sprintf(":%d", port))
}

type dashboardApi struct {
	k8sClient *k8sclient.K8sClient
}

func newApi(k8sClient *k8sclient.K8sClient) *dashboardApi {
	return &dashboardApi{
		k8sClient: k8sClient,
	}
}

func (api *dashboardApi) handle(group *gin.RouterGroup) {
	group.GET("/pods", api.listAppPod())
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
