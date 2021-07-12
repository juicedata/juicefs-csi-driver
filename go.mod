module github.com/juicedata/juicefs-csi-driver

require (
	github.com/container-storage-interface/spec v1.1.0
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/kubernetes-csi/csi-test v1.1.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	google.golang.org/grpc v1.27.1
	google.golang.org/protobuf v1.26.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog v0.1.0
	k8s.io/kubernetes v1.13.1
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.9.2
)

go 1.14
