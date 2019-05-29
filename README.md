# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

The [JuiceFS](https://juicefs.com) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS filesystems.

## Installation

Deploy the driver:

```s
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```

Additional steps could be required on some provider, e.g. Aliyun Container Service Kubernetes. See [Troubleshooting#AttachVolume.Attach failed](DEVELOP.md#attachvolumeattach-failed) for details.

## Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and how to [create JuiceFS filesystem](https://juicefs.com/docs/en/getting_started.html).
* When creating JuiceFS filesystem, make sure it is accessible from Kuberenetes cluster. It is recommended to create the filesystem inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](README.md#Installation) steps.

### Example links

* [Basic](examples/basic)
* [Static provisioning](examples/static-provisioning/)
* [Mount options](examples/mount-options/)
* [Accessing the filesystem from multiple pods](examples/multiple-pods-read-write-many/)

**Notes**:

* Since JuiceFS is an elastic filesystem it doesn't really enforce any filesystem capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the filesystem. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.

## CSI Specification Compatibility

| JuiceFS CSI Driver \ CSI Version       | v0.3.0|
|----------------------------------------|-------|
| master branch                          | yes   |

### Interfaces

Currently only static provisioning is supported. This means an JuiceFS filesystem needs to be created manually on [juicefs web console](https://juicefs.com/console/create) first. After that it can be mounted inside a container as a volume using the driver.

The following CSI interfaces are implemented:

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## JuiceFS CSI Driver on Kubernetes

The following sections are Kubernetes specific. If you are a Kubernetes user, use this for driver features, installation steps and examples.

### Kubernetes Version Compatibility Matrix

| JuiceFS CSI Driver \ Kubernetes Version| v1.11 | v1.12 | v1.13 | v1.14 |
|----------------------------------------|-------|-------|-------|-------|
| master branch                          | yes   | yes   | yes   | yes   |

### Container Images

|JuiceFS CSI Driver Version | Image                                   |
|---------------------------|-----------------------------------------|
|master branch              |juicedata/juicefs-csi-driver:latest      |

### Features

* Static provisioning - JuiceFS filesystem needs to be created manually first, then it could be mounted inside container as a persistent volume (PV) using the driver.
* Mount Options - CSI volume attributes can be specified in the persistence volume (PV) to define how the volume should be mounted.

|Feature \ JuiceFS CSI Driver | master |
|-----------------------------|--------|
| static provision            | yes    |
| mount options               | yes    |

### Validation

JuiceFS CSI driver has been validated in the following Kubernetes version

| Kubernetes \ JuiceFS CSI Driver   | master |
|-----------------------------------|--------|
| v1.11.9 / kops 1.11.1             | yes    |
| v1.12.6-eks-d69f1b / AWS EKS      | yes    |
| v1.12.6-aliyun.1 / Aliyun CS K8s  | yes    |
| v1.13.5 / kops 1.13.0-alpha.1     | yes    |
| v1.14.1 / kops (git-39884d0b5)    | yes    |

Manual configuration is required for Aliyun Container Service Kubernetes. See [Troubleshooting#AttachVolume.Attach failed](DEVELOP.md#attachvolumeattach-failed) for details.

## Develop

See [DEVELOP](./DEVELOP.md) document.

## License

This library is licensed under the Apache 2.0 License.
