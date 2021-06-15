package client

import (
	"context"
	mountv1 "github.com/juicedata/juicefs-csi-driver/pkg/apis/juicefs.com/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type JuiceMountInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*mountv1.JuiceMountList, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*mountv1.JuiceMount, error)
	Create(ctx context.Context, mount *mountv1.JuiceMount, opts metav1.CreateOptions) (*mountv1.JuiceMount, error)
}

type JuiceMounts struct {
	restClient rest.Interface
	namespace  string
}

func newJuiceMounts(c *JuiceFsClient, namespace string) *JuiceMounts {
	return &JuiceMounts{
		restClient: c.restClient,
		namespace:  namespace,
	}
}
func (j JuiceMounts) List(ctx context.Context, opts metav1.ListOptions) (*mountv1.JuiceMountList, error) {
	result := &mountv1.JuiceMountList{}
	err := j.restClient.
		Get().
		Namespace(j.namespace).
		Resource("juicemounts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (j JuiceMounts) Get(ctx context.Context, name string, options metav1.GetOptions) (*mountv1.JuiceMount, error) {
	result := &mountv1.JuiceMount{}
	err := j.restClient.Get().
		Namespace(j.namespace).
		Resource("juicemounts").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (j JuiceMounts) Create(ctx context.Context, mount *mountv1.JuiceMount, opts metav1.CreateOptions) (*mountv1.JuiceMount, error) {
	result := &mountv1.JuiceMount{}
	err := j.restClient.Post().
		Namespace(j.namespace).
		Resource("juicemounts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(mount).
		Do(ctx).
		Into(result)
	return result, err
}
