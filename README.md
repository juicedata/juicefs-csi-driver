# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

The [JuiceFS](https://github.com/juicedata/juicefs) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS filesystems.

## Installation

Deploy the driver:

```shell
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```

## Installation with Helm

## Prerequisites
- Kubernetes 1.12+
- Helm 3.1.0

### Install Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

To install Helm, refer to the [Helm install guide](https://github.com/helm/helm#install) and ensure that the `helm` binary is in the `PATH` of your shell.


### Using Helm To Deploy

1. edit your self backend by copy `values.yaml`
```bash
cp deploy/chart/values.yaml ./self-values.yaml
```

2. edit backend part

3. deploy
```shell
helm install -f self-values.yaml juicefs ./deploy/chart -n SELF_NAMESPACE
```

### Upgrade CSI Driver

1. Stop all pods using this driver.
2. Upgrade driver:
	* If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make sure juicefs-csi-controller and juicefs-csi-node pods are restarted.
	* If you have pinned to a specific version, modify your k8s.yaml to a newer version, then run `kubectl apply -f k8s.yaml`.

Visit [here](https://hub.docker.com/r/juicedata/juicefs-csi-driver) for more versions.

## Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and how to [use JuiceFS filesystem](https://github.com/juicedata/juicefs).
* Make sure JuiceFS is accessible from Kuberenetes cluster. It is recommended to create the filesystem inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](README.md#Installation) steps.

### Example links

* [Basic](examples/basic)
* [Static provisioning](examples/static-provisioning/)
  * [Mount options](examples/static-provisioning-mount-options/)
  * [Read write many](examples/static-provisioning-rwx/)
  * [Sub path](examples/static-provisioning-subpath/)
* [Dynamic provisioning](examples/dynamic-provisioning/)

**Notes**:

* Since JuiceFS is an elastic filesystem it doesn't really enforce any filesystem capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the filesystem. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.
* kustomize 3.x is required to build some examples.

## CSI Specification Compatibility

| JuiceFS CSI Driver \ CSI Version | v0.3 | v1.0 |
| -------------------------------- | ---- | ---- |
| master branch                    | no   | yes  |

### Interfaces

The following CSI interfaces are implemented:

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## JuiceFS CSI Driver on Kubernetes

The following sections are Kubernetes specific. If you are a Kubernetes user, use this for driver features, installation steps and examples.

### Kubernetes Version Compatibility

JuiceFS CSI Driver is compatible with Kubernetes **v1.14+**

Container Images

| JuiceFS CSI Driver Version | Image                               |
| -------------------------- | ----------------------------------- |
| master branch              | juicedata/juicefs-csi-driver:latest |

### Features

* **Static provisioning** - JuiceFS filesystem needs to be created manually first, then it could be mounted inside container as a persistent volume (PV) using the driver.
* **Mount options** - CSI volume attributes can be specified in the persistence volume (PV) to define how the volume should be mounted.
* **Read write many** - Support `ReadWriteMany` access mode
* **Sub path** - provision persisten volume with subpath in JuiceFS filesystem
* **Dynamic provisioning** - allows storage volumes to be created on-demand

### Validation

JuiceFS CSI driver has been validated in the following Kubernetes version

| Kubernetes                 | master |
| -------------------------- | ------ |
| v1.19.2 / minikube v1.16.0 | Yes    |
| v1.20.2 / minikube v1.16.0 | Yes    |

### Known issues

#### JuiceFS CSI volumes can NOT reconcile [#14](https://github.com/juicedata/juicefs-csi-driver/issues/14)

When `juicefs-csi-node` is killed, existing JuiceFS volume will become inaccessible. It will not recover automatically even after `juicefs-csi-node` reconcile.

Delete the pods mounting JuiceFS volume and recreate them to recover.

## Develop

See [DEVELOP](./docs/DEVELOP.md) document.

## License

This library is licensed under the Apache 2.0 License.
