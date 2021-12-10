# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

English | [简体中文](./README_CN.md)

The [JuiceFS](https://github.com/juicedata/juicefs) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS file system.

## Prerequisites

- Kubernetes 1.14+

## Installation

There are two ways to install JuiceFS CSI Driver.

### 1. Install via Helm

#### Prerequisites

- Helm 3.1.0+

#### Install Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

To install Helm, refer to the [Helm install guide](https://github.com/helm/helm#install) and ensure that the `helm` binary is in the `PATH` of your shell.

#### Using Helm To Deploy

1. Prepare a YAML file

Create a configuration file, for example: `values.yaml`, copy and complete the following configuration information. Among them, the `backend` part is the information related to the JuiceFS file system, you can refer to [JuiceFS Quick Start Guide](https://github.com/juicedata/juicefs/blob/main/docs/zh_cn/quick_start_guide.md) for more information. If you are using a JuiceFS volume that has been created, you only need to fill in the two items `name` and `metaurl`. The `mountPod` part can specify CPU/memory limits and requests of mount pod for pods using this driver. Unneeded items should be deleted, or its value should be left blank.

```yaml
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  backend:
    name: "<name>"
    metaurl: "<meta-url>"
    storage: "<storage-type>"
    accessKey: "<access-key>"
    secretKey: "<secret-key>"
    bucket: "<bucket>"
  mountPod:
    resources:
      limits:
        cpu: "<cpu-limit>"
        memory: "<memory-limit>"
      requests:
        cpu: "<cpu-request>"
        memory: "<memory-request>"
```

2. Check and update kubelet root-dir

Execute the following command.

```shell
$ ps -ef | grep kubelet | grep root-dir
```

If the result is not empty, it means that the `root-dir` path of kubelet is not the default value and you need to set `kubeletDir` to the current root-dir path of kubelet in the configuration file `values.yaml` prepared in the first step.

```yaml
kubeletDir: <kubelet-dir>
```

3. Deploy

```sh
helm repo add juicefs-csi-driver https://juicedata.github.io/juicefs-csi-driver/
helm repo update
helm install juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver -n kube-system -f ./values.yaml
```

4. Check the deployment

- **Check pods are running**: the deployment will launch a `StatefulSet` named `juicefs-csi-controller` with replica `1` and a `DaemonSet` named `juicefs-csi-node`, so run `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` should see `n+1` (where `n` is the number of worker nodes of the Kubernetes cluster) pods is running. For example:

```sh
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

- **Check secret**: `kubectl -n kube-system describe secret juicefs-sc-secret` will show the secret with above `backend` fields in `values.yaml`:

```
Name:         juicefs-sc-secret
Namespace:    kube-system
Labels:       app.kubernetes.io/instance=juicefs-csi-driver
              app.kubernetes.io/managed-by=Helm
              app.kubernetes.io/name=juicefs-csi-driver
              app.kubernetes.io/version=0.7.0
              helm.sh/chart=juicefs-csi-driver-0.1.0
Annotations:  meta.helm.sh/release-name: juicefs-csi-driver
              meta.helm.sh/release-namespace: default

Type:  Opaque

Data
====
access-key:  0 bytes
bucket:      47 bytes
metaurl:     54 bytes
name:        4 bytes
secret-key:  0 bytes
storage:     2 bytes
```

- **Check storage class**: `kubectl get sc juicefs-sc` will show the storage class like this:

```
NAME         PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
juicefs-sc   csi.juicefs.com   Retain          Immediate           false                  69m
```

### 2. Install via kubectl

Since Kubernetes will deprecate some old APIs when a new version is released, you need to choose the appropriate deployment configuration file.

1. Check the root directory path of `kubelet`.

Execute the following command on any non-Master node in the Kubernetes cluster.

```shell
$ ps -ef | grep kubelet | grep root-dir
```

2. Deploy

**If the check command returns a non-empty result**, it means that the `root-dir` path of the kubelet is not the default, so you need to update the `kubeletDir` path in the CSI Driver's deployment file and deploy.

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> **Note**: please replace `{{KUBELET_DIR}}` in the above command with the actual root directory path of kubelet.

**If the check command returns an empty result**, you can deploy directly without modifying the configuration:

```shell
# Kubernetes version >= v1.18
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

# Kubernetes version < v1.18
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
```

## Troubleshooting & FAQs

If you encounter any issue, please refer to [Troubleshooting](docs/en/troubleshooting.md) or [FAQs](docs/en/FAQs.md) document. 

## Upgrade CSI Driver

Refer to [Upgrade Csi Driver](docs/en/upgrade-csi-driver.md) document.

## Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and [how to use JuiceFS file system](https://github.com/juicedata/juicefs).
* Make sure JuiceFS is accessible from Kuberenetes cluster. It is recommended to create the file system inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](#installation) steps.

### Example links

* [Static provisioning](docs/en/examples/static-provisioning.md)
* [Dynamic provisioning](docs/en/examples/dynamic-provisioning.md)
* [Mount options](docs/en/examples/mount-options.md)
* [ReadWriteMany and ReadOnlyMany](docs/en/examples/rwx-and-rox.md)
* [Sub path](docs/en/examples/subpath.md)
* [Mount resources](docs/en/examples/mount-resources.md)
* [Config and env](docs/en/examples/config-and-env.md)

**Notes**:

* Since JuiceFS is an elastic file system it doesn't really enforce any file system capacity. The actual storage capacity value in PersistentVolume and PersistentVolumeClaim is not used when creating the file system. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.
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

* **Static provisioning** - JuiceFS file system needs to be created manually first, then it could be mounted inside container as a PersistentVolume (PV) using the driver.
* **Mount options** - CSI volume attributes can be specified in the PersistentVolume (PV) to define how the volume should be mounted.
* **Read write many** - Support `ReadWriteMany` access mode
* **Sub path** - provision PersistentVolume with subpath in JuiceFS file system
* **Mount resources** - CSI volume attributes can be specified in the PersistentVolume (PV) to define CPU/memory limits/requests of mount pod.
* **Config files & env in mount pod** - Support set config files and envs in mount pod.
* **Dynamic provisioning** - allows storage volumes to be created on-demand

### Known issues

The mount option `--cache-dir` in JuiceFS CSI driver (>=v0.10.0) does not support wildcards currently.

## Miscellaneous

- [Access Ceph cluster with librados](docs/en/ceph.md)

## License

This library is licensed under the Apache 2.0 License.
