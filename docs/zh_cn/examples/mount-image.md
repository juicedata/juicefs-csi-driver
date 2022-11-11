---
sidebar_label: 自定义 Mount Pod 的容器镜像
---

# 如何自定义 Mount Pod 的容器镜像

:::note 注意
此特性需使用 0.17.1 及以上版本的 JuiceFS CSI 驱动
若采用进程挂载的方式启动 CSI 驱动，即 CSI Node 和 CSI Controller 的启动参数使用 `--by-process=true`，则本文档的相关配置会被忽略。
:::

默认情况下，JuiceFS Mount Pod 的容器镜像为 `juicedata/mount:v<JUICEFS-CE-LATEST-VERSION>-<JUICEFS-EE-LATEST-VERSION>`，其中 `<JUICEFS-CE-LATEST-VERSION>` 表示 JuiceFS 社区版客户端的最新版本号（如 `1.0.0`），`<JUICEFS-EE-LATEST-VERSION>` 表示 JuiceFS 云服务客户端的最新版本号（如 `4.8.0`）。你可以在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags) 上查看所有镜像标签。

本文档展示了如何自定义 Mount Pod 的容器镜像，关于如何构建 Mount Pod 的容器镜像请参考[文档](../develop/build-juicefs-image.md#构建-juicefs-mount-pod-的容器镜像)。

## 安装 CSI 驱动时覆盖默认容器镜像

JuiceFS CSI Node 启动时，在 `juicefs-plugin` 容器中设置 `JUICEFS_MOUNT_IMAGE` 环境变量可覆盖默认的 Mount Pod 镜像：

:::note 注意
一旦 `juicefs-plugin` 容器启动，默认的 Mount Pod 镜像就无法修改。如需修改请重新创建容器，并设置新的 `JUICEFS_MOUNT_IMAGE` 环境变量。
:::

```yaml {12-13}
apiVersion: apps/v1
kind: DaemonSet
# metadata: ...
spec:
  template:
    # metadata: ...
    spec:
      containers:
      - name: juicefs-plugin
        image: juicedata/juicefs-csi-driver:nightly
        env:
        - name: JUICEFS_MOUNT_IMAGE
          value: juicedata/mount:patch-some-bug
```

## 在 `PersistentVolume` 中配置容器镜像

您也可以在 `PersistentVolume` 中配置 Mount Pod 镜像：

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
      juicefs/mount-image: juicedata/mount:patch-some-bug
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
  name: juicefs-app
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

### 检查 Mount Pod 的 image 设置

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您可以验证 Mount Pod 的 image 设置得是否正确：

```sh
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-mount -o yaml | grep 'image: '
```

## 在 `StorageClass` 中配置容器镜像

您可以在 `StorageClass` 中配置 Mount Pod 镜像：

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
  juicefs/mount-image: juicedata/mount:patch-some-bug
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
  name: juicefs-app
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

### 检查 Mount Pod 的 image 设置

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您可以验证 Mount Pod 的 image 设置得是否正确：

```sh
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-mount -o yaml | grep 'image: '
```
