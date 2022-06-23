---
sidebar_label: 设置缓存 PVC
---

# 如何在 Kubernetes 中设置自定义 PVC 作为缓存

:::note 注意
此特性需使用 0.15.1 及以上版本的 JuiceFS CSI 驱动
:::

JuiceFS 的客户端运行在 pod 中，称为 mount pod。我们可以为 JuiceFS 的客户端配置专门的缓存路径，比如使用 EBS 作为客户端的缓存。
本文档讲述如何配置 PVC 作为 mount pod 的缓存路径。

## 静态配置

首先需要准备一个给 mount pod 用的 PVC，需要与 mount pod 在同一个 namespace 下， 即 csi driver 的组件所在 namespace（默认为 kube-system）。

您可以在 PV 中配置给 mount pod 用的 PVC，在 `volumeAttributes` 中设置 `juicefs/mount-cache-pvc`，值为 PVC 名，假设 PVC 名为 `ebs-pvc`：

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

PVC 和示例 pod 可参考 [这篇文档](./static-provisioning.md)。

## 动态配置

您也可以在 StorageClass 中配置给 mount pod 用的 PVC，在 `parameters` 中设置 `juicefs/mount-cache-pvc`，值为 PVC 名，假设 PVC 名为 `ebs-pvc`：

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

PVC 和示例 pod 可参考 [这篇文档](./dynamic-provisioning.md)。
