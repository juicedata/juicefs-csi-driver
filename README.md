# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

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

```shell
ps -ef | grep kubelet | grep root-dir
```

If the result isn't empty, update kubeletDir in `values.yaml`:

```yaml
kubeletDir: <kubelet-dir>
```

3. Deploy

```sh
helm repo add juicefs-csi-driver https://juicedata.github.io/juicefs-csi-driver/
helm repo update
helm upgrade juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver --install -f ./values.yaml
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

1. Check the root directory path of `kubelet`. Run the following command on any non-master node in your Kubernetes cluster:

```shell
ps -ef | grep kubelet | grep root-dir
```

2. Deploy

If the result of cmd above isn't empty, modify the CSI driver deployment `k8s.yaml` file with the new path and deploy:

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> **Note**: please replace `{{KUBELET_DIR}}` in the above command with the actual root directory path of kubelet.

If the result of cmd above is empty, deploy directly:

```shell
# Kubernetes version >= v1.18
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

# Kubernetes version < v1.18
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
```

## Troubleshooting & FAQs

If you encounter any issue, please refer to [Troubleshooting](docs/troubleshooting.md) or [FAQs](docs/FAQs.md) document. 

## Upgrade CSI Driver

### CSI Driver version >= v0.10

Juicefs CSI Driver separated JuiceFS client from CSI Driver since v0.10.0, CSI Driver upgrade will not interrupt existing PVs. If CSI Driver version >= v0.10.0, do operations below:

* If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make sure `juicefs-csi-controller` and `juicefs-csi-node` pods are restarted.
* If you have pinned to a specific version, modify your `k8s.yaml` to a newer version, then run `kubectl apply -f k8s.yaml`.
* Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.

### CSI Driver version < v0.10

#### Minor version upgrade

Upgrade of CSI Driver requires restart the DaemonSet, which has all the JuiceFS client running inside. The restart will cause all PVs become unavailable, so we need to stop all the application pod first.

1. Stop all pods using this driver.
2. Upgrade driver:
	* If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make sure `juicefs-csi-controller` and `juicefs-csi-node` pods are restarted.
	* If you have pinned to a specific version, modify your `k8s.yaml` to a newer version, then run `kubectl apply -f k8s.yaml`.
    * Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.
3. Start all the application pods.

#### Cross-version upgrade

If you want to upgrade CSI Driver from v0.9.0 to v0.10.0+, follow ["How to upgrade CSI Driver from v0.9.0 to v0.10.0+"](./docs/upgrade-csi-driver.md).

#### Other

For users of the old version, you can also upgrade the JuiceFS client without upgrading the CSI driver. For details, refer to [this document](./docs/upgrade-juicefs.md).

Visit [Docker Hub](https://hub.docker.com/r/juicedata/juicefs-csi-driver) for more versions.

## Examples

Before the example, you need to:

* Get yourself familiar with how to setup Kubernetes and [how to use JuiceFS file system](https://github.com/juicedata/juicefs).
* Make sure JuiceFS is accessible from Kuberenetes cluster. It is recommended to create the file system inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](#installation) steps.

### Example links

* [Basic](examples/basic)
* [Static provisioning](examples/static-provisioning/)
  * [Mount options](examples/static-provisioning-mount-options/)
  * [Read write many](examples/static-provisioning-rwx/)
  * [Sub path](examples/static-provisioning-subpath/)
  * [Mount resources](examples/static-provisioning-mount-resources/)
  * [Config and env](examples/static-provisioning-config-and-env/)
* [Dynamic provisioning](examples/dynamic-provisioning/)

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

### Validation

JuiceFS CSI driver has been validated in the following Kubernetes version

| Kubernetes                 | master |
| -------------------------- | ------ |
| v1.19.2 / minikube v1.16.0 | Yes    |
| v1.20.2 / minikube v1.16.0 | Yes    |

### Known issues

The mount option `--cache-dir` in JuiceFS CSI driver (>=v0.10.0) does not support wildcards currently.

## Miscellaneous

- [Access Ceph cluster with librados](./docs/ceph.md)

## Develop

See [DEVELOP](./docs/DEVELOP.md) document.

## License

This library is licensed under the Apache 2.0 License.
