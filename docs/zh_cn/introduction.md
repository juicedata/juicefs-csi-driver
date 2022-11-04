---
sidebar_label: 介绍
---

# JuiceFS CSI 驱动

[JuiceFS CSI 驱动](https://github.com/juicedata/juicefs-csi-driver)遵循 [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) 规范，实现了容器编排系统与 JuiceFS 文件系统之间的接口，让 JuiceFS 文件系统以卷（Volume）的形式提供给 Pod 使用。

JuiceFS CSI 驱动会在你的集群中部署以下组件：JuiceFS CSI Controller（StatefulSet）以及 JuiceFS CSI Node Service（DaemonSet），你可以方便地用 `kubectl` 查看：

```shell
$ kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS        RESTARTS   AGE
juicefs-csi-controller-0   2/2     Running       0          141d
juicefs-csi-node-8rd96     3/3     Running       0          141d
```

JuiceFS CSI 驱动的架构如图所示：

![](./images/csi-driver-architecture.jpg)

顺着架构图简单介绍一番 CSI 驱动的工作原理：JuiceFS CSI 驱动采用单独的 Mount Pod 来运行 JuiceFS 客户端，并由 Node Service 来管理 Mount Pod 的生命周期。这样的架构提供如下好处：

* 允许多个应用 Pod 共用 PV，当多个 Pod 共用 PV 时，不会新建 Mount Pod，而是对已有的 Mount Pod 做引用计数，计数归零时删除 Mount Pod。
* CSI 驱动组件与客户端解耦，方便 CSI 驱动自身的升级。详见[「升级」](./upgrade-csi-driver.md)。

以[「动态配置」](./examples/dynamic-provisioning.md)为例，创建 PV 和使用的流程大致如下：

* 用户创建 PVC (PersistentVolumeClaim)，声明其使用的 StorageClass；
* CSI Controller 负责在 JuiceFS 文件系统中做初始化，默认以 PV id 为 name 创建子目录，同时创建对应的 PV（PersistentVolume）；
* Kubernetes (PVController 组件) 将上述用户创建的 PVC 与 CSI Controller 创建的 PV 进行绑定，此时 PVC 与 PV 的状态皆为 bound；
* 用户创建应用 Pod，Pod 中声明使用的 PVC；
* CSI Node Service 负责在应用 Pod 所在节点创建 Mount Pod，并等待其启动成功；
* Mount Pod 启动，执行 JuiceFS 客户端挂载，运行 JuiceFS 客户端，挂载路径暴露在宿主机上，路径为 `/var/lib/juicefs/volume/[pv-name]`；
* CSI 等待 Mount Pod 启动成功后，将 PV 对应的 JuiceFS 子目录 bind 到容器内，路径为其申明的 VolumeMount 路径；
* Kubelet 创建应用 Pod。

因此在使用 JuiceFS CSI 驱动时，应用 Pod 总是与 Mount Pod 一起存在：

```
default       app-web-xxx            1/1     Running        0            1d
kube-system   juicefs-host-pvc-xxx   1/1     Running        0            1d
```

阅读以下文章深入了解 CSI 驱动的架构设计：

* [JuiceFS CSI Driver 架构设计详解](https://juicefs.com/zh-cn/blog/engineering/juicefs-csi-driver-arch-design)
