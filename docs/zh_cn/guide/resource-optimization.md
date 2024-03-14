---
title: 资源优化
sidebar_position: 3
---

Kubernetes 的一大好处就是促进资源充分利用，在 JuiceFS CSI 驱动中，也有不少方面可以做资源占用优化，甚至带来一定的性能提升。在这里集中罗列介绍。

## 配置资源请求和约束 {#mount-pod-resources}

每一个使用着 JuiceFS PV 的容器，都对应着一个 Mount Pod（会智能匹配和复用），因此为 Mount Pod 配置合理的资源声明，将是最有效的优化资源占用的手段。关于配置资源请求（`request`）和约束（`limit`），可以详读 [Kubernetes 官方文档](https://kubernetes.io/zh-cn/docs/concepts/configuration/manage-resources-containers)，此处不赘述。

JuiceFS Mount Pod 的 `requests` 默认为 1 CPU 和 1GiB Memory，`limits` 默认为 2 CPU 和 5GiB Memory。考虑到 JuiceFS 的使用场景多种多样，1C1G 的资源请求可能不一定适合你的集群，比方说：

* 实际场景下用量极低，比如 Mount Pod 只使用了 0.1 CPU、100MiB Memory，那么你应该尊重实际监控数据，将资源请求调整为 0.1 CPU，100MiB Memory，避免过大的 `requests` 造成资源闲置，甚至导致容器拒绝启动，或者抢占其他应用容器（Preemption）。对于 `limits`，你也可以根据实际监控数据，调整为一个大于 `requests` 的数值，允许突发瞬时的资源占用上升。
* 实际场景下用量更高，比方说 2 CPU、2GiB 内存，此时虽然 1C1G 的默认 `requests` 允许容器调度到节点上，但实际资源占用高于 `requests`，这便是「资源超售」（Overcommitment），严重的超售会影响集群稳定性，让节点出现各种资源挤占的问题，比如 CPU Throttle、OOM。因此这种情况下，你也应该根据实际用量，调整 `requests` 和 `limits`。

如果你安装了 [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)，可以方便地用类似下方命令查看 CSI 驱动组件的实际资源占用：

```shell
# 查看 Mount Pod 实际资源占用
kubectl top pod -n kube-system -l app.kubernetes.io/name=juicefs-mount

# 查看 CSI Controller，CSI Node 实际资源占用，依同样的原理调整其资源声明
kubectl top pod -n kube-system -l app.kubernetes.io/name=juicefs-csi-driver
```

### 在 PVC 配置资源声明 {#mount-pod-resources-pvc}

自 0.23.4 开始，在 PVC 的 annotations 中可以自由配置资源声明，由于 annotations 可以随时更改，因此这也是最灵活、我们最推荐的方式。但也要注意：

* 修改以后，已有的 mount pod 并不会自动按照新的配置重建。需要删除 mount pod，才能以新的资源配置触发创建新的 mount pod。
* 必须配置好[挂载点自动恢复](./pv.md#automatic-mount-point-recovery)，重建后 mount pod 的挂载点才能传播回应用 pod。
* 就算配置好了挂载点自动恢复，重启过程也会造成服务闪断，注意在应用空间做好错误处理。

```yaml {6-9}
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
  annotations:
    juicefs/mount-cpu-request: 100m
    juicefs/mount-cpu-limit: "1"  # 数字必须以引号封闭，作为字符串传入
    juicefs/mount-memory-request: 500Mi
    juicefs/mount-memory-limit: 1Gi
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
```

### 其他方式（不推荐） {#deprecated-resources-definition}

:::warning
优先使用上方介绍的 PVC annotations 方式，他支持动态变更，所以是我们更为推荐的方式。而下方介绍的方式一旦设置成功，就无法修改，只能删除重建 PV，已不再推荐使用。
:::

静态配置中，可以在 `PersistentVolume` 中配置资源请求和约束：

```yaml {22-25}
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
      juicefs/mount-cpu-limit: 5000m
      juicefs/mount-memory-limit: 5Gi
      juicefs/mount-cpu-request: 100m
      juicefs/mount-memory-request: 500Mi
```

动态配置中，可以在 `StorageClass` 中配置资源请求和约束：

```yaml {11-14}
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
  juicefs/mount-cpu-limit: 5000m
  juicefs/mount-memory-limit: 5Gi
  juicefs/mount-cpu-request: 100m
  juicefs/mount-memory-request: 500Mi
```

在 0.23.4 以及之后的版本中，由于支持[参数模板化](./pv.md#)，可以在 StorageClass 的 `parameters` 参数支持模版配置：

```yaml {8-11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  ...
  juicefs/mount-cpu-limit: ${.pvc.annotations.csi.juicefs.com/mount-cpu-limit}
  juicefs/mount-memory-limit: ${.pvc.annotations.csi.juicefs.com/mount-memory-limit}
  juicefs/mount-cpu-request: ${.pvc.annotations.csi.juicefs.com/mount-cpu-request}
  juicefs/mount-memory-request: ${.pvc.annotations.csi.juicefs.com/mount-memory-request}
```

需要注意，由于已经支持[在 PVC annotations 定义 mount pod 资源](#mount-pod-resources-pvc)，已不需要用到此配置方法。

如果你使用 Helm 管理 StorageClass，则直接在 `values.yaml` 中定义：

```yaml title="values.yaml" {5-12}
storageClasses:
- name: juicefs-sc
  enabled: true
  ...
  mountPod:
    resources:
      requests:
        cpu: "100m"
        memory: "500Mi"
      limits:
        cpu: "5"
        memory: "5Gi"
```

## 为 Mount Pod 设置非抢占式 PriorityClass {#set-non-preempting-priorityclass-for-mount-pod}

:::tip 提示

- 建议默认为 Mount Pod 设置非抢占式 PriorityClass
- 如果 CSI 驱动的运行模式为[「Sidecar 模式」](../introduction.md#sidecar)，则不会遇到以下问题。
:::

CSI Node 在创建 Mount Pod 时，会默认给其设置 PriorityClass 为 `system-node-critical`，目的是为了在机器资源不足时，Mount Pod 不会被驱逐。

但在 Mount Pod 创建时，若机器资源不足，`system-node-critical` 会使得调度器为 Mount Pod 开启抢占，此时可能会影响到节点上已有的业务。若不希望现有的业务被影响，可以设置 Mount Pod 的 PriorityClass 为非抢占式的，具体方式如下：

1. 在集群中创建一个非抢占式 PriorityClass，更多 PriorityClass 信息参考[官方文档](https://kubernetes.io/zh-cn/docs/concepts/scheduling-eviction/pod-priority-preemption)：

   ```yaml
   apiVersion: scheduling.k8s.io/v1
   kind: PriorityClass
   metadata:
     name: juicefs-mount-priority-nonpreempting
   value: 1000000000           # 值越大，优先级越高，范围为 -2,147,483,648 到 1,000,000,000（含）。应尽可能大，确保 Mount Pod 不会被驱逐
   preemptionPolicy: Never     # 非抢占式
   globalDefault: false
   description: "This priority class used by JuiceFS Mount Pod."
   ```

2. 为 CSI Node Service 和 CSI Controller Service 添加 `JUICEFS_MOUNT_PRIORITY_NAME` 这个环境变量，值为上述 PriorityClass 名，同时添加环境变量 `JUICEFS_MOUNT_PREEMPTION_POLICY` 为 `Never`，设置 Mount Pod 的抢占策略为 Never：

   ```shell
   kubectl -n kube-system set env -c juicefs-plugin daemonset/juicefs-csi-node JUICEFS_MOUNT_PRIORITY_NAME=juicefs-mount-priority-nonpreempting JUICEFS_MOUNT_PREEMPTION_POLICY=Never
   kubectl -n kube-system set env -c juicefs-plugin statefulset/juicefs-csi-controller JUICEFS_MOUNT_PRIORITY_NAME=juicefs-mount-priority-nonpreempting JUICEFS_MOUNT_PREEMPTION_POLICY=Never
   ```

## 为相同的 StorageClass 复用 Mount Pod {#share-mount-pod-for-the-same-storageclass}

默认情况下，仅在多个应用 Pod 使用相同 PV 时，Mount Pod 才会被复用。如果你希望进一步降低开销，可以更加激进地复用 Mount Pod，让使用相同 StorageClass 创建出来的所有 PV，都复用同一个 Mount Pod（当然了，复用只能发生在同一个节点）。不同的应用 Pod，将会绑定挂载点下不同的路径，实现一个挂载点为多个应用容器提供服务。

为相同 StorageClass PV 复用 Mount Pod，需要为 CSI Node Service 添加 `STORAGE_CLASS_SHARE_MOUNT` 这个环境变量：

```shell
kubectl -n kube-system set env -c juicefs-plugin daemonset/juicefs-csi-node STORAGE_CLASS_SHARE_MOUNT=true
```

可想而知，高度复用意味着更低的隔离程度，如果 Mount Pod 发生意外，挂载点异常，影响面也会更大，因此如果你决定启用该复用策略，请务必同时启用[「挂载点自动恢复」](./pv.md#automatic-mount-point-recovery)，以及合理增加 [「Mount Pod 的资源请求」](#mount-pod-resources)。

## 配置 Mount Pod 退出时清理缓存 {#clean-cache-when-mount-pod-exits}

详见[「缓存相关章节」](./cache.md#mount-pod-clean-cache)。

## 延迟删除 Mount Pod {#delayed-mount-pod-deletion}

:::note 注意
此特性需使用 0.13.0 及以上版本的 JuiceFS CSI 驱动
:::

Mount Pod 是支持复用的，由 JuiceFS CSI Node Service 以引用计数的方式进行管理：当没有任何应用 Pod 在使用该 Mount Pod 创建出来的 PV 时，JuiceFS CSI Node Service 会删除 Mount Pod。

但在 Kubernetes 不少场景中，容器转瞬即逝，调度极其频繁，这时可以为 Mount Pod 配置延迟删除，这样一来，如果短时间内还有新应用 Pod 使用相同的 Volume，Mount Pod 能够被继续复用，免除了反复销毁创建的开销。

控制延迟删除 Mount Pod 的配置项形如 `juicefs/mount-delete-delay: 1m`，单位支持 `ns`（纳秒）、`us`（微秒）、`ms`（毫秒）、`s`（秒）、`m`（分钟）、`h`（小时）。

配置好延迟删除后，当引用计数归零，Mount Pod 会被打上 `juicefs-delete-at` 的注解（annotation），标记好删除时间，到达设置的删除时间后，Mount Pod 才会被删除。但如果在此期间有新的应用 Pod 欲使用该 PV，注解 `juicefs-delete-at` 就被清空，Mount Pod 的删除计划随之取消，得以继续复用。

静态和动态配置方式中，需要在不同的地方填写该配置。

### 静态配置

需要在 PV 定义中配置延迟删除的时长，修改 `volumeAttributes` 字段，添加 `juicefs/mount-delete-delay`，设置为需要的时长：

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
      juicefs/mount-delete-delay: 1m
```

### 动态配置

需要在 StorageClass 定义中配置延迟删除的时长，修改 `parameters` 字段，添加 `juicefs/mount-delete-delay`，设置为需要的时长：

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
  juicefs/mount-delete-delay: 1m
```

## PV 回收策略 {#reclaim-policy}

[回收策略](https://kubernetes.io/zh-cn/docs/concepts/storage/persistent-volumes/#reclaiming)决定了 PVC 或 PV 被删除后，存储里的数据何去何从。常用的回收策略是保留（Retain）和删除（Delete），保留回收策略需要用户自己回收资源（包括 PV、JuiceFS 上的数据），而删除回收策略则意味着 PV 及 JuiceFS 上的数据会随着 PVC 删除而直接清理掉。

### 静态配置

静态配置中，只支持 Retain 回收策略：

```yaml {13}
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
```

### 动态配置

动态配置默认的回收策略为 Delete，可以在 StorageClass 定义中修改为 Retain：

```yaml {6}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
reclaimPolicy: Retain
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
```

## 仅在某些节点上运行 CSI Node Service {#csi-node-node-selector}

JuiceFS CSI 驱动的组件分为 CSI Controller、CSI Node Service 及 Mount Pod，详细可参考 [JuiceFS CSI 驱动架构](../introduction.md#architecture)。

默认情况下，CSI Node Service（DaemonSet）会在所有 Kubernetes 节点上启动，如果希望进一步减少资源占用，则可按照本节介绍的方式，让 CSI Node 仅在实际需要使用 JuiceFS 的节点上启动。

### 配置节点标签 {#add-node-label}

先为需要使用 JuiceFS 的节点加上相应的标签，比方说为执行模型训练的节点打上标签：

```shell
# 根据实际情况为 Kubernetes 节点加上标签
kubectl label node [node-1] [node-2] app=model-training
```

### 修改 JuiceFS CSI 驱动安装配置 {#modify-juicefs-csi-driver-installation-configuration}

除了 `nodeSelector`，Kubernetes 还提供更多方式控制容器调度，参考[将 Pod 指派给节点](https://kubernetes.io/zh-cn/docs/concepts/scheduling-eviction/assign-pod-node)。

:::warning
如果使用 nodeSelector 将 CSI-node 驱动部署到选定的节点，那么使用了 JuiceFS PV 的应用，也需要加上相同的 nodeSelector，才能保证分配到能够提供文件系统服务的节点上。
:::

#### 通过 Helm 安装

在 `values.yaml` 中添加如下配置：

```yaml title="values.yaml"
node:
  nodeSelector:
    # 根据实际情况修改节点标签
    app: model-training
```

安装 JuiceFS CSI 驱动：

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

#### 通过 kubectl 安装

在 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml) 中新增 `nodeSelector` 配置：

```yaml {11-13} title="k8s.yaml"
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: juicefs-csi-node
  namespace: kube-system
  ...
spec:
  ...
  template:
    spec:
      nodeSelector:
        # 根据实际情况修改节点标签
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

安装 JuiceFS CSI 驱动：

```shell
kubectl apply -f k8s.yaml
```

## 卸载 JuiceFS CSI Controller {#uninstall-juicefs-csi-controller}

CSI Controller 的作用仅仅是[动态配置](./pv.md#dynamic-provisioning)下的初始化，因此，如果你完全不需要以动态配置方式使用 CSI 驱动，可以卸载 CSI Controller，仅留下 CSI Node Service：

```shell
kubectl -n kube-system delete sts juicefs-csi-controller
```

如果你使用 Helm 管理 CSI 驱动：

```yaml title="values.yaml"
controller:
  enabled: false
```

考虑到 CSI Controller 消耗资源并不多，并不建议卸载，该实践仅供参考。
