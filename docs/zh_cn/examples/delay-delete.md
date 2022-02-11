---
sidebar_label: 延迟删除 mount pod
---

# 如何延迟删除 mount pod

JuiceFS CSI 驱动在没有应用 pod 使用 mount pod 的时候，会立即删除 mount pod。但在某些时候，您可能希望 mount pod 被延迟删除，如果短时间内还有新应用 Pod 使用相同的 JuiceFS
volume，mount pod 不会被摧毁重建，造成不必要的资源浪费。

本文档展示如何设置 mount pod 的延迟删除时长。

## 静态配置

您可以在 PV 中配置延迟删除的时长，在 `volumeAttributes` 中设置 `juicefs/mount-delete-delay`，值为需要设置的时长，如下：

```yaml
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
    volumeHandle: test-bucket
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/mount-delete-delay: 1m
```

其中，单位可以为："ns"（纳秒），"us"（微秒），"ms"（毫秒），"s"（秒），"m"（分钟），"h"（小时）。

当最后一个应用 pod 删除后，mount pod 被打上 `juicefs-delete-at` 的 annotation，记录应该被删除的时刻，当到了设置的删除时间后，mount pod 才会被删除；
当有新的应用 Pod 使用相同 JuiceFS Volume 后，annotation `juicefs-delete-at` 会被删除。

部署 PVC 和示例 pod：

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  storageClassName: ""
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app-mount-options
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: data
      resources:
        requests:
          cpu: 10m
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-pvc
```

## 动态配置

您也可以在 StorageClass 中配置延迟删除的时长，在 `parameters` 中设置 `juicefs/mount-delete-delay`，值为需要设置的时长，如下：

```yaml
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
  juicefs/mount-delete-delay: 1m
```

部署 PVC 和示例 pod：

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app-mount-options
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: juicefs-pv
  volumes:
    - name: juicefs-pv
      persistentVolumeClaim:
        claimName: juicefs-pvc
```
