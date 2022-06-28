---
sidebar_label: PV 的回收策略
---

# PV 的回收策略

## 静态配置

静态配置中，只支持 Retain 回收策略，即需要集群管理员手动回收资源。配置方式如下：

```yaml {13}
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
```

## 动态配置

动态配置中，支持 Retain 和 Delete 两种回收策略，Retain 指由管理员手动回收资源；Delete 指自动回收动态创建的资源。配置方式如下：

```yaml {6}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
reclaimPolicy: Retain
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
```

动态配置的 PV 会继承其 StorageClass 中设置的回收策略，该策略默认为 Delete。
