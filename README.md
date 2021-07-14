# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

The [JuiceFS](https://github.com/juicedata/juicefs) Container Storage Interface (CSI) Driver implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS file system.

## Prerequisites

- Kubernetes 1.14+

## Installation

### Installation with Helm

#### Prerequisites

- Helm 3.1.0+

#### Install Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

To install Helm, refer to the [Helm install guide](https://github.com/helm/helm#install) and ensure that the `helm` binary is in the `PATH` of your shell.

#### Using Helm To Deploy

1. Prepare a `values.yaml` file with access information about Redis and object storage. If already formatted a volume, only `name` and `metaurl` is required. You can specify cpu/memory limits and requests of mount pod for pods using this driver. 

```yaml
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  backend:
    name: "<name>"
    metaurl: "<redis-url>"
    storage: "<storage-type>"
    accessKey: "<access-key>"
    secretKey: "<secret-key>"
    bucket: "<bucket>"
  mountPod:
    resources:
      limits:
        cpu: "<cpu_limit>"
        memory: "<memory_limit>"
      requests:
        cpu: "<cpu_request>"
        memory: "<memory_request>"
```

2. Install

```sh
helm repo add juicefs-csi-driver https://juicedata.github.io/juicefs-csi-driver/
helm repo update
helm upgrade juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver --install -f ./values.yaml
```

3. Check the deployment

- Check pods are running: the deployment will launch a `StatefulSet` named `juicefs-csi-controller` with replica `1` and a `DaemonSet` named `juicefs-csi-node`, so run `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` should see `n+1` (where `n` is the number of worker nodes of the Kubernetes cluster) pods is running. For example:

```sh
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

- Check secret: `kubectl -n kube-system describe secret juicefs-sc-secret` will show the secret with above `backend` fields in `values.yaml`:

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

- Check storage class: `kubectl get sc juicefs-sc` will show the storage class like this:

```
NAME         PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
juicefs-sc   csi.juicefs.com   Retain          Immediate           false                  69m
```

### Install with kubectl

Deploy the driver:

```shell
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```

If the CSI driver couldn't be discovered by Kubernetes and the error like this: **driver name csi.juicefs.com not found in the list of registered CSI drivers**, check the root directory path of `kubelet`. Run the following command on any non-master node in your Kubernetes cluster:

```shell
ps -ef | grep kubelet | grep root-dir
```

If the result isn't empty, modify the CSI driver deployment `k8s.yaml` file with the new path and redeploy the CSI driver again.

```shell
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

**Replace** `{{KUBELET_DIR}}` with your own `--root-dir` value in above command.

## Upgrade CSI Driver

### After v0.10.0

Juicefs csi driver separated JuiceFS client from CSI Driver since v0.10.0, csi driver upgrade will not interrupt existing PVs. If csi driver version >= v0.10.0, 

* If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make sure `juicefs-csi-controller` and `juicefs-csi-node` pods are restarted.
* If you have pinned to a specific version, modify your `k8s.yaml` to a newer version, then run `kubectl apply -f k8s.yaml`. 
* Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.

If you want to upgrade csi driver from v0.9.0 to v0.10.0, follow [how to upgrade csi-driver from v0.9.0 to v0.10.0](./docs/upgrade-csi-driver.md).

### Before v0.9.0

Upgrade of CSI Driver requires restart the DaemonSet, which has all the JuiceFS client running inside. The restart will cause all PVs become unavailable, so we need to stop all the application pod first.

1. Stop all pods using this driver.
2. Upgrade driver:
	* If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make sure `juicefs-csi-controller` and `juicefs-csi-node` pods are restarted.
	* If you have pinned to a specific version, modify your `k8s.yaml` to a newer version, then run `kubectl apply -f k8s.yaml`.
    * Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.
3. Start all the application pods.

We are working on launching the JuiceFS client in separate container, so the upgrade will NOT interrupt existing PVs. Before that, we can upgrade the JuiceFS client without upgrading the CSI driver, please follow [this workaround](./docs/upgrade-juicefs.md).

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
* [Dynamic provisioning](examples/dynamic-provisioning/)

**Notes**:

* Since JuiceFS is an elastic file system it doesn't really enforce any file system capacity. The actual storage capacity value in persistence volume and persistence volume claim is not used when creating the file system. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.
* kustomize 3.x is required to build some examples.

## CSI Specification Compatibility

| JuiceFS CSI Driver \ CSI Version | v0.3 | v1.0 |
| -------------------------------- | ---- | ---- |
| master branch                    | no   | yes  |

### Interfaces

The following CSI interfaces are implemented:

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## Troubleshooting

If you encounter any issue, please refer to [Troubleshooting](docs/troubleshooting.md) document.

## JuiceFS CSI Driver on Kubernetes

The following sections are Kubernetes specific. If you are a Kubernetes user, use this for driver features, installation steps and examples.

### Kubernetes Version Compatibility

JuiceFS CSI Driver is compatible with Kubernetes **v1.14+**

Container Images

| JuiceFS CSI Driver Version | Image                               |
| -------------------------- | ----------------------------------- |
| master branch              | juicedata/juicefs-csi-driver:latest |

### Features

* **Static provisioning** - JuiceFS file system needs to be created manually first, then it could be mounted inside container as a persistent volume (PV) using the driver.
* **Mount options** - CSI volume attributes can be specified in the persistence volume (PV) to define how the volume should be mounted.
* **Read write many** - Support `ReadWriteMany` access mode
* **Sub path** - provision persist volume with subpath in JuiceFS file system
* **Mount resources** - CSI volume attributes can be specified in the persistence volume (PV) to define cpu/memory limits/requests of mount pod. (above v0.10.0)
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

## Miscellaneous

- [Access ceph cluster with librados](./docs/ceph.md)

## Develop

See [DEVELOP](./docs/DEVELOP.md) document.

## License

This library is licensed under the Apache 2.0 License.
