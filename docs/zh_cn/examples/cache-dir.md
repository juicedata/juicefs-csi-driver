---
sidebar_label: 设置缓存路径
---

# 如何在 Kubernetes 中设置缓存路径

本文档展示了如何在 Kubernetes 中设置 JuiceFS 的缓存路径。CSI 驱动在部署 mount pod 时，
会将 Kubernetes 节点上的对应路径挂载到 mount pod 中，如果需要将节点上的磁盘路径设置为客户端的缓存路径，可遵循本文档。

## 静态配置

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
    volumeHandle: test-bucket
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

PVC 和示例 pod 可参考 [这篇文档](./static-provisioning.md)。

### 检查缓存路径

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您还可以验证 JuiceFS 客户端是否设置了预期的缓存路径，参考 [这篇文档](../troubleshooting.md#找到-mount-pod) 找到对应的 mount pod：

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-test-bucket -oyaml | grep mount.juicefs
```

## 动态配置

默认情况下，缓存路径为 `/var/jfsCache`，CSI 驱动会将该路径挂载到 mount pod 中。您也可以在 StorageClass 的 `mountOptions` 中配置缓存路径：

```yaml {13}
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
mountOptions:
  - cache-dir=/dev/vdb1
```

PVC 和示例 pod 可参考 [这篇文档](./dynamic-provisioning.md)。

### 检查缓存路径

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您还可以验证 JuiceFS 客户端是否设置了预期的缓存路径，参考 [这篇文档](../troubleshooting.md#找到-mount-pod) 找到对应的 mount pod：

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-pvc-5916988b-71a0-4494-8315-877d2dbb8709 -oyaml | grep mount.juicefs
```
