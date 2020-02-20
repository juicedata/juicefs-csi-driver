# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

The [JuiceFS](https://juicefs.com) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS filesystems.

## Installation

Deploy the driver:

```s
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```

Additional steps could be required on some provider, e.g. Aliyun Container Service Kubernetes. See [Troubleshooting#AttachVolume.Attach failed](docs/DEVELOP.md#attachvolumeattach-failed) for details.

## Upgrade

We have two components to upgrade:

* CSI Driver
* JuiceFS client in CSI Driver

### Upgrade CSI Driver

1. Stop all pods using this driver.
2. Upgrade driver:
	* If you're using `latest` tag, simple run `kubectl apply -f` like [installation](#installation).
	* If you have pinned to a specific version, modify your k8s.yaml to a newer version, then run `kubectl apply -f k8s.yaml`.

### Upgrade JuiceFS client

Refer to the notes in [Examples](#examples) section below.

## Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and how to [create JuiceFS filesystem](https://juicefs.com/docs/en/getting_started.html).
* When creating JuiceFS filesystem, make sure it is accessible from Kuberenetes cluster. It is recommended to create the filesystem inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](README.md#Installation) steps.

### Example links

* [Basic](examples/basic)
* [Static provisioning](examples/static-provisioning/)
  * [Mount options](examples/static-provisioning-mount-options/)
  * [Read write nany](examples/static-provisioning-rwx/)
  * [Sub path](examples/static-provisioning-subpath/)
* [Dynamic provisioning](examples/dynamic-provisioning/)

**Notes**:

* Since JuiceFS is an elastic filesystem it doesn't really enforce any filesystem capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the filesystem. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.
* JuiceFS CSI Driver now supports automatically upgrade of JuiceFS client. You can use latest docker image to always enable auto-upgrade, or you can still pin to a specific version to disable auto-upgrade. Visit [here](https://hub.docker.com/r/juicedata/juicefs-csi-driver) for more versions. We support two environment variables to configure auto-upgrade:
	* `JFS_AUTO_UPGRADE`: auto-upgrade enabled if set, otherwise disabled
	* `JFS_AUTP_UPGRADE_TIMEOUT`: time in seconds to do auto-upgrade (default 10)

	You can also configure these in your own way.  
	JuiceFS client will upgrade itself everytime before mounting. You can achieve this by simply re-deploying your pods.

## CSI Specification Compatibility

| JuiceFS CSI Driver \ CSI Version       | v0.3 | v1.0 |
|----------------------------------------|------|------|
| master branch (csi-v1)                 | no   | yes  |
| csi-v0 branch                          | yes  | no   |

### Interfaces

Currently only static provisioning is supported. This means an JuiceFS filesystem needs to be created manually on [juicefs web console](https://juicefs.com/console/create) first. After that it can be mounted inside a container as a volume using the driver.

The following CSI interfaces are implemented:

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## JuiceFS CSI Driver on Kubernetes

The following sections are Kubernetes specific. If you are a Kubernetes user, use this for driver features, installation steps and examples.

### Kubernetes Version Compatibility Matrix

| JuiceFS CSI Driver \ Kubernetes Version| v1.11 | v1.12 | v1.13 | v1.14 | v1.15 | v1.16 |
|----------------------------------------|-------|-------|-------|-------|-------|-------|
| master branch (csi-v1)                 | no    | no    | yes   | yes   | yes   | yes   |
| csi-v0 branch                          | yes   | yes   | yes   | yes   |       |       |

### Container Images

|JuiceFS CSI Driver Version | Image                                   |
|---------------------------|-----------------------------------------|
| master branch             |juicedata/juicefs-csi-driver:latest      |
| csi-v1 branch             |juicedata/juicefs-csi-driver:csi-v1      |
| csi-v0 branch             |juicedata/juicefs-csi-driver:csi-v0      |

### Features

* **Static provisioning** - JuiceFS filesystem needs to be created manually first, then it could be mounted inside container as a persistent volume (PV) using the driver.
* **Mount options** - CSI volume attributes can be specified in the persistence volume (PV) to define how the volume should be mounted.
* **Read write many** - Support `ReadWriteMany` access mode
* **Sub path** - provision persisten volume with subpath in JuiceFS filesystem
* **Dynamic provisioning** - allows storage volumes to be created on-demand

|Feature \ JuiceFS CSI Driver | master (csi-v1) | csi-v0 |
|-----------------------------|-----------------|--------|
| static provisioning         |       yes       | yes    |
|   mount options             |       yes       | yes    |
|   read write many           |       yes       | yes    |
|   sub path                  |       yes       | yes    |
| dynamic provisioning        |       yes       | no     |

### Validation

JuiceFS CSI driver has been validated in the following Kubernetes version

| Kubernetes \ JuiceFS CSI Driver   | master (csi-v1) | csi-v0 |
|-----------------------------------|-----------------|--------|
| v1.11.9 / kops 1.11.1             |                 | yes    |
| v1.12.6-eks-d69f1b / AWS EKS      |                 | yes    |
| v1.12.6-aliyun.1 / Aliyun CS K8s  |                 | yes    |
| v1.13.0 / minikube 1.4.0          |       yes       |        |
| v1.13.5 / kops 1.13.0-alpha.1     |                 | yes    |
| v1.14.1 / kops (git-39884d0b5)    |       yes       | yes    |
| v1.14.8 / minikube 1.4.0          |       yes       |        |
| v1.15.0 / minikube 1.4.0          |       yes       |        |
| v1.16.0 / minikube 1.4.0          |       yes       |        |
| v1.17.2 / minikube 1.7.2          |       yes       |        |

Manual configuration is required for Aliyun Container Service Kubernetes. See [Troubleshooting#AttachVolume.Attach failed](docs/DEVELOP.md#attachvolumeattach-failed) for details.

### Known issues

#### JuiceFS CSI volumes can NOT reconcile [#14](https://github.com/juicedata/juicefs-csi-driver/issues/14)

When `juicefs-csi-node` is killed, existing JuiceFS volume will become inaccessible. It will not recover automatically even after `juicefs-csi-node` reconcile.

Delete the pods mounting JuiceFS volume and recreate them to recover.

## Develop

See [DEVELOP](./docs/DEVELOP.md) document.

## License

This library is licensed under the Apache 2.0 License.
