---
title: 定制容器镜像
sidebar_position: 7
---

本章介绍如何设置 Mount Pod 镜像，以及自行构建 CSI 驱动组件镜像。

## Mount Pod 镜像拆分 {#ce-ee-separation}

Mount Pod 中运行着 JuiceFS 客户端，而 JuiceFS 又提供[「社区版」](https://juicefs.com/docs/zh/community/introduction)和[「商业版」](https://juicefs.com/docs/zh/cloud)客户端，因此在很长一段时间，Mount 镜像中同时包含着两个版本的 JuiceFS 客户端：

* `/usr/local/bin/juicefs`：社区版 JuiceFS 客户端
* `/usr/bin/juicefs`：商业版 JuiceFS 客户端

为了避免误用、同时精简容器镜像，在 CSI 驱动 0.19.0 及以上版本对镜像进行了拆分，你可以在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags) 找到 CSI 驱动所使用的 Mount Pod 容器镜像，形如：

:::tip
如果你需要把 Mount Pod 容器镜像从 Docker Hub 搬运到其它镜像仓库，请参考[文档](../administration/offline.md#copy-images)。
:::

```shell
# 社区版镜像标签以 ce- 开头
juicedata/mount:ce-v1.3.1

# 商业版镜像标签以 ee- 开头
juicedata/mount:ee-5.3.8-fc708b6

# 在 0.19.0 以前，镜像标签中包含社区版和商业版客户端的版本号
# 该系列镜像不再继续更新维护
juicedata/mount:v1.0.3-4.8.3
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
curl -O https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/docker/dev.juicefs.Dockerfile

# 构建镜像，并上传至私有镜像仓库
docker build -t registry.example.com/juicefs-csi-mount:ce-latest -f dev.juicefs.Dockerfile .
docker push registry.example.com/juicefs-csi-mount:ce-latest
```

镜像构建完毕以后，请参考[「升级 JuiceFS 客户端」](../administration/upgrade-juicefs-client.md)来指定刚刚构建好的 Mount Pod 的镜像。

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
