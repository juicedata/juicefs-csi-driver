---
title: 挂载子目录
sidebar_position: 3
---

在 JuiceFS CSI 驱动中，你可以用 `subdir` 和 `subPath` 两种方式来实现挂载子目录。其中，`subdir` 是指直接用 JuiceFS 提供的子路径挂载特性（`juicefs mount --subdir`）来挂载子目录，而 `subPath` 则是由 CSI 驱动将指定的子路径 [bind mount](https://docs.docker.com/storage/bind-mounts) 到应用 Pod 中。

注意以下使用场景必须使用 `subdir` 来挂载子目录，而不能使用 `subPath`：

- 需要在应用 Pod 中进行[缓存预热](./cache.md#warmup)
- （云服务）挂载所使用的 [Token](https://juicefs.com/docs/zh/cloud/acl) 只有子目录的访问权限

## 使用 `subdir`

在 PV 的 `mountOptions` 中指定 `subdir=xxx` 即可，如果指定的子目录不存在，CSI Controller 会自动创建。

```yaml {21-22}
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
  mountOptions:
    - subdir=/test
```

## 使用 `subPath`

在 PV 中这样使用 `subPath`，如果指定的子目录不存在，CSI Controller 会自动创建。

```yaml {21-22}
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
      subPath: fluentd
```
