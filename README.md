# juicefs-csi-driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

## JuiceFS CSI Driver

The [JuiceFS](https://juicefs.com) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS filesystems.

### CSI Specification Compability Matrix

| JuiceFS CSI Driver \ CSI Version       | v0.3.0|
|----------------------------------------|-------|
| master branch                          | yes   |
| csi-v0.3.0                             | yes   |

## Features

Currently only static provisioning is supported. This means an JuiceFS filesystem needs to be created manually on [juicefs web console](https://juicefs.com/console/create) first. After that it can be mounted inside a container as a volume using the driver.

The following CSI interfaces are implemented:

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## JuiceFS CSI Driver on Kubernetes

The following sections are Kubernetes specific. If you are a Kubernetes user, use this for driver features, installation steps and examples.

### Kubernetes Version Compability Matrix

| JuiceFS CSI Driver \ Kubernetes Version| v1.11 |
|----------------------------------------|-------|
| master branch                          | yes   |
| csi-v0.3.0                             | yes   |

### Container Images

|JuiceFS CSI Driver Version | Image                                   |
|---------------------------|-----------------------------------------|
|master branch              |juicedata/juicefs-csi-driver:latest      |
|csi-v0.3.0                 |juicedata/juicefs-csi-driver:csi-v0.3.0  |

### Features

* Static provisioning - JuiceFS filesystem needs to be created manually first, then it could be mounted inside container as a persistent volume (PV) using the driver.
* Mount Options - Mount options can be specified in the persistence volume (PV) to define how the volume should be mounted.

**Notes**:

* Since JuiceFS is an elastic filesystem it doesn't really enforce any filesystem capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the filesystem. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value for the capacity.

### Installation

Deploy the driver:

```sh
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/install/manifest.yaml
```

### Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and how to [create JuiceFS filesystem](https://juicefs.com/docs/en/getting_started.html).
* When creating JuiceFS filesystem, make sure it is accessible from Kuberenetes cluster. It is recommended to create the filesystem inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](README.md#Installation) steps.

#### Example links

* [Static provisioning](examples/static-provisioning/README.md)

## Development

Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

### Requirements

* Golang 1.11.4+

### Dependency

Dependencies are managed through go module. To build the project, first turn on go mod using `export GO111MODULE=on`, to build the project run: `make`

### Build container image

```sh
make image-dev
make push-dev
```

### Testing

To execute all unit tests, run: `make test`

## License

This library is licensed under the Apache 2.0 License.
