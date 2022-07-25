---
sidebar_label: Set Cache Directory
---

# How to set cache directory in Kubernetes

The JuiceFS CSI driver supports mounting local disks or cloud disks to the mount pod. This document describes how to set the cache path of JuiceFS in Kubernetes.

## Static provisioning

### Use local disk as cache path

By default, the cache path is `/var/jfsCache`, which CSI Driver will mount into the mount pod. You can set cache directory in `spec.mountOptions` of PV (Persistent Volume):

```yaml {15}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  mountOptions:
    - cache-dir=/dev/vdb1
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

For PVC (PersistentVolumeClaim) and sample pod, Refer to [this document](./static-provisioning.md) for more details.

#### Check cache directory

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

You can also verify that the JuiceFS client has the expected cache path set. Refer
to [this document](../troubleshooting.md#find-mount-pod) to find mount pod and run this command as follows:

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-juicefs-pv -oyaml | grep mount.juicefs
```

### Use PVC as cache path

:::note
This feature requires JuiceFS CSI Driver version 0.15.1 and above.
:::

We can also configure a dedicated cloud disk as a cache path for JuiceFS clients, such as using EBS as a client cache.

First, you need to prepare a PVC for mount pod, which needs to be in the same namespace as the mount pod, that is,
the namespace where the components of the csi driver are located (the default is kube-system).

You can configure the PVC for the mount pod in the PV, set `juicefs/mount-cache-pvc` in `volumeAttributes`, the value is
the PVC name, assuming the PVC name is `ebs-pvc`:

```yaml {22}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/mount-cache-pvc: "ebs-pvc"
```

## Dynamic provisioning

### Use local disk as cache path

By default, the cache path is `/var/jfsCache`, which CSI Driver will mount into the mount pod. You can set cache directory in `mountOptions` of StorageClass:

```yaml {12}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
mountOptions:
  - cache-dir=/dev/vdb1
```

For PVC (PersistentVolumeClaim) and sample pod, Refer to [this document](./dynamic-provisioning.md) for more details.

#### Check cache directory

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

You can also verify that the JuiceFS client has the expected cache path set. Refer
to [this document](../troubleshooting.md#find-mount-pod) to find mount pod and run this command as follows:

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-pvc-5916988b-71a0-4494-8315-877d2dbb8709 -oyaml | grep mount.juicefs
```

### Use PVC as cache path

You can also configure a PVC for mount pods in StorageClass, set `juicefs/mount-cache-pvc` in `parameters`, the value is
the PVC name, assuming the PVC name is `ebs-pvc`:

```yaml {11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  juicefs/mount-cache-pvc: "ebs-pvc"
```
