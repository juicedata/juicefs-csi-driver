package client

import (
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type JuiceMountV1Interface interface {
	JuiceMounts(namespace string) JuiceMountInterface
}

type JuiceFsClient struct {
	restClient rest.Interface
}

func (j *JuiceFsClient) JuiceMounts(namespace string) JuiceMountInterface {
	return newJuiceMounts(j, namespace)
}

func NewForConfig() (*JuiceFsClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	config.ContentConfig.GroupVersion = &mountv1.GroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	client, err := rest.RESTClientFor(config)
	if err != nil {
		return nil, err
	}
	return &JuiceFsClient{client}, err
}
