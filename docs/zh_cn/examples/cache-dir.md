---
sidebar_label: 设置缓存路径
---

# 如何在 Kubernetes 中设置缓存路径

JuiceFS CSI 驱动支持将本地磁盘或者云盘挂载到 mount pod，本文档介绍如何在 Kubernetes 中设置 JuiceFS 的缓存路径。

## 静态配置

### 使用本地磁盘作为缓存路径

默认情况下，缓存路径为 `/var/jfsCache`，CSI 驱动会将该路径挂载到 mount pod 中。您也可以在 PV 的 `spec.mountOptions` 中设置缓存路径：

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

PVC 和示例 pod 可参考 [这篇文档](./static-provisioning.md)。

#### 检查缓存路径

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您还可以验证 JuiceFS 客户端是否设置了预期的缓存路径，参考 [这篇文档](../troubleshooting.md#找到-mount-pod) 找到对应的 mount pod：

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-juicefs-pv -oyaml | grep mount.juicefs
```

### 使用 PVC 作为缓存路径

:::note 注意
此特性需使用 0.15.1 及以上版本的 JuiceFS CSI 驱动
:::

我们也可以为 JuiceFS 的客户端配置专门的云盘作为缓存路径，比如使用 EBS 作为客户端的缓存。

首先需要准备一个给 mount pod 用的 PVC，需要与 mount pod 在同一个 namespace 下， 即 csi 驱动的组件所在 namespace（默认为 kube-system）。
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

## 动态配置

### 使用本地磁盘作为缓存路径

默认情况下，缓存路径为 `/var/jfsCache`，CSI 驱动会将该路径挂载到 mount pod 中。您也可以在 StorageClass 的 `mountOptions` 中配置缓存路径：

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

PVC 和示例 pod 可参考 [这篇文档](./dynamic-provisioning.md)。

#### 检查缓存路径

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您还可以验证 JuiceFS 客户端是否设置了预期的缓存路径，参考 [这篇文档](../troubleshooting.md#找到-mount-pod) 找到对应的 mount pod：

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-pvc-5916988b-71a0-4494-8315-877d2dbb8709 -oyaml | grep mount.juicefs
```

### 使用独立 PVC 作为缓存路径

您也可以在 StorageClass 中配置给 mount pod 用的 PVC，在 `parameters` 中设置 `juicefs/mount-cache-pvc`，值为 PVC 名，假设 PVC 名为 `ebs-pvc`：

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
