---
sidebar_label: 构建 JuiceFS CSI 驱动的容器镜像
---

# 如何构建 JuiceFS CSI 驱动的容器镜像

JuiceFS CSI 驱动中包含了多种类型的组件，不同组件使用了不同的容器镜像，以下分别介绍如何构建特定组件的容器镜像。

## 构建 JuiceFS CSI Controller 及 JuiceFS CSI Node 的容器镜像

JuiceFS CSI Controller 及 JuiceFS CSI Node 默认使用的容器镜像是 [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver)，对应的 Dockerfile 是 [`docker/Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/Dockerfile)。你可以通过如下命令构建容器镜像：

```shell
make image-latest
```

构建完毕将会产生一个叫做 `juicedata/juicefs-csi-driver:latest` 的容器镜像，如果希望修改容器镜像的名称可以设置 `IMAGE` 环境变量：

```shell
IMAGE=foo/juicefs-csi-driver make image-latest
```

容器镜像中默认包含了 JuiceFS 社区版以及 JuiceFS 云服务最新版本的客户端，如果你希望使用不同版本的 JuiceFS 客户端，可以遵循以下步骤：

1. 将 JuiceFS 仓库克隆到 JuiceFS CSI 驱动项目的根目录，并切换到你想要编译的分支或者按需要修改代码：

   ```shell
   git clone git@github.com:juicedata/juicefs.git
   cd juicefs
   ```

2. 执行以下命令构建镜像：

   ```shell
   docker build -t foo/juicefs-csi-driver:latest -f ../docker/dev.juicefs.Dockerfile .
   ```

## 构建 JuiceFS Mount Pod 的容器镜像

JuiceFS Mount Pod 默认使用的容器镜像是 [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount)，对应的 Dockerfile 是 [`docker/juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile)。你可以通过如下命令构建容器镜像：

```shell
make juicefs-image
```

构建完毕将会产生一个叫做 `juicedata/mount:v<JUICEFS-CE-LATEST-VERSION>-<JUICEFS-EE-LATEST-VERSION>` 的容器镜像，其中 `<JUICEFS-CE-LATEST-VERSION>` 表示 JuiceFS 社区版客户端的最新版本号（如 `1.0.0`），`<JUICEFS-EE-LATEST-VERSION>` 表示 JuiceFS 云服务客户端的最新版本号（如 `4.8.0`）。如果希望修改容器镜像的名称可以设置 `JUICEFS_IMAGE` 环境变量：

```shell
JUICEFS_IMAGE=foo/mount make juicefs-image
```

容器镜像中默认包含了 JuiceFS 社区版以及 JuiceFS 云服务最新版本的客户端，如果你希望使用不同版本的 JuiceFS 客户端，可以设置 `JUICEFS_REPO_URL` 及 `JUICEFS_REPO_REF` 环境变量：

```shell
JUICEFS_REPO_URL=https://github.com/foo/juicefs JUICEFS_REPO_REF=v1.0.0 make juicefs-image
```
