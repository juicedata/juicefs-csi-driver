---
title: 升级 JuiceFS CSI 驱动
slug: /upgrade-csi-driver
sidebar_position: 2
---

我们推荐你及时跟进主要版本的升级，不要在大小版本上落后太久（patch 版本如果没有特殊需要，不必频繁升级）。如果你不清楚当前所使用的版本，只需要查询 CSI 驱动组件所使用的镜像 tag，就能确认当前版本。可以用下方一行命令快速检查：

```shell
kubectl get pods -l app=juicefs-csi-node -ojsonpath='{range .items[*]}{..spec..image}{"\n"}{end}' --all-namespaces | head -n 1 | grep -oP 'juicefs-csi-driver:\S+'
```

阅读 JuiceFS CSI 驱动的[发布说明](https://github.com/juicedata/juicefs-csi-driver/releases)以了解是否需要升级。需要注意的是：

* 升级 CSI 驱动，会伴随着 JuiceFS 客户端（也就是 mount 镜像）的升级，不过不用担心，正在运行的 Mount Pod 并不受影响，应用滚升或重建以后才会生效
* 如果已经通过[其他手段](../guide/custom-image.md)显式指定了 mount 镜像，那么升级 CSI 驱动就不再“顺便”升级 JuiceFS 客户端版本了
* 若需要单独升级 JuiceFS 客户端，而不升级 CSI 驱动，参考[「升级 JuiceFS 客户端」](./upgrade-juicefs-client.md)

## 升级 CSI 驱动（容器挂载模式） {#upgrade}

v0.10.0 开始，JuiceFS 客户端与 CSI 驱动进行了分离，升级 CSI 驱动将不会影响已存在的 PV。因此升级过程大幅简化，同时不影响已有服务。

但这也意味着，升级 CSI 驱动后，**应用并不会自动地享受到新版 JuiceFS 客户端**。你还需要重新创建应用容器，这样一来，CSI Node 会使用新版 Mount Pod 容器镜像创建挂载点，让应用使用新版 JuiceFS 客户端。

特别地，如果你[修改了 Mount Pod 容器镜像](../guide/custom-image.md#overwrite-mount-pod-image)，那么升级 CSI 驱动就完全不影响 JuiceFS 客户端版本了，你需要按照[文档](../guide/custom-image.md#overwrite-mount-pod-image)，继续自行管理 Mount Pod 容器镜像。

### 通过 Helm 升级 {#helm-upgrade}

仔细按照下方列表进行确认和操作。

* 找到该集群安装时使用的 Helm 配置文件，比如 `values-mycluster.yaml`，在文件名中体现出集群名是为了方便管理。如果该文件没有进行源码管理、已经丢失，那么可以尝试用下方命令恢复：

  ```shell
  # 根据实际情况修改命名空间和 Release 名称
  helm get values juicefs-csi-driver -n kube-system > values-mycluster.yaml
  ```

* 打开 `values-mycluster.yaml`，将其与 CSI 驱动默认的 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml) 进行比对，精简删去重复内容，尤其不应包含镜像 Tag。如果你通过复制粘贴的方式生成最初的 `values-mycluster.yaml`，那么会包含如下配置：

  ```yaml
  image:
    repository: juicedata/juicefs-csi-driver
    tag: "v0.28.1"
  ```

  按照上述示范，由于 `tag` 被固定在 `v0.28.1`，就算运行了 `helm repo update`，再次安装的时候，也不会进行升级。因此，我们**不推荐用复制的方式生成 Helm 集群配置。**如果确实需要修改某些字段，比如容器镜像的仓库地址，应该删去其他无关字段：

  ```yaml
  image:
    repository: registry.mycompany.com/juicefs-csi-driver
  ```

  如此一来，在 `values-mycluster.yaml` 中仅仅覆盖了镜像仓库，不影响版本号，那么在升级操作的时候，Helm 仍然会以默认 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml) 中的内容为准，如此方可顺利升级。

* 如果你的工作电脑可以顺利访问 `https://juicedata.github.io/charts/`，那么并不需要将 Helm Chart 离线下载到本地，直接运行下方命令就能更新升级：

  ```shell
  helm repo update
  helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
  ```

  如果你的环境属于这种情况，用上方的示范命令升级完毕以后，就可以检查组件状态、核实升级结果，不需要再继续阅读后续步骤。

