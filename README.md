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

* Since JuiceFS is an elastic filesystem it doesn't really enforce any filesystem capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the filesystem. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.

### Installation

Deploy the driver:

```s
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/install/manifest.yaml
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

If the application pod is hanging in `ContainerCreating` status for a long time

```s
$ kubectl get pods
NAME            READY     STATUS              RESTARTS   AGE
juicefs-app-1   0/1       ContainerCreating   0          10m
juicefs-app-2   0/1       ContainerCreating   0          10m
```

Describe it to see the events

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

Grep `Error` and `Warnings` and inspect the content

#### kube-controller-manager

```s
E0506 12:05:14.362447 1 stateful_set.go:400] Error syncing StatefulSet juicefs-csi-demo/juicefs-csi-controller, requeuing: pods "juicefs-csi-controller-0" is forbidden: pods with system-cluster-critical priorityClass is not permitted in juicefs-csi-demo namespace
```

`juicefs-csi-driver` **MUST** be deployed to `kube-system` namespace

```s
E0506 12:11:55.012059 1 csi_attacher.go:226] kubernetes.io/csi: attacher.WaitForAttach timeout after 15s [volume=csi-demo; attachment.ID=csi-d1cff3ccc0e51d613a4881426f44d802c155cf2b9979c028f50df5004478fe16]
E0506 12:11:55.012219 1 nestedpendingoperations.go:267] Operation for "\"kubernetes.io/csi/csi.juicefs.com^csi-demo\"" failed. No retries permitted until 2019-05-06 12:13:57.012184787 +0000 UTC m=+85219.996841359 (durationBeforeRetry 2m2s). Error: "AttachVolume.Attach failed for volume \"juicefs-csi-demo\" (UniqueName: \"kubernetes.io/csi/csi.juicefs.com^csi-demo\") from node \"ip-10-0-0-31.us-west-2.compute.internal\" : attachment timeout for volume csi-demo"
```

## License

This library is licensed under the Apache 2.0 License.
