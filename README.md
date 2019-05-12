# juicefs-csi-driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

## JuiceFS CSI Driver

The [JuiceFS](https://juicefs.com) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS filesystems.

### CSI Specification Compatibility Matrix

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

### Kubernetes Version Compatibility Matrix

| JuiceFS CSI Driver \ Kubernetes Version| v1.11 | v1.12 | v1.14 |
|----------------------------------------|-------|-------|-------|
| master branch                          | yes   | yes   | yes   |
| csi-v0.3.0                             | yes   | yes   | yes   |

### Container Images

|JuiceFS CSI Driver Version | Image                                   |
|---------------------------|-----------------------------------------|
|master branch              |juicedata/juicefs-csi-driver:latest      |
|csi-v0.3.0                 |juicedata/juicefs-csi-driver:csi-v0.3.0  |

### Features

* Static provisioning - JuiceFS filesystem needs to be created manually first, then it could be mounted inside container as a persistent volume (PV) using the driver.
* (WIP) Mount Options - Mount options can be specified in the persistence volume (PV) to define how the volume should be mounted.

|Feature \ JuiceFS CSI Driver | master | csi-v0.3.0 |
|-----------------------------|--------|------------|
| static provision            | yes    | yes        |
| mount options               | no     | no         |

### Validation

JuiceFS CSI driver has been validated in the following Kubernetes version

| Kubernetes \ JuiceFS CSI Driver   | master | csi-v0.3.0 |
|-----------------------------------|--------|------------|
| v1.11.9 / kops 1.11.1             | yes    | yes        |
| v1.12.6-eks-d69f1b / AWS EKS      | yes    | yes        |
| v1.14.1 / kops (git-39884d0b5)    | yes    | yes        |

**Notes**:

* Since JuiceFS is an elastic filesystem it doesn't really enforce any filesystem capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the filesystem. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.

### Installation

Deploy the driver:

```s
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```

### Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and how to [create JuiceFS filesystem](https://juicefs.com/docs/en/getting_started.html).
* When creating JuiceFS filesystem, make sure it is accessible from Kuberenetes cluster. It is recommended to create the filesystem inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](README.md#Installation) steps.

#### Example links

* [Static provisioning](examples/static-provisioning/)
* [Accessing the filesystem from multiple pods](examples/multiple-pods-read-write-many/)

## Development

Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

### Requirements

* Golang 1.11.4+

### Dependency

Dependencies are managed through go module. To build the project, first turn on go mod using `export GO111MODULE=on`, to build the project run: `make`

### Build container image

```s
make image-dev
make push-dev
```

### Testing

To execute all unit tests, run: `make test`

### Troubleshooting

If the application pod is hanging in `ContainerCreating` status for a long time, e.g.

```s
$ kubectl get pods
NAME            READY     STATUS              RESTARTS   AGE
juicefs-app-1   0/1       ContainerCreating   0          10m
juicefs-app-2   0/1       ContainerCreating   0          10m
```

Describe it to see the events, e.g.

```s
$ kubectl describe pod juicefs-app-1
Name:               juicefs-app-1
Namespace:          juicefs-csi-demo
...
Events:
  Type     Reason              Age                From                                              Message
  ----     ------              ----               ----                                              -------
  Normal   Scheduled           12m                default-scheduler                                 Successfully assigned juicefs-csi-demo/juicefs-app-1 to ip-10-0-0-31.us-west-2.compute.internal
  Warning  FailedMount         1m (x5 over 10m)   kubelet, ip-10-0-0-31.us-west-2.compute.internal  Unable to mount volumes for pod "juicefs-app-1_juicefs-csi-demo(45654a9b-6fee-11e9-aee6-06b5b6616e3c)": timeout expired waiting for volumes to attach or mount for pod "juicefs-csi-demo"/"juicefs-app-1". list of unmounted volumes=[persistent-storage]. list of unattached volumes=[persistent-storage default-token-xjj8k]
  Warning  FailedAttachVolume  1m (x12 over 12m)  attachdetach-controller                           AttachVolume.Attach failed for volume "juicefs-csi-demo" : attachment timeout for volume csi-demo
```

Check the logs of the following components

* `kube-controller-manager`
* `kubelet`
* `juicefs-csi-node`
* `juicefs-csi-attacher`

`juicefs-csi-driver` **MUST** be deployed to `kube-system` namespace

#### kubelet

```s
sudo journalctl -u kubelet -f
```

##### Orphaned pod

```s
May 12 09:58:03 ip-172-20-48-5 kubelet[1028]: E0512 09:58:03.411256    1028 kubelet_volumes.go:154] Orphaned pod "e7d422a7-7495-11e9-937d-0adc9bc4231a" found, but volume paths are still present on disk : There were a total of 1 errors similar to this. Turn up verbosity to see them.
```

Workaround

```s
$ sudo su
# cd /var/lib/kubelet/pods
# rm -rf e7d422a7-7495-11e9-937d-0adc9bc4231a/volumes/kubernetes.io~csi/
```

## License

This library is licensed under the Apache 2.0 License.
