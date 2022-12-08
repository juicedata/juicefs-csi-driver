---
title: 介绍
---

## 架构 {#architecture}

[JuiceFS CSI 驱动](https://github.com/juicedata/juicefs-csi-driver)遵循 [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) 规范，实现了容器编排系统与 JuiceFS 文件系统之间的接口。在 Kubernetes 下，JuiceFS 可以用持久卷（PersistentVolume）的形式提供给 Pod 使用。

JuiceFS CSI 驱动包含以下组件：JuiceFS CSI Controller（StatefulSet）以及 JuiceFS CSI Node Service（DaemonSet），你可以方便地用 `kubectl` 查看：

```shell
$ kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS        RESTARTS   AGE
juicefs-csi-controller-0   2/2     Running       0          141d
juicefs-csi-node-8rd96     3/3     Running       0          141d
```

JuiceFS CSI 驱动的架构如图所示：

![](./images/csi-driver-architecture.svg)

JuiceFS CSI 驱动采用单独的 Mount Pod 来运行 JuiceFS 客户端，并由 CSI Node Service 来管理 Mount Pod 的生命周期。这样的架构提供如下好处：

* 多个 Pod 共用 PV 时，不会新建 Mount Pod，而是对已有的 Mount Pod 做引用计数，计数归零时删除 Mount Pod。
* CSI 驱动组件与客户端解耦，方便 CSI 驱动自身的升级。详见[「升级」](./administration/upgrade-csi-driver.md)。

在同一个节点上，一个 PVC 会对应一个 Mount Pod。PVC、PV、Mount Pod 之间的关系如下图所示：

![](./images/mount-pod-architecture.svg)

## 使用方式 {#usage}

你可以用[「静态配置」](./guide/pv.md#static-provisioning)和[「动态配置」](./guide/pv.md#dynamic-provisioning)的方式来使用 JuiceFS CSI 驱动。「静态配置」最为简单直接，需要你自行创建 PersistentVolume（PV）以及 PersistentVolumeClaim（PVC），流程类似[「配置 Pod 以使用 PersistentVolume 作为存储」](https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/configure-persistent-volume-storage/)。创建完毕以后，手动在 Pod 定义中引用该 PVC，就可以在容器中使用 JuiceFS 了。一般在以下场景使用静态配置：

* 你在 JuiceFS 中已经存储了大量数据，想要直接在 Kubernetes 容器中访问。
* 对 JuiceFS CSI 驱动功能做简单验证。

考虑到静态配置的管理更加复杂，规模化使用 CSI 驱动时，一般会以[「动态配置」](./guide/pv.md#dynamic-provisioning)方式使用。这种模式下，你只需要创建 PVC，并且在 Pod 中引用该 PVC，JuiceFS CSI 驱动会为你自动创建 PV。其流程大致如下：

* 用户创建 PVC，使用 JuiceFS 作为 StorageClass；
* CSI Controller 负责在 JuiceFS 文件系统中做初始化，默认以 PV ID 为名字创建子目录，同时创建对应的 PV；
* Kubernetes (PV Controller 组件) 将上述用户创建的 PVC 与 CSI Controller 创建的 PV 进行绑定，此时 PVC 与 PV 的状态变为「Bound」；
* 用户创建应用 Pod，Pod 中声明使用先前创建的 PVC；
* CSI Node Service 负责在应用 Pod 所在节点创建 Mount Pod；
* Mount Pod 启动，执行 JuiceFS 客户端挂载，运行 JuiceFS 客户端，挂载路径暴露在宿主机上，路径为 `/var/lib/juicefs/volume/[pv-name]`；
* CSI Node Service 等待 Mount Pod 启动成功后，将 PV 对应的 JuiceFS 子目录 bind 到容器内，路径为其声明的 VolumeMount 路径；
* Kubelet 创建应用 Pod。

因此在使用 JuiceFS CSI 驱动时，应用 Pod 总是与 Mount Pod 一起存在：

```
default       app-web-xxx            1/1     Running        0            1d
kube-system   juicefs-host-pvc-xxx   1/1     Running        0            1d
```

阅读以下文章深入了解 CSI 驱动的架构设计：

* [JuiceFS CSI Driver 架构设计详解](https://juicefs.com/zh-cn/blog/engineering/juicefs-csi-driver-arch-design)

## 进程挂载模式 {#by-process}

相较于采用独立 mount pod 的容器挂载方式，JuiceFS CSI 驱动还提供无需 mount pod 的进程挂载模式，在这种模式下，CSI Node Service 容器中将会负责运行一个或多个 JuiceFS 客户端，该节点上所有需要挂载的 JuiceFS PV，均在 CSI Node Service 容器中以进程模式执行挂载。

在 CSI Node Service 和 CSI Controller 的启动参数中添加 `--by-process=true`，就能启用进程挂载模式。

可想而知，由于所有 JuiceFS 客户端均在 CSI Node Service 容器中运行，CSI Node Service 将需要更大的资源声明，推荐将其资源请求调大到至少 1 CPU 和 1GiB 内存，资源约束调大到至少 2 CPU 和 5GiB 内存，或者根据实际场景资源占用进行调整。

在 Kubernetes 中，容器挂载模式无疑是更加推荐的 CSI 驱动用法，但脱离 Kubernetes 的某些场景，则可能需要选用进程挂载模式，比如[「在 Nomad 中使用 JuiceFS CSI 驱动」](./cookbook/csi-in-nomad.md)。

在 v0.10 之前，JuiceFS CSI 驱动仅支持进程挂载模式。而 v0.10 及之后版本则默认为容器挂载模式。如果你需要升级到 v0.10，请参考[「进程挂载模式下升级」](./administration/upgrade-csi-driver.md#mount-by-process-upgrade)。
