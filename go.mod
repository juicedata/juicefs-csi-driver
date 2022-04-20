module github.com/juicedata/juicefs-csi-driver

require (
	github.com/agiledragon/gomonkey v2.0.2+incompatible
	github.com/container-storage-interface/spec v1.1.0
	github.com/golang/mock v1.6.0
	github.com/kubernetes-csi/csi-test v1.1.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/smartystreets/goconvey v1.6.4
	golang.org/x/sys v0.0.0-20211205182925-97ca703d548d // indirect
	golang.org/x/tools v0.1.8 // indirect
	google.golang.org/grpc v1.27.1
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog v1.0.0
	k8s.io/kubernetes v1.13.1
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/sig-storage-lib-external-provisioner/v6 v6.3.0
)

go 1.14
