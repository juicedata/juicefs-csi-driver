package main

import (
	goflag "flag"
	"net/http"
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

	cmd.PersistentFlags().Uint16Var(&port, "port", 9090, "port to listen on")
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
		client, err = k8sclient.NewClient()
	}
	if err != nil {
		klog.V(5).Infof("Can't get k8s client: %v", err)
	}
	_ = newApi(client)
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.Run()
}

type dashboardApi struct {
	k8sClient *k8sclient.K8sClient
}

func newApi(k8sClient *k8sclient.K8sClient) *dashboardApi {
	return &dashboardApi{
		k8sClient: k8sClient,
	}
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