* 如果网络条件不佳，无法访问我们的 Helm 仓库，则可以通过下列操作，将安装资源全部离线下载、保存在本地：

  ```shell
  # 在另一个可以顺利访问 Helm 仓库的环境提前下载好 Helm Chart
  helm repo add juicefs https://juicedata.github.io/charts/
  helm repo update
  helm fetch --untar juicefs/juicefs-csi-driver
  ```

  下载好 Helm Chart 后，将集群配置文件移入该目录，并创建 Git 仓库，将所有资源纳入源码管理。最终的目录结构类似下方示范：

  ```shell
  $ tree -L1
  .
  ├── Chart.yaml
  ├── README.md
  ├── templates
  ├── values-mycluster.yaml
  └── values.yaml
  ```

  在未来需要升级 CSI 驱动的时候，也需要再次下载 Chart，对本地资源进行删除和替换（注意不能直接 `cp -r` 覆盖，必须采用删除替换的方式，避免新版本可能淘汰了特定资源）。

  :::warning
  在删除本地旧版资源之前，务必确认是否存在定制化改动，比如在 `templates` 目录下增删或修改相关资源，来对集群进行特殊适配。如果有此类修改，也需要将定制化的部分重新施加在新版 Chart 中。

  由于此类操作的复杂性，我们不推荐任何针对 Chart 资源的定制，请与 Juicedata 工程师讨论协商，贡献代码来支持特殊的定制需求。
  :::

  ```shell
  # 将新版 Chart 下载到临时目录，方便后续替换操作
  helm repo update
  helm fetch --untar juicefs/juicefs-csi-driver --untardir=temp

  # 删除旧版相关资源，替换为新版 Chart
  rm -rf juicefs-csi-driver/templates
  mv temp/juicefs-csi-driver/* juicefs-csi-driver

  # 进入 Chart 目录，用本地资源执行升级
  helm upgrade --install juicefs-csi-driver . -n kube-system -f ./values-mycluster.yaml
  ```

### 通过 kubectl 升级 {#kubectl-upgrade}

如果你使用 kubectl 的安装方式，我们不建议你对 `k8s.yaml` 做任何定制修改，这些修改将会在升级时带来沉重的负担：你需要对新旧版本的 `k8s.yaml` 进行 diff 比对，辨认出哪些改动是需要保留的，哪些改动则是新版 CSI 驱动所需要的、应当覆盖。随着你的修改增多，这些工作会变得十分艰难。

因此如果你的生产集群仍在使用 kubectl 安装方式，务必尽快切换成 Helm 安装方式（参考下一小节了解如何迁移到 Helm 安装方式）。考虑到默认的容器挂载模式是一个[解耦架构](../introduction.md#architecture)，卸载 CSI 驱动并不影响正在运行的服务，你可以放心地卸载、[使用 Helm 重新安装 CSI 驱动](../getting_started.md#helm)，享受更便利的升级流程。

当然了，如果你并未对 CSI 驱动配置做任何改动，那么直接下载最新的 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml)，然后用下边的命令进行覆盖安装即可。

```shell
kubectl apply -f ./k8s.yaml
```

但如果你的团队自行维护 `k8s.yaml`，已经对其中的配置做了变更，那就需要对新老版本的 `k8s.yaml` 进行内容比对，将新版引入的变动追加进来，然后再进行覆盖安装：

```shell
curl https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml > k8s-new.yaml

# 梳理配置前，对老版本安装文件进行备份
cp k8s.yaml k8s.yaml.bak

# 对比新老版本的内容差异，在保留配置变更的基础上，将新版引入的变动进行追加
# 比方说，新版的 CSI 驱动组件镜像往往会更新，例如 image: juicedata/juicefs-csi-driver:v0.21.0
vimdiff k8s.yaml k8s-new.yaml

# 配置梳理完毕，进行覆盖安装
kubectl apply -f ./k8s.yaml
```

如果安装过程中报错提示 CSI 驱动无法更新，报错提示资源无法变更：

```
csidrivers.storage.k8s.io "csi.juicefs.com" was not valid:
* spec.storageCapacity: Invalid value: true: field is immutable
```

