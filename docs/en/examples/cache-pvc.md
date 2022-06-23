---
sidebar_label: Set Cache PVC
---

# How to set PVC as cache in Kubernetes

:::note
This feature requires JuiceFS CSI Driver version 0.15.1 and above.
:::

JuiceFS client runs in pod called mount pods. We can configure a dedicated cache path for JuiceFS client, such as using
EBS as cache.
This document describes how to configure PVCs as cache paths for mount pods.

## Static provisioning

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

For PVC (PersistentVolumeClaim) and sample pod, Refer to [this document](./static-provisioning.md) for more details.

## Dynamic provisioning

You can also configure a PVC for mount pods in StorageClass, set `juicefs/mount-cache-pvc` in `parameters`, the value is
the PVC name, assuming the PVC name is `ebs-pvc`:

```yaml {12}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
  namespace: default
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  juicefs/mount-cache-pvc: "ebs-pvc"
```

For PVC (PersistentVolumeClaim) and sample pod, Refer to [this document](./dynamic-provisioning.md) for more details.
