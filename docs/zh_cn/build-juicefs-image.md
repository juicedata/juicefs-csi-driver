---
sidebar_label: 构建 JuiceFS CSI 驱动的容器镜像
---

# 如何构建 JuiceFS CSI 驱动的容器镜像

JuiceFS CSI 驱动中包含了多种类型的组件，不同组件使用了不同的容器镜像，以下分别介绍如何构建特定组件的容器镜像。

## 构建 JuiceFS CSI Controller 及 JuiceFS CSI Node 的容器镜像

JuiceFS CSI Controller 及 JuiceFS CSI Node 默认使用的容器镜像是 [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver)，对应的 Dockerfile 是 [`docker/Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/Dockerfile)。你可以通过如下命令构建容器镜像：

容器镜像中默认包含了 JuiceFS 社区版以及 JuiceFS 云服务最新版本的客户端。如果你希望修改代码并构建自己的 CSI 镜像，可以执行以下命令：

```shell
DEV_REGISTRY=foo/juicefs-csi-driver make image-dev
```

## 构建 JuiceFS Mount Pod 的容器镜像

JuiceFS Mount Pod 默认使用的容器镜像是 [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount)，对应的 Dockerfile 是 [`docker/juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile)。

如果您想构建自己的镜像，可以遵循以下步骤：

1. 将 JuiceFS 仓库克隆到 JuiceFS CSI 驱动项目的根目录，并切换到你想要编译的分支或者按需要修改代码：

   ```shell
   git clone git@github.com:juicedata/juicefs.git
   cd juicefs
   ```

2. 执行以下命令构建镜像：

   ```shell
   docker build -t foo/mount:latest -f ../docker/dev.juicefs.Dockerfile .
   ```

构建完后，可以参照[这篇文档](examples/mount-image.md) 在 PV/StorageClass 中指定 Mount Pod 的镜像。