这往往表示新版 `k8s.yaml` 引入了 CSI 驱动的资源定义更新（比方说 [v0.21.0](https://github.com/juicedata/juicefs-csi-driver/releases/tag/v0.21.0) 引入了 `podInfoOnMount: true`），你需要手动删除相关资源，才能重装：

```shell
kubectl delete csidriver csi.juicefs.com

# 覆盖安装
kubectl apply -f ./k8s.yaml
```

复杂的梳理流程、以及安装时的异常处理，对生产环境的维护极不友好，因此在生产环境务必使用 Helm 的安装方式。

### 迁移到 Helm 安装方式 {#migrate-to-helm}

Helm 安装需要先填写 `values.yaml`——你对 `k8s.yaml` 做过的所有改动，只要属于正常使用范畴，都能在 `values.yaml` 中找到对应的配置字段，你需要做的就是梳理当前配置，将他们填写到 `values.yaml`。当然了，如果你并没有对 `k8s.yaml` 进行定制（也没有对线上环境直接修改过配置），那么迁移会非常简单，直接跳过梳理步骤，直接按照下方的指示卸载重装即可。

#### 梳理配置、填写 `values.yaml` {#sort-out-the-configuration-and-fill-in-values-yaml}

开始之前，你需要确定当前所使用的 CSI 驱动版本，可以直接使用本文开头的方法进行判断。下方以 v0.18.0 升级到 v0.21.0 为例，讲解如何逐行梳理配置，填写 `values.yaml`。

1. 用浏览器访问 GitHub，打开两个版本的 diff。这个过程需要手动输入链接，注意链接末尾的版本号，例如 [https://github.com/juicedata/juicefs-csi-driver/compare/v0.18.0..v0.21.0](https://github.com/juicedata/juicefs-csi-driver/compare/v0.18.0..v0.21.0)，在文件列表中找到 `k8s.yaml`。在页面中会显示版本更新所引入的所有 `k8s.yaml` 变动。保留这个页面，稍后梳理配置的时候，如果不确定哪些变动是你的集群定制配置，哪些是升级所带来的修改，都可以参照这个页面来判断；
1. 找到当前线上集群安装时所使用的 `k8s.yaml`，将其拷贝重命名为 `k8s-online.yaml`，本文档后续也用该名称来指代当前线上的安装文件。一定要注意，该文件必须能准确反映出「当前线上的配置」，如果你的团队临时修改过线上配置（比如使用 `kubectl edit` 临时添加环境变量、修改镜像），你需要对这些改动加以确认，并追加到 `k8s-online.yaml`；
1. 将新版（此处链接以 v0.21.0 为例）CSI 驱动的 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/94d4f95a5d0f15a7a430ea31257d725306e90ca4/deploy/k8s.yaml) 下载到本地，与线上配置进行对比，可以直接运行 `vimdiff k8s.yaml k8s-online.yaml`；
1. 逐行对比配置文件，确定每一处配置修改是升级带来的，还是你的团队进行的定制。判断这些定制是否需要保留，然后填写到 `values.yaml`。如果不确定如何填写，仔细阅读 `values.yaml` 中的注释文档，通常就能找到线索。

我们对撰写 `values.yaml` 有如下建议：

如果 `k8s-online.yaml` 中[覆盖了默认的 Mount Pod 镜像](../guide/custom-image.md)（可以通过 `JUICEFS_EE_MOUNT_IMAGE` 环境变量，或者 StorageClass 的 `juicefs/mount-image` 字段），并且指定了一个更老版本的 Mount Pod 镜像，我们鼓励你丢弃该配置，让集群随着 CSI 驱动升级，启用新版 Mount Pod 镜像，相当于伴随着 CSI 驱动升级，JuiceFS 客户端一并升级。

动态配置需要[创建 StorageClass](../guide/pv.md#create-storage-class)，而在 Helm Values 中，StorageClass 和[文件系统认证信息](../guide/pv.md#volume-credentials)是捆绑一起管理的，为了避免将敏感信息留在 `values.yaml`，我们一般建议手动管理文件系统认证信息和 StorageClass，然后将 `values.yaml` 中的 StorageClass 禁用：

```yaml title="values.yaml"
storageClasses:
- enabled: false
```

#### 卸载重装 {#uninstall-and-reinstall}

如果你使用默认的容器挂载，或者 Sidecar 模式，那么卸载 CSI 驱动是不影响当前服务的（期间新的 PV 无法创建、挂载）。只有[进程挂载模式](../introduction.md#by-process)会因为卸载而中断服务。如果你没有在使用该模式，迁移过程对正在运行的 PV 完全没有影响，可以放心执行。

如果你的环境是离线集群，无法直接从外网拉取镜像，那么也需要提前[搬运镜像](./offline.md)。

提前准备好需要运行的运维命令，例如：

```shell
# 卸载当前 CSI 驱动
kubectl delete -f k8s-online.yaml

# 用 Helm 重装，不同集群的配置可以用不同的 values.yaml 文件来进行管理，比如 values-dev.yaml, values-prod.yaml。
# CSI 驱动对安装命名空间没有特殊要求，你可以按需修改，比如 jfs-system。
helm upgrade --install juicefs-csi-driver . -f values.yaml -n kube-system
```

运行这些命令，重装以后，立刻观察 CSI 驱动各组件的启动情况：

```shell
kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
```

等待所有组件启动完毕，然后简单创建应用 Pod 进行验证，可以参考[我们的示范](../guide/pv.md#static-provisioning)。

## 升级 CSI 驱动（进程挂载模式） {#mount-by-process-upgrade}

所谓[进程挂载模式](../introduction.md#by-process)，就是 JuiceFS 客户端运行在 CSI Node Service Pod 内，此模式下升级，将不可避免地需要中断 JuiceFS 客户端挂载，你需要根据实际情况，参考以下方案进行升级。

v0.10 之前的 JuiceFS CSI 驱动，仅支持进程挂载模式，因此如果你还在使用 v0.9.x 或更早的版本，也需要参考本节内容进行升级。

### 方案一：滚动升级

如果使用 JuiceFS 的应用不可被中断，可以采用此方案。

由于是滚动升级，因此目标版本如果引入了新资源，你需要提前梳理出来，并单独安装。本小节中包含的各种 YAML 内容，仅适用于自 v0.9 升级至 v0.10 的情况。视目标版本不同，你可能需要比较不同版本的 `k8s.yaml` 文件，提取出有差异的 Kubernetes 资源，并手动安装。

#### 1. 创建新增资源

将以下内容保存成 `csi_new_resource.yaml`，然后执行 `kubectl apply -f csi_new_resource.yaml`。

```yaml title="csi_new_resource.yaml"
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: juicefs-csi-external-node-service-role
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - create
      - update
      - delete
      - patch
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
  name: juicefs-csi-node-service-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: juicefs-csi-external-node-service-role
subjects:
  - kind: ServiceAccount
    name: juicefs-csi-node-sa
    namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: juicefs-csi-node-sa
  namespace: kube-system
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
```

#### 2. 将 CSI Node Service 的升级策略改成 `OnDelete`

```shell
kubectl -n kube-system patch ds <ds_name> -p '{"spec": {"updateStrategy": {"type": "OnDelete"}}}'
```

#### 3. 升级 CSI Node Service

将以下内容保存成 `ds_patch.yaml`，然后执行 `kubectl -n kube-system patch ds <ds_name> --patch "$(cat ds_patch.yaml)"`。

```yaml title="ds_patch.yaml"
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:v0.10.6
          args:
            - --endpoint=$(CSI_ENDPOINT)
            - --logtostderr
            - --nodeid=$(NODE_ID)
            - --v=5
            - --enable-manager=true
          env:
            - name: CSI_ENDPOINT
              value: unix:/csi/csi.sock
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
          volumeMounts:
            - mountPath: /jfs
              mountPropagation: Bidirectional
              name: jfs-dir
            - mountPath: /root/.juicefs
              mountPropagation: Bidirectional
              name: jfs-root-dir
      serviceAccount: juicefs-csi-node-sa
      volumes:
        - hostPath:
            path: /var/lib/juicefs/volume
            type: DirectoryOrCreate
          name: jfs-dir
        - hostPath:
            path: /var/lib/juicefs/config
            type: DirectoryOrCreate
          name: jfs-root-dir
```

#### 4. 执行滚动升级

在每台节点上执行以下操作：

1. 删除当前节点上的 CSI Node Service Pod：

   ```shell
   kubectl -n kube-system delete po juicefs-csi-node-df7m7
   ```

2. 确认新的 CSI Node Service Pod 已经 ready：

   ```shell
   $ kubectl -n kube-system get po -o wide -l app.kubernetes.io/name=juicefs-csi-driver | grep kube-node-2
   juicefs-csi-node-6bgc6     3/3     Running   0          60s   172.16.11.11   kube-node-2   <none>           <none>
   ```

3. 在当前节点上，删除使用 JuiceFS 的业务 Pod 并重新创建。

4. 确认使用 JuiceFS 的业务 Pod 已经 ready，并检查是否正常工作。

#### 5. 升级 CSI Controller 及其 role

将以下内容保存成 `sts_patch.yaml`，然后执行 `kubectl -n kube-system patch sts <sts_name> --patch "$(cat sts_patch.yaml)"`。

```yaml title="sts_patch.yaml"
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:v0.10.6
          env:
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
          volumeMounts:
            - mountPath: /jfs
              mountPropagation: Bidirectional
              name: jfs-dir
            - mountPath: /root/.juicefs
              mountPropagation: Bidirectional
              name: jfs-root-dir
      volumes:
        - hostPath:
            path: /var/lib/juicefs/volume
            type: DirectoryOrCreate
          name: jfs-dir
        - hostPath:
            path: /var/lib/juicefs/config
            type: DirectoryOrCreate
          name: jfs-root-dir
```

将以下内容保存成 `clusterrole_patch.yaml`，然后执行 `kubectl patch clusterrole <role_name> --patch "$(cat clusterrole_patch.yaml)"`。

```yaml title="clusterrole_patch.yaml"
rules:
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
    verbs:
      - get
      - list
      - watch
      - create
      - delete
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - storage.k8s.io
    resources:
      - storageclasses
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - csinodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
```

### 方案二：替换升级

如果你能接受服务中断，这将是更为简单的升级手段。

此法需要卸载所有正在使用 JuiceFS PV 的应用，按照上方的[正常升级流程](#upgrade)操作，然后重新创建受影响的应用即可。
