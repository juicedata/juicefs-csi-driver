---
title: 介绍
---

## 架构 {#architecture}

[JuiceFS CSI 驱动](https://github.com/juicedata/juicefs-csi-driver)遵循 [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) 规范，实现了容器编排系统与 JuiceFS 文件系统之间的接口。在 Kubernetes 下，JuiceFS 可以用持久卷（PersistentVolume）的形式提供给 Pod 使用。

<div className="video-container">
  <iframe src="//player.bilibili.com/player.html?aid=898153616&bvid=BV1qN4y1M7Nk&cid=933003550&page=1&autoplay=0" width="100%" height="360" scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true"> </iframe>
</div>

JuiceFS CSI 驱动包含以下组件：JuiceFS CSI Controller（StatefulSet）以及 JuiceFS CSI Node Service（DaemonSet），你可以方便地用 `kubectl` 查看：

```shell
$ kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS        RESTARTS   AGE
juicefs-csi-controller-0   2/2     Running       0          141d
juicefs-csi-node-8rd96     3/3     Running       0          141d
```

CSI 默认采用容器挂载（Mount Pod）模式，也就是让 JuiceFS 客户端运行在独立的 Pod 中，其架构如下：

![CSI-driver-architecture](./images/csi-driver-architecture.svg)

采用独立 Mount Pod 来运行 JuiceFS 客户端，并由 CSI Node Service 来管理 Mount Pod 的生命周期。这样的架构提供如下好处：

* 多个 Pod 共用 PV 时，不会新建 Mount Pod，而是对已有的 Mount Pod 做引用计数，计数归零时删除 Mount Pod。
* CSI 驱动组件与客户端解耦，方便 CSI 驱动自身的升级。详见[「升级」](./administration/upgrade-csi-driver.md)。

在同一个节点上，一个 PVC 会对应一个 Mount Pod。而使用了相同 PV 的容器，则可以共享一个 Mount Pod。PVC、PV、Mount Pod 之间的关系如下图所示：

![mount-pod-architecture](./images/mount-pod-architecture.svg)

如果该模式不适用于你的场景，CSI 驱动还提供其他机制，详见[「其他运行模式」](#other-mount-modes)。

## 使用方式 {#usage}

你可以以[「静态配置」](./guide/pv.md#static-provisioning)和[「动态配置」](./guide/pv.md#dynamic-provisioning)的方式来使用 CSI 驱动。

### 静态配置

静态配置方式最为简单直接，会直接将整个文件系统的根目录作为 PV 挂载到容器里（当然，也可以指定[子目录挂载](./guide/pv.md#mount-subdirectory)）。这种方式需要 Kubernetes 管理员创建 PersistentVolume（PV）以及[文件系统认证信息](./guide/pv.md#volume-credentials)（以 Kubernetes Secret 形式保存），然后用户创建 PersistentVolumeClaim（PVC），在定义中绑定该 PV，最后在 Pod 定义中引用该 PVC。资源间关系如下图所示：

![static-provisioning](./images/static-provisioning.svg)

一般在以下场景使用静态配置：

* 你在 JuiceFS 中已经存储了大量数据，想要直接在 Kubernetes 容器中访问；
* 对 CSI 驱动功能做简单验证；

### 动态配置

考虑到静态配置的管理比较复杂，需要手动创建 PV，所以有大量应用需要使用 CSI 驱动时，一般会以「动态配置」方式使用，管理员不再需要手动创建 PV，同时实现应用间的数据隔离。这种模式下，管理员会负责创建一个或多个 StorageClass，用户只需要创建 PVC，指定 StorageClass，并且在 Pod 中引用该 PVC，CSI 驱动就会按照 StorageClass 中配置好的参数，为你自动创建 PV，每一个 PV 对应着 JuiceFS 文件系统的一个子目录。

动态配置的资源间关系如下：

![dynamic-provisioning](./images/dynamic-provisioning.svg)

以容器挂载模式为例，从创建到使用的流程大致如下：

* 用户创建 PVC，指定已经创建好的 StorageClass；
* CSI Controller 负责在 JuiceFS 文件系统中做初始化，默认以 PV ID 为名字创建子目录，同时创建对应的 PV。该过程所需的配置，都在 StorageClass 中指定或引用；
* Kubernetes (PV Controller 组件) 将上述用户创建的 PVC 与 CSI Controller 创建的 PV 进行绑定，此时 PVC 与 PV 的状态变为「Bound」；
* 用户创建应用 Pod，声明使用先前创建的 PVC；
* CSI Node Service 负责在应用 Pod 所在节点创建 Mount Pod；
* Mount Pod 启动，执行 JuiceFS 客户端挂载，将挂载点暴露给宿主机，路径为 `/var/lib/juicefs/volume/[pv-name]`；
* CSI Node Service 等待 Mount Pod 启动成功后，将 PV 对应的 JuiceFS 子目录 bind 到容器内，路径为其声明的 VolumeMount 路径；
* Kubelet 启动应用 Pod。

阅读以下文章深入了解 CSI 驱动的架构设计：

* [JuiceFS CSI Driver 架构设计详解](https://juicefs.com/zh-cn/blog/engineering/juicefs-csi-driver-arch-design)

## 其他运行模式 {#other-mount-modes}

CSI 驱动默认以容器挂载（Mount Pod）模式运行，但特定场景下该模式不一定适用，因此 CSI 驱动还提供以下运行模式。

### Sidecar 模式 {#sidecar}

Mount Pod 需要由 CSI Node 创建，考虑到 CSI Node 是一个 DaemonSet 组件，如果你的 Kubernetes 集群不支持部署 DaemonSet（比如一些云服务商提供的 Serverless Kubernetes 服务），那么 CSI Node 将无法部署，也就无法正常使用 CSI 驱动。对于这种情况，可以选择使用 CSI 驱动的 Sidecar 模式，让 JuiceFS 客户端运行在 Sidecar 容器中。

<div className="video-container">
  <iframe src="//player.bilibili.com/player.html?aid=266921439&bvid=BV1YY411e72C&cid=1016796350&page=1&autoplay=0" width="100%" height="360" scrolling="no" border="0" frameborder="no" framespacing="0" allowfullscreen="true"> </iframe>
</div>

以 Sidecar 模式安装 CSI 驱动，所部署的组件只有 CSI Controller，不再需要 CSI Node。对于需要使用 CSI 驱动的 Kubernetes 命名空间，CSI Controller 会监听容器变动，检查是否使用了 JuiceFS PVC，并根据情况为其注入 Sidecar 容器。

![sidecar-architecture](./images/sidecar-architecture.svg)

创建和使用的流程大致如下：

* CSI Controller 启动时，向 API Server 注册 Webhook；
* 应用 Pod 指定使用 JuiceFS PVC；
* API Server 在创建应用 Pod 前调用 CSI Controller 的 Webhook 接口；
* CSI Controller 向应用 Pod 中注入 Sidecar 容器，容器中运行着 JuiceFS 客户端；
* API Server 创建应用 Pod，Sidecar 容器启动后运行 JuiceFS 客户端执行挂载，应用容器启动后可直接访问文件系统。

使用 Sidecar 模式需要注意：

* 运行环境需要支持 FUSE，也就是支持以特权容器（Privileged）运行；
* 不同于 Mount Pod 的容器挂载方式，Sidecar 容器注入进了应用 Pod，因此将无法进行任何复用，大规模场景下，请尤其注意资源规划和分配；
* Sidecar 容器和应用容器的挂载点共享是通过 `hostPath` 实现的，是一个有状态服务，如果 Sidecar 容器发生意外重启，应用容器中的挂载点不会自行恢复，需要整个 Pod 重新创建（相较下，Mount Pod 模式则支持[挂载点自动恢复](./guide/pv.md#automatic-mount-point-recovery)）；
* 不要直接从 Mount Pod 模式升级成 Sidecar 模式。已有的 Mount Pod 在 Sidecar 模式下将无法回收。并且一般而言，考虑到 Sidecar 不支持复用，我们不推荐从 Mount Pod 模式迁移为 Sidecar 模式；
* 对于启用了 Sidecar 注入的命名空间，CSI Controller 会监听该命名空间下创建的所有容器，检查 PVC 的使用并查询获取相关信息。如果希望最大程度地减小开销，可以在该命名空间下，对不使用 JuiceFS PV 的应用 Pod 打上 `disable.sidecar.juicefs.com/inject: true` 标签，让 CSI Controller 忽略这些不相关的容器。

欲使用 Sidecar 模式，需要[以 Sidecar 模式安装 CSI 驱动](./getting_started.md#sidecar)。安装完毕以后，继续阅读[「在 Serverless 环境中使用 JuiceFS CSI 驱动」]了解如何在各个云服务商的 Serverless 产品中使用 CSI 驱动。

### 进程挂载模式 {#by-process}

相较于采用独立 Mount Pod 的容器挂载方式或 Sidecar 模式，CSI 驱动还提供无需独立 Pod 的进程挂载模式，在这种模式下，CSI Node Service 容器中将会负责运行一个或多个 JuiceFS 客户端，该节点上所有需要挂载的 JuiceFS PV，均在 CSI Node Service 容器中以进程模式执行挂载。

![byprocess-architecture](./images/byprocess-architecture.svg)

可想而知，由于所有 JuiceFS 客户端均在 CSI Node Service 容器中运行，CSI Node Service 将需要更大的资源声明，推荐将其资源请求调大到至少 1 CPU 和 1GiB 内存，资源约束调大到至少 2 CPU 和 5GiB 内存，或者根据实际场景资源占用进行调整。

在 Kubernetes 中，容器挂载模式无疑是更加推荐的 CSI 驱动用法，但脱离 Kubernetes 的某些场景，则可能需要选用进程挂载模式，比如[「在 Nomad 中使用 JuiceFS CSI 驱动」](./cookbook/csi-in-nomad.md)。

在 v0.10 之前，JuiceFS CSI 驱动仅支持进程挂载模式。而 v0.10 及之后版本则默认为容器挂载模式。如果你需要升级到 v0.10，请参考[「进程挂载模式下升级」](./administration/upgrade-csi-driver.md#mount-by-process-upgrade)。

欲使用进程挂载模式，需要[以进程挂载模式安装 CSI 驱动](./getting_started.md#by-process)。
