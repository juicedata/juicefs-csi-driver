---
title: 定制容器镜像
sidebar_position: 4
---

本章介绍如何设置 Mount Pod 镜像，以及自行构建 CSI 驱动组件镜像。

## Mount Pod 镜像拆分 {#ce-ee-separation}

Mount Pod 中运行着 JuiceFS 客户端，而 JuiceFS 又提供[「社区版」](https://juicefs.com/docs/zh/community/introduction)和[「商业版」](https://juicefs.com/docs/zh/cloud)客户端，因此在很长一段时间，Mount 镜像中同时包含着两个版本的 JuiceFS 客户端：

* `/usr/local/bin/juicefs`：社区版 JuiceFS 客户端
* `/usr/bin/juicefs`：商业版 JuiceFS 客户端

为了避免误用、同时精简容器镜像，在 CSI 驱动 0.19.0 及以上版本对镜像进行了拆分，你可以在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v) 找到 CSI 驱动所使用的 Mount Pod 容器镜像，形如：

```shell
# 社区版镜像标签以 ce- 开头
juicedata/mount:ce-v1.1.0

# 商业版镜像标签以 ee- 开头
juicedata/mount:ee-5.0.2-69f82b3

# 在 0.19.0 以前，镜像标签中包含社区版和商业版客户端的版本号
# 该系列镜像不再继续更新维护
juicedata/mount:v1.0.3-4.8.3
```

## 覆盖 Mount Pod 镜像 {#overwrite-mount-pod-image}

JuiceFS CSI 驱动 0.17.1 及以上版本支持自定义 Mount Pod 镜像，有多种修改 Mount Pod 镜像的方式，满足不同的定制需要，根据实际情况选择合适的手段。

:::tip 提示
覆盖 mount 镜像后，注意：

* 已有的 Mount Pod 不会受影响，需要随着应用 Pod 滚动升级或者重新创建 PVC，才会采用新的镜像
* [升级 CSI 驱动](../administration/upgrade-csi-driver.md)时，默认会连带升级到 mount 镜像的最新稳定版。但如果你覆盖了 mount 镜像，那么这就是固定的配置了，继续升级 CSI 驱动，也不会引入连带的 mount 镜像升级
:::

### 全局修改 {#overwrite-in-csi-node}

如果你用 Helm 安装 CSI 驱动，修改 mount 镜像非常简单，在 values 中定义即可：

```yaml
defaultMountImage:
  # 社区版
  ce: "juicedata/mount:ce-v1.1.2"
  # 企业版
  ee: "juicedata/mount:ee-5.0.10-10fbc97"
```

而如果是 kubectl 直接安装，那么需要在 CSI 驱动的组件中设置环境变量：

```shell
# 社区版
kubectl -n kube-system set env daemonset/juicefs-csi-node -c juicefs-plugin JUICEFS_CE_MOUNT_IMAGE=juicedata/mount:ce-v1.1.0
kubectl -n kube-system set env statefulset/juicefs-csi-controller -c juicefs-plugin JUICEFS_CE_MOUNT_IMAGE=juicedata/mount:ce-v1.1.0

# 企业版
kubectl -n kube-system set env daemonset/juicefs-csi-node -c juicefs-plugin JUICEFS_EE_MOUNT_IMAGE=juicedata/mount:ee-5.0.2-69f82b3
kubectl -n kube-system set env statefulset/juicefs-csi-controller -c juicefs-plugin JUICEFS_EE_MOUNT_IMAGE=juicedata/mount:ee-5.0.2-69f82b3
```

修改完毕以后，别忘了将这些配置同时加入 `k8s.yaml`，避免下次安装时配置丢失。正因为 kubectl 的安装方式管理配置不方便，所以建议在生产集群采用 [Helm 安装方式](../getting_started.md#helm)。

### 动态配置 {#overwrite-in-sc}

CSI 驱动允许[在 StorageClass 中进行覆盖](#overwrite-in-sc)，如果你需要为不同应用配置不同的 Mount Pod 镜像，那就需要创建多个 StorageClass，为每个 StorageClass 单独指定所使用的 Mount Pod 镜像。

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
  juicefs/mount-image: juicedata/mount:ce-v1.1.0
```

配置完成后，在不同的 PVC 中，通过 `storageClassName` 指定不同的 StorageClass，便能为不同的应用设置不同的 Mount Pod 镜像了。

### 静态配置

对于[「静态配置」](./pv.md#static-provisioning)用法，需要在 PV 定义中配置 Mount Pod 镜像：

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
      juicefs/mount-image: juicedata/mount:ce-v1.1.0
```

## 构建镜像

### 构建 Mount Pod 的容器镜像 {#build-mount-pod-image}

JuiceFS CSI 驱动采用[「分离架构」](../introduction.md#architecture)，Mount Pod 默认使用的容器镜像是 [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount)，社区版对应的 Dockerfile 是 [`docker/ce.juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/ce.juicefs.Dockerfile)。

因此，如果要自行构建挂载镜像，可以参考下方命令克隆 JuiceFS 社区版仓库，然后直接用内置的 Dockerfile 执行构建：

```shell
# 克隆 JuiceFS 社区版仓库
git clone https://github.com/juicedata/juicefs
cd juicefs

# 切换到你想要编译的分支，或者按需要修改代码
git checkout ...

# 由于 Dockerfile 在 CSI 驱动的仓库，此处需要自行下载
curl -O https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/docker/ce.juicefs.Dockerfile

# 构建镜像，并上传至私有镜像仓库
docker build -t registry.example.com/juicefs-csi-mount:ce-latest -f ce.juicefs.Dockerfile .
docker push registry.example.com/juicefs-csi-mount:ce-latest
```

参照[覆盖默认容器镜像](#overwrite-mount-pod-image)来指定刚刚构建好的 Mount Pod 的镜像。

### 构建 CSI 驱动组件镜像

JuiceFS CSI Controller 及 JuiceFS CSI Node 默认使用的容器镜像是 [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver)，对应的 Dockerfile 是 [`docker/csi.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/csi.Dockerfile)。

若希望深入开发 JuiceFS CSI 驱动，可以参考下方命令克隆仓库，然后执行内置的构建脚本：

```shell
# 克隆 CSI 驱动仓库
git clone https://github.com/juicedata/juicefs-csi-driver
cd juicefs-csi-driver

# 切换到你想要编译的分支，或者按需要修改代码
git checkout ...

# 用 IMAGE 环境变量指定将要构建的 CSI 驱动镜像名称，并上传至私有镜像仓库
IMAGE=registry.example.com/juicefs-csi-driver make image-dev
docker push registry.example.com/juicefs-csi-driver:dev-xxx
```
