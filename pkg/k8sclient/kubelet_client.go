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

package k8sclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/klog"
)

const (
	defaultKubeletTimeout   = 10
	serviceAccountTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

type KubeletClient struct {
	host   string
	port   int
	client *http.Client
}

// KubeletClientConfig defines config parameters for the kubelet client
type KubeletClientConfig struct {
	// Address specifies the kubelet address
	Address string

	// Port specifies the default port - used if no information about Kubelet port can be found in Node.NodeStatus.DaemonEndpoints.
	Port int

	// TLSClientConfig contains settings to enable transport layer security
	restclient.TLSClientConfig

	// Server requires Bearer authentication
	BearerToken string

	// HTTPTimeout is used by the client to timeout http requests to Kubelet.
	HTTPTimeout time.Duration
}

// makeTransport creates a RoundTripper for HTTP Transport.
func makeTransport(config *KubeletClientConfig, insecureSkipTLSVerify bool) (http.RoundTripper, error) {
	// do the insecureSkipTLSVerify on the pre-transport *before* we go get a potentially cached connection.
	// transportConfig always produces a new struct pointer.
	preTLSConfig := config.transportConfig()
	if insecureSkipTLSVerify && preTLSConfig != nil {
		preTLSConfig.TLS.Insecure = true
		preTLSConfig.TLS.CAData = nil
		preTLSConfig.TLS.CAFile = ""
	}

	tlsConfig, err := transport.TLSConfigFor(preTLSConfig)
	if err != nil {
		return nil, err
	}

	rt := http.DefaultTransport
	if tlsConfig != nil {
		// If SSH Tunnel is turned on
		rt = utilnet.SetOldTransportDefaults(&http.Transport{
			TLSClientConfig: tlsConfig,
		})
	}

	return transport.HTTPWrappersForConfig(config.transportConfig(), rt)
}

// transportConfig converts a client config to an appropriate transport config.
func (c *KubeletClientConfig) transportConfig() *transport.Config {
	cfg := &transport.Config{
		TLS: transport.TLSConfig{
			CAFile:   c.CAFile,
			CAData:   c.CAData,
			CertFile: c.CertFile,
			CertData: c.CertData,
			KeyFile:  c.KeyFile,
			KeyData:  c.KeyData,
		},
		BearerToken: c.BearerToken,
	}
	if !cfg.HasCA() {
		cfg.TLS.Insecure = true
	}
	return cfg
}

func NewKubeletClient(host string, port int) (*KubeletClient, error) {
	var token string
	var err error
	kubeletClientCert := os.Getenv("KUBELET_CLIENT_CERT")
	kubeletClientKey := os.Getenv("KUBELET_CLIENT_KEY")
	if kubeletClientCert == "" && kubeletClientKey == "" {
		// get CSI sa token
		tokenByte, err := os.ReadFile(serviceAccountTokenFile)
		if err != nil {
			return nil, fmt.Errorf("in cluster mode, find token failed: %v", err)
		}
		token = string(tokenByte)
	}

	kubeletTimeout := defaultKubeletTimeout
	if os.Getenv("KUBELET_TIMEOUT") != "" {
		if kubeletTimeout, err = strconv.Atoi(os.Getenv("KUBELET_TIMEOUT")); err != nil {
			return nil, fmt.Errorf("got error when parsing kubelet timeout: %v", err)
		}
	}
	config := &KubeletClientConfig{
		Address: host,
		Port:    port,
		TLSClientConfig: rest.TLSClientConfig{
			ServerName: "kubelet",
			Insecure:   true,
			CertFile:   kubeletClientCert,
			KeyFile:    kubeletClientKey,
		},
		BearerToken: token,
		HTTPTimeout: time.Duration(kubeletTimeout) * time.Second,
	}

	trans, err := makeTransport(config, config.Insecure)
	if err != nil {
		return nil, err
	}

	return &KubeletClient{
		host: config.Address,
		port: config.Port,
		client: &http.Client{
			Transport: trans,
			Timeout:   config.HTTPTimeout,
		},
	}, nil
}

func (kc *KubeletClient) GetNodeRunningPods() (*corev1.PodList, error) {
	resp, err := kc.client.Get(fmt.Sprintf("https://%v:%d/pods/", kc.host, kc.port))
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if string(body) == "Unauthorized" {
		return nil, fmt.Errorf("unauthorized")
	}
	podLists := &corev1.PodList{}
	if err = json.Unmarshal(body, &podLists); err != nil {
		klog.V(5).Infof("GetNodeRunningPods err: %s", body)
		return nil, err
	}
	return podLists, err
}
