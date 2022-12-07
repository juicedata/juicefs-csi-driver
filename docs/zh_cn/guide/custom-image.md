---
title: 定制镜像
---

本章介绍如何设置 Mount Pod 镜像，以及自行构建 Mount Pod 以及 CSI 驱动组件镜像。

## 覆盖默认容器镜像 {#overwrite-mount-pod-image}

JuiceFS CSI 驱动 0.17.1 及以上版本支持自定义 Mount Pod 镜像。你可以在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v) 找到 CSI 驱动所使用的 Mount Pod 容器镜像，格式为 `juicedata/mount:v<JUICEFS-CE-LATEST-VERSION>-<JUICEFS-EE-LATEST-VERSION>`，其中 `<JUICEFS-CE-LATEST-VERSION>` 表示 JuiceFS 社区版客户端的最新版本号（如 `1.0.0`），`<JUICEFS-EE-LATEST-VERSION>` 表示 JuiceFS 云服务客户端的最新版本号（如 `4.8.0`）。

修改 JuiceFS CSI Node，在 `juicefs-plugin` 容器中设置 `JUICEFS_MOUNT_IMAGE` 环境变量，即可覆盖默认的 Mount Pod 镜像：

```shell
kubectl -n kube-system edit daemonset juicefs-csi-node
```

修改内容如下：

```yaml {11-12}
apiVersion: apps/v1
kind: DaemonSet
...
spec:
  template:
    spec:
      containers:
      - name: juicefs-plugin
        image: juicedata/juicefs-csi-driver:nightly
        env:
        - name: JUICEFS_MOUNT_IMAGE
          value: juicedata/mount:patch-some-bug
```

## 动态配置

对于[「动态配置」](./pv.md#dynamic-provisioning)用法，需要在 `StorageClass` 中配置 Mount Pod 镜像：

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

## 静态配置

对于[「静态配置」](./pv.md#static-provisioning)用法，需要在 `PersistentVolume` 中配置 Mount Pod 镜像：

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

## 构建镜像

### 构建 JuiceFS Mount Pod 的容器镜像 {#build-mount-pod-image}

JuiceFS Mount Pod 默认使用的容器镜像是 [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount)，对应的 Dockerfile 是 [`docker/juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile)。

如果您想构建自己的镜像，可以遵循以下步骤：

1. 将 JuiceFS 仓库克隆到 JuiceFS CSI 驱动项目的根目录，并切换到你想要编译的分支或者按需要修改代码：

   ```shell
   git clone https://github.com/juicedata/juicefs
   cd juicefs
   ```

2. 执行以下命令构建镜像，并上传至私有镜像仓库：

   ```shell
   docker build -t registry.example.com/mount:latest -f ../docker/dev.juicefs.Dockerfile .
   docker push registry.example.com/mount:latest
   ```

镜像构建完后，参照[覆盖默认容器镜像](#overwrite-mount-pod-image)来指定刚刚构建好的 Mount Pod 的镜像。

JuiceFS CSI 驱动中包含了多种类型的组件，不同组件使用了不同的容器镜像，这里分别介绍如何构建特定组件的容器镜像。

### 构建 JuiceFS CSI Controller 及 JuiceFS CSI Node 的容器镜像

JuiceFS CSI Controller 及 JuiceFS CSI Node 默认使用的容器镜像是 [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver)，对应的 Dockerfile 是 [`docker/Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/Dockerfile)。

容器镜像中默认包含了 JuiceFS 社区版以及 JuiceFS 云服务最新版本的客户端。如果你希望修改代码并构建自己的容器镜像，可以执行以下命令：

```shell
IMAGE=foo/juicefs-csi-driver make image-dev
```
