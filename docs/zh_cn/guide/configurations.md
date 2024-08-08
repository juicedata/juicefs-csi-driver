---
title: 高级功能与配置
sidebar_position: 2
---

CSI 驱动的各种高级功能，以及使用 JuiceFS PV 的各项配置、CSI 驱动自身的配置，都在本章详述。

## ConfigMap 配置 {#configmap}

从 v0.24 开始，CSI 驱动支持在名为 `juicefs-csi-driver-config` 的 ConfigMap 中书写配置，支持多种多样的配置项，既可以用来配置 mount pod 或 sidecar，也包含 CSI 驱动自身的配置，并且支持动态更新：修改 mount pod 配置时不需要重建 PV，修改 CSI 自身配置时，也不需要重启 CSI Node 或者 Controller。

由于 ConfigMap 功能强大、更加灵活，他将会或已经取代从前在 CSI 驱动中各种修改配置的方式，例如下方标有「不推荐」的小节，均为旧版中灵活性欠佳的实践，请及时弃用。**简而言之，如果一项配置已经在 ConfigMap 中得到支持，请优先在 ConfigMap 中对其进行配置，无需再使用旧版本中的实践。**

:::info 更新时效
修改 ConfigMap 以后，相关改动并不会立刻生效，这是由于挂载进容器的 ConfigMap 并非实时更新，而是定期同步（详见 [Kubernetes 官方文档](https://kubernetes.io/docs/concepts/configuration/configmap/#mounted-configmaps-are-updated-automatically)）。

如果希望立即生效，可以给 CSI 组件 pods 临时添加 annotation 来触发更新：

```shell
kubectl -n kube-system annotate pods -l app.kubernetes.io/name=juicefs-csi-driver useless-annotation=true
```

:::

ConfigMap 中支持的所有配置项，都可以在[这里](https://github.com/juicedata/juicefs-csi-driver/blob/master/juicefs-csi-driver-config.example.yaml)找到示范，并且在本文档相关小节中进行更详细介绍。

### 定制 Mount Pod 和 Sidecar 容器 {#customize-mount-pod}

Mount Pod 并非由用户直接创建，而是 CSI Node 负责生成。Sidecar 容器则是 [webhook](#webhook) 负责注入，如果用户希望定制 mount pod 或者 sidecar 容器，需要通过下列介绍的方法进行。

在 [ConfigMap](#configmap) 中，`mountPodPatch` 这个字段专门用于定制 mount pod 或者 sidecar 容器，可供定制的部分均已在示范中列出。使用前需要注意：

* **修改后并不会立即生效**，Kubernetes 会定期同步 ConfigMap 的挂载。详见 [ConfigMap 的更新时效](#configmap)
* 对于 sidecar 场景，相关的字段只要是合法的 sidecar 容器配置，那么对于 sidecar 容器同样生效：比如 `resources` 是 mount pod 和 sidecar 容器都具备的配置，因此对两种场景都生效；`custom-labels` 的作用是为 pod 添加自定义标签，而「标签」是 pod 独有的属性，container 是没有标签的，因此 `custom-labels` 就只对 mount pod 生效，sidecar 场景则会忽略该配置

```yaml title="values-mycluster.yaml"
globalConfig:
  # 支持模板变量，比如 ${MOUNT_POINT}、${SUB_PATH}、${VOLUME_ID}
  mountPodPatch:
    # 未定义 pvcSelector，则为全局配置
    - lifecycle:
        preStop:
          exec:
            command:
            - sh
            - -c
            - +e
            - umount -l ${MOUNT_POINT}; rmdir ${MOUNT_POINT}; exit 0

    # 如果多个 pvcSelector 匹配的是相同的 PVC，则后定义的配置会覆盖更早定义的配置
    - pvcSelector:
        matchLabels:
          mylabel1: "value1"
      # 启用 host network
      hostNetwork: true

    - pvcSelector:
        matchLabels:
          mylabel2: "value2"
      # 增加 labels
      labels:
        custom-labels: "mylabels"

    - pvcSelector:
        matchLabels:
          ...
      # 修改资源定义
      resources:
        requests:
          cpu: 100m
          memory: 512Mi

    - pvcSelector:
        matchLabels:
          ...
      readinessProbe:
        exec:
          command:
          - stat
          - ${MOUNT_POINT}/${SUB_PATH}
        failureThreshold: 3
        initialDelaySeconds: 10
        periodSeconds: 5
        successThreshold: 1

    - pvcSelector:
        matchLabels:
          ...
      # 目前暂不推荐使用 liveness probe，请优先使用 readiness probe
      # JuiceFS 客户端自身也会进行检活和重启，因此避免额外设置 liveness probe，从外部重启
      livenessProbe:
        exec:
          command:
          - stat
          - ${MOUNT_POINT}/${SUB_PATH}
        failureThreshold: 3
        initialDelaySeconds: 10
        periodSeconds: 5
        successThreshold: 1

    - pvcSelector:
        matchLabels:
          ...
      annotations:
        # 延迟删除
        juicefs-delete-delay: 5m
        # 退出时清理 cache
        juicefs-clean-cache: "true"

      # 为 mount pod 注入 env
    - pvcSelector:
        matchLabels:
          ...
      env:
      - name: DEMO_GREETING
        value: "Hello from the environment"
      - name: DEMO_FAREWELL
        value: "Such a sweet sorrow"

      # 挂载 volumes 到 mount pod
    - pvcSelector:
        matchLabels:
          ...
      volumeDevices:
        - name: block-devices
          devicePath: /dev/sda1
      volumes:
        - name: block-devices
          persistentVolumeClaim:
            claimName: block-pv
        
```

### 通过继承 CSI Node 配置进行定制（不推荐） {#inherit-from-csi-node}

:::tip
从 v0.24 开始，CSI 驱动支持在 [ConfigMap](#configmap) 中定制 mount pod 和 sidecar 容器，本小节所介绍的方式已经不再推荐使用。
:::

Mount Pod 自身的资源定义（Kubernetes manifests，也就是 Pod YAML）大部分继承自 CSI Node，比方说如果希望给 mount pod 启用 hostNetwork，可以先为 CSI Node 启用 hostNetwork：

```yaml title="values-mycluster.yaml"
node:
  hostNetwork: true
```

让配置生效，后续生成的 mount pod 就会继承 hostNetwork 配置了。

之所以说「大部分」配置继承自 CSI Node，是因为 labels、annotations 等字段包含组件特定内容，无法简单继承。因此为这部分配置提供单独的定制手段，继续阅读下一小节了解。

### 通过 Annotation 进行定制（不推荐） {#others}

:::tip
从 v0.24 开始，CSI 驱动支持在 [ConfigMap](#configmap) 中定制 mount pod 和 sidecar 容器，本小节所介绍的方式已经不再推荐使用。
:::

部分无法用继承方式定制的配置项，我们提供了额外的手段来单独定制。具体而言，就是下方代码块列出的字段，既可以将他们配置在 storageClass 的 parameters 参数中（动态配置），也可放在 PVC 的 annotations 中（静态配置）。

```yaml
juicefs/mount-cpu-limit: ""
juicefs/mount-memory-limit: ""
juicefs/mount-cpu-request: ""
juicefs/mount-memory-request: ""

juicefs/mount-labels: ""
juicefs/mount-annotations: ""
juicefs/mount-service-account: ""
juicefs/mount-image: ""
juicefs/mount-delete-delay: ""

# 退出时清理缓存
juicefs/clean-cache: ""
juicefs/mount-cache-pvc: ""
juicefs/mount-cache-emptydir: ""
juicefs/mount-cache-inline-volume: ""

# 在 mount pod 中挂载宿主机文件或目录
# 容器内路径将会等同宿主机路径，无法定制
juicefs/host-path: "/data/file.txt"
```

## 格式化参数/认证参数 {#format-options}

「格式化参数/认证参数」是 `juicefs [format|auth]` 命令所接受的参数，其中：

* 社区版的 [`format`](https://juicefs.com/docs/zh/community/command_reference/#format) 是用于创建新文件系统的命令。社区版需要用户自行用客户端 `format` 命令创建文件系统，然后才能挂载；
* 企业版的 [`auth`](https://juicefs.com/docs/zh/cloud/reference/command_reference/#auth) 命令是负责向控制台发起认证、获取客户端配置文件。他在使用流程中的作用和 `format` 有些相似，这涉及到两个版本在使用上的区别：和社区版需要先格式化创建文件系统不同，企业版需要在 Web 控制台创建文件系统，客户端并不具备创建文件系统的能力，但是挂载时需要向控制台发起认证，这也就是 `auth` 命令的功能。

考虑到这两个命令的相似性，不论你使用社区版还是企业版，对应的命令运行参数都填入 `format-options`，示范如下。

:::tip
修改 `format-options` 并不影响已有的挂载客户端，即便重启 Mount Pod 也不会生效，需要滚升/重启应用 Pod，或者重建 PVC，方能生效。
:::

社区版：

```yaml {13}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <JUICEFS_NAME>
  metaurl: <META_URL>
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  format-options: trash-days=1
```

企业版：

```yaml {13}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${ACCESS_KEY}
  secret-key: ${SECRET_KEY}
  format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

## 挂载参数 {#mount-options}

「挂载参数」，也就是 `juicefs mount` 命令所接受的参数，可以用于调整挂载配置。你需要通过 `mountOptions` 字段填写需要调整的挂载配置，在静态配置和动态配置下填写的位置不同，见下方示范。

### 静态配置 {#static-mount-options}

注意，如果是修改已有 PV 的挂载配置，修改后需要重建应用 Pod，才会触发重新创建 Mount Pod，令变动生效。

```yaml {8-9}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  mountOptions:
    - cache-size=204800
  ...
```

### 动态配置 {#dynamic-mount-options}

在 `StorageClass` 定义中调整挂载参数。如果需要为不同应用使用不同挂载参数，则需要创建多个 `StorageClass`，单独添加所需参数。

注意，StorageClass 仅仅是动态配置下用于创建 PV 的「模板」，也正因此，**在 StorageClass 中修改挂载配置，不影响已经创建的 PV。**如果你需要调整挂载配置，需要删除 PVC 重建，或者直接[在 PV 级别调整挂载配置](#static-mount-options)。

```yaml {6-7}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
mountOptions:
  - cache-size=204800
parameters:
  ...
```

### 参数详解

JuiceFS 社区版与云服务的挂载参数有所区别，请参考文档：

- [社区版](https://juicefs.com/docs/zh/community/command_reference#mount)
- [云服务](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)

PV/StorageClass 中的 `mountOptions` 同时支持 JuiceFS 本身的挂载参数和 FUSE 相关选项。但要注意，虽然 FUSE 参数在命令行使用时会用 `-o` 传入，但在 `mountOptions` 中需要省略 `-o`，直接在列表中追加参数即可。以下方挂载命令为例：

```shell
juicefs mount ... --cache-size=204800 -o writeback_cache,debug
```

翻译成 CSI 中的 `mountOptions`，格式如下：

```yaml
mountOptions:
  # JuiceFS mount options
  - cache-size=204800
  # 额外的 FUSE 相关选项
  - writeback_cache
  - debug
```

## 应用间共享存储 {#share-directory}

如果你在 JuiceFS 文件系统已经存储了大量数据，希望挂载进容器使用，或者希望让多个应用共享同一个 JuiceFS 目录，有以下做法：

### 静态配置

#### 挂载子目录 {#mount-subdirectory}

挂载子目录有两种方式，一种是通过 `--subdir` 挂载选项，另一种是通过 [`volumeMounts.subPath` 属性](https://kubernetes.io/zh-cn/docs/concepts/storage/volumes/#using-subpath)，下面分别介绍。

- **使用 `--subdir` 挂载选项**

  修改[「挂载参数」](#mount-options)，用 `subdir` 参数挂载子目录。如果子目录尚不存在，CSI Controller 会在挂载前自动创建。

  ```yaml {8-9}
  apiVersion: v1
  kind: PersistentVolume
  metadata:
    name: juicefs-pv
    labels:
      juicefs-name: ten-pb-fs
  spec:
    mountOptions:
      - subdir=/my/sub/dir
    ...
  ```

- **使用 `volumeMounts.subPath` 属性**

  ```yaml {11-12}
  apiVersion: v1
  kind: Pod
  metadata:
    name: juicefs-app
    namespace: default
  spec:
    containers:
      - volumeMounts:
          - name: data
            mountPath: /data
            # 注意 subPath 只能用相对路径，不能用绝对路径。
            subPath: my/sub/dir
        ...
    volumes:
      - name: data
        persistentVolumeClaim:
          claimName: juicefs-pvc
  ```

  如果在同一台宿主机上可能会运行多个应用 Pod，并且这些应用 Pod 需要挂载同一个文件系统的不同子目录，那么建议使用 `volumeMounts.subPath` 属性来挂载，因为这种方式只会创建 1 个 Mount Pod，可以大大节省宿主机的资源。

#### 跨命名空间（namespace）共享同一个文件系统 {#sharing-same-file-system-across-namespaces}

如果想要在不同命名空间中共享同一个文件系统，只需要让不同 PV 使用相同的文件系统认证信息（Secret）即可：

```yaml {10-12,24-26}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: mypv1
  labels:
    pv-name: mypv1
spec:
  csi:
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
  ...
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: mypv2
  labels:
    pv-name: mypv2
spec:
  csi:
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
  ...
```

### 动态配置

严格来说，由于动态配置本身的性质，并不支持挂载 JuiceFS 中已经存在的目录。但动态配置下可以[调整子目录命名模板](#using-path-pattern)，让生成的子目录名称对齐 JuiceFS 中已有的目录，来达到同样的效果。

## Webhook 相关功能 {#webhook}

CSI 驱动的 Controller 组件可以通过增加相关参数，令其兼具 Webhook 的功能。Webhook 启动以后将会额外支持更多高级功能，在本小节分别介绍。

### Mutating webhook

如果启用了 [sidecar 模式](../introduction.md#sidecar)，那么 Controller 同时会作为 mutating webhook 运行，此时 Controller 进程的启动参数里会包含 [`--webhook`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/templates/controller.yaml#L76)，你可以通过这个参数判断目前是否启用了该特性。

顾名思义，mutating 会对资源进行变更，也就是指定命名空间下的所有 pod 创建，都会经过这个 webhook，如果检测到他使用了 JuiceFS PV，便会向其中注入 sidecar 容器。

### Validating webhook

:::tip
目前该功能仅对 JuiceFS 企业版生效。
:::

自 v0.23.6 起，CSI 驱动可选地提供 Secret 校验功能，帮助用户正确填写[文件系统认证信息](./pv.md#volume-credentials)。如果填错了[文件系统令牌](https://juicefs.com/docs/zh/cloud/acl#client-token)，那么创建 Secret 将会失败，并提示用户错误信息。

如果要开启 validating webhook，需要在 Helm values 中调整配置（参考默认的 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml#L342)）：

```yaml name="values-mycluster.yaml"
validatingWebhook:
  enabled: true
```

## 高级 PV 初始化功能 {#provisioner}

CSI 驱动提供两种方式进行 PV 初始化：

* 使用标准的 [Kubernetes CSI provisioner](https://github.com/kubernetes-csi/external-provisioner)，在旧版 CSI 驱动默认按照这种方式运行，因此 juicefs-csi-controller 的 Pod 内会包含 provisioner，共 4 个容器
* （推荐）不再使用标准的 CSI provisioner，而是将 controller 作为 provisioner。从 v0.23.4 开始，如果通过我们推荐的 Helm 方式安装 CSI 驱动，那么该功能会默认启用，此时 juicefs-csi-controller 的 Pod 只包含 3 个容器，没有 provisioner

之所以更推荐使用我们自带的 provisioner，是因为他给一系列高级自定义功能提供了可能，包括：

* [配置更加易读的 PV 目录名称](#using-path-pattern)，不再面对形如 `pvc-4f2e2384-61f2-4045-b4df-fbdabe496c1b` 的随机 PV 子目录，而是自定义成更易读的格式，比如 `default-juicefs-myapp`
* 模板化方式配置挂载参数，实现类似[根据网络区域设置缓存组](#regional-cache-group)的高级功能

在「动态配置」方式下，Provisoner 组件会根据 StorageClass 中的配置动态地创建的 PV。所以默认情况下这些 PV 的挂载参数是固定的（继承自 StorageClass）。但如果使用自定义 Provisoner，就可以为不同 PVC 创建使用不同挂载参数的 PV。

此特性默认关闭，需要手动启用。启用的方式就是为 CSI Controller 增添 `--provisioner=true` 启动参数，并且删去原本的 sidecar 容器，相当于让 CSI Controller 主进程自行监听资源变更，并执行相应的初始化操作。请根据 CSI Controller 的安装方式，按照下方步骤启用。

:::info
[进程挂载模式](../introduction.md#by-process)不支持该功能。
:::

### Helm

在 `values.yaml` 中添加如下配置：

```yaml title="values.yaml"
controller:
  provisioner: true
```

再重新部署 JuiceFS CSI 驱动：

```shell
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### kubectl

如果是 kubectl 安装方式，启用该功能需要手动编辑 CSI Controller，操作较为复杂，因此建议[迁移到 Helm 安装方式](../administration/upgrade-csi-driver.md#migrate-to-helm)。

```shell
kubectl edit sts -n kube-system juicefs-csi-controller
```

需要修改的部分，已经在下方示范中进行高亮和注释，请参考：

```diff
 apiVersion: apps/v1
 kind: StatefulSet
 metadata:
   name: juicefs-csi-controller
   ...
 spec:
   ...
   template:
     ...
     spec:
       containers:
         - name: juicefs-plugin
           image: juicedata/juicefs-csi-driver:v0.17.4
           args:
             - --endpoint=$(CSI_ENDPOINT)
             - --logtostderr
             - --nodeid=$(NODE_NAME)
             - --v=5
+            # 令 juicefs-plugin 自行监听资源变动，执行初始化流程
+            - --provisioner=true
         ...
-        # 删除默认的 csi-provisioner，不再通过该容器监听资源变动，执行初始化流程
-        - name: csi-provisioner
-          image: quay.io/k8scsi/csi-provisioner:v1.6.0
-          args:
-            - --csi-address=$(ADDRESS)
-            - --timeout=60s
-            - --v=5
-          env:
-            - name: ADDRESS
-              value: /var/lib/csi/sockets/pluginproxy/csi.sock
-          volumeMounts:
-            - mountPath: /var/lib/csi/sockets/pluginproxy/
-              name: socket-dir
         - name: liveness-probe
           image: quay.io/k8scsi/livenessprobe:v1.1.0
           args:
             - --csi-address=$(ADDRESS)
             - --health-port=$(HEALTH_PORT)
           env:
             - name: ADDRESS
               value: /csi/csi.sock
             - name: HEALTH_PORT
               value: "9909"
           volumeMounts:
             - mountPath: /csi
               name: socket-dir
         ...
```

上述操作也可以用下方的一行命令达成，但请注意，**该命令并非幂等，不能重复执行**：

```shell
kubectl -n kube-system patch sts juicefs-csi-controller \
  --type='json' \
  -p='[{"op": "remove", "path": "/spec/template/spec/containers/1"}, {"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--provisioner=true"]}]'
```

### 使用场景

#### 根据网络区域设置缓存组 {#regional-cache-group}

挂载参数模版可以为不同网络区域的客户端设置不同的 `cache-group`。首先我们为不同网络区域的节点设置 annotations 以标记缓存组名称：

```shell
kubectl annotate --overwrite node minikube myjfs.juicefs.com/cacheGroup=region-1
```

然后在 `StorageClass` 中修改相关配置：

```yaml {11-13}
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
mountOptions:
  - cache-group="${.node.annotations.myjfs.juicefs.com/cacheGroup}"
# 必须设置为 `WaitForFirstConsumer`，否则 PV 会提前创建，此时不确定被分配的 Node，cache-group 注入不生效。
volumeBindingMode: WaitForFirstConsumer
```

当创建 PVC 和使用它的 Pod 后，可以用下方命令核实 Provisioner 把节点 annotations 注入了相应的 PV：

```bash {8}
$ kubectl get pv pvc-4f2e2384-61f2-4045-b4df-fbdabe496c1b -o yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pvc-4f2e2384-61f2-4045-b4df-fbdabe496c1b
spec:
  mountOptions:
  - cache-group="region-1"
```

#### 配置更加易读的 PV 目录名称 {#using-path-pattern}

在「动态配置」方式下，CSI 驱动在 JuiceFS 创建的子目录名称形如 `pvc-234bb954-dfa3-4251-9ebe-8727fb3ad6fd`，如果有众多应用同时使用 CSI 驱动，更会造成 JuiceFS 文件系统中创建大量此类 PV 目录，让人难以辨别：

```shell
$ ls /jfs
pvc-76d2afa7-d1c1-419a-b971-b99da0b2b89c  pvc-a8c59d73-0c27-48ac-ba2c-53de34d31944  pvc-d88a5e2e-7597-467a-bf42-0ed6fa783a6b
...
```

在 JuiceFS CSI 驱动 0.13.3 及以上版本，支持通过 `pathPattern` 这个配置来定义其不同 PV 的子目录格式，让目录名称更容易阅读、查找：

```shell
$ ls /jfs
default-dummy-juicefs-pvc  default-example-juicefs-pvc ...
```

:::tip
如果你的场景需要在动态配置下，让多个应用使用同一个 JuiceFS 子目录，也可以合理配置 `pathPattern`，让多个 PV 对应着 JuiceFS 文件系统中相同的子目录，实现多应用共享存储。顺带一提，[「静态配置」](#share-directory)是更为简单直接的实现多应用共享存储的方式（多个应用复用同一个 PVC 即可），如果条件允许，不妨优先采用静态配置方案。
:::

在 `StorageClass` 中这样使用 `pathPattern`：

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
  pathPattern: "${.pvc.namespace}-${.pvc.name}"
```

### 模板注入值参考

在 0.23.3 版本中，挂载参数和 `pathPattern` 中均可注入 Node 和 PVC 的元数据，比如：

1. `${.node.name}-${.node.podCIDR}`，注入 Node 的 `metadata.name` 和 `spec.podCIDR`，例如 `minikube-10.244.0.0/24`
2. `${.node.labels.foo}`，注入 Node 的 `metadata.labels["foo"]`
3. `${.node.annotations.bar}`，注入 Node 的 `metadata.annotations["bar"]`
4. `${.pvc.namespace}-${.pvc.name}`，注入 PVC 的 `metadata.namespace` 和 `metadata.name`，例如 `default-dynamic-pvc`
5. `${.PVC.namespace}-${.PVC.name}`，注入 PVC 的 `metadata.namespace` 和 `metadata.name`（与旧版本兼容）
6. `${.pvc.labels.foo}`，注入 PVC 的 `metadata.labels["foo"]`
7. `${.pvc.annotations.bar}`，注入 PVC 的 `metadata.annotations["bar"]`

而在更早版本中（>=0.13.3）只有 `pathPattern` 支持注入，且仅支持注入 PVC 的元数据，比如：

1. `${.PVC.namespace}/${.PVC.name}`，注入 PVC 的 `metadata.namespace` 和 `metadata.name`，例如 `default/dynamic-pvc`
2. `${.PVC.labels.foo}`，注入 PVC 的 `metadata.labels["foo"]`
3. `${.PVC.annotations.bar}`，注入 PVC 的 `metadata.annotations["bar"]`

## 常用 PV 设置

### 挂载点自动恢复 {#automatic-mount-point-recovery}

JuiceFS CSI 驱动自 v0.10.7 开始支持挂载点自动恢复：当 Mount Pod 遭遇故障，重启或重新创建 Mount Pod 以后，应用容器也能继续工作。

:::note 注意
挂载点自动恢复后，已经打开的文件无法继续访问，请在应用程序中做好重试，重新打开文件，避免异常。
:::

启用自动恢复，需要在应用 Pod 的 `volumeMounts` 中[设置 `mountPropagation` 为 `HostToContainer` 或 `Bidirectional`](https://kubernetes.io/zh-cn/docs/concepts/storage/volumes/#mount-propagation)，从而将宿主机的挂载传播给 Pod。这样一来，Mount Pod 重启后，宿主机上的挂载点被重新挂载，然后 CSI 驱动将会在容器挂载路径上重新执行一次 mount bind。

```yaml {12-18}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: juicefs-app-static-deploy
spec:
  ...
  template:
    ...
    spec:
      containers:
      - name: app
        # 如果设置为 Bidirectional，则需要启用 privileged
        # securityContext:
        #   privileged: true
        volumeMounts:
        - mountPath: /data
          name: data
          mountPropagation: HostToContainer
        ...
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: juicefs-pvc-static
```

也可以使用社区开发者提供的工具，自动为应用容器添加 `mountPropagation: HostToContainer`，具体请参考[项目文档](https://github.com/breuerfelix/juicefs-volume-hook)。

### 缓存客户端配置文件 {#cache-client-conf}

从 v0.23.3 开始，CSI 驱动默认缓存了 JuiceFS 客户端的配置文件（配置文件是[企业版 JuiceFS 客户端的挂载配置](https://juicefs.com/docs/zh/cloud/reference/command_reference/#auth)，因此仅对 JuiceFS 企业版生效）。对这个配置文件启用缓存，能带来以下好处：

* &#8203;<Badge type="primary">私有部署</Badge> 如果私有 Web 控制台发生故障，或者网络异常、造成容器无法访问控制台，客户端依然可以读取缓存好的配置文件，正常挂载和服务

缓存配置文件的工作方式如下：

1. 用户填写或更新[文件系统认证信息](./pv.md#volume-credentials)，CSI Controller 会监听 Secret 的变化，并立刻发起认证、获取配置文件；
1. CSI Controller 将配置文件注入进 Secret，保存在 `initconfig` 字段；
1. 当 CSI Node 创建 mount pod，或者 CSI Controller 注入 sidecar 容器的时候，会将 `initconfig` 挂载进容器内；
1. 容器内的 JuiceFS 客户端会运行 [`juicefs auth`](https://juicefs.com/docs/zh/cloud/reference/command_reference/#auth) 命令，但由于配置文件已经挂载进容器内，因此就算容器无法访问 JuiceFS Web 控制台，挂载也能照常继续。

如果希望关闭该功能，需要将 Helm 集群配置中的 [`cacheClientConf`](https://github.com/juicedata/charts/blob/96dafec08cc20a803d870b38dcc859f4084a5251/charts/juicefs-csi-driver/values.yaml#L114-L115) 字段设置为 `false`。

### PV 容量分配 {#storage-capacity}

从 v0.19.3 开始，JuiceFS CSI 驱动支持在动态配置设置存储容量（要注意，仅支持动态配置）。

在静态配置中，PVC 中指定的容量会被忽略，填写任意有效值即可，建议填写一个较大的数值，避免未来版本如果带来该功能支持时，因为容量超限导致问题。

```yaml
...
storageClassName: ""
resources:
  requests:
    storage: 10Ti
```

而在动态配置中，可以在 PVC 中指定存储容量，这个容量限制将会被翻译成 `juicefs quota` 命令，在 CSI Controller 中执行，为该 PV 所对应的子目录添加容量限制。关于 `juicefs quota` 命令，可以参考[社区版文档](https://juicefs.com/docs/zh/community/command_reference/#quota)，商业版文档待补充。

```yaml
...
storageClassName: juicefs-sc
resources:
  requests:
    storage: 100Gi
```

创建并挂载好 PV 后，可以进入容器用 `df -h` 验证容量生效：

```shell
$ df -h
Filesystem         Size  Used Avail Use% Mounted on
overlay             84G   66G   18G  80% /
tmpfs               64M     0   64M   0% /dev
JuiceFS:myjfs       100G     0  100G   0% /data-0
```

### PV 扩容 {#pv-expansion}

在 JuiceFS CSI 驱动 0.21.0 及以上版本，支持动态扩展 PersistentVolume 的容量（仅支持[动态配置](./pv.md#dynamic-provisioning)）。需要在 [StorageClass](./pv.md#create-storage-class) 中指定 `allowVolumeExpansion: true`，同时指定扩容时所需使用的 Secret，主要提供文件系统的认证信息，例如：

```yaml {9-11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
...
parameters:
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/controller-expand-secret-name: juicefs-secret   # 与 provisioner-secret-name 相同即可
  csi.storage.k8s.io/controller-expand-secret-namespace: default     # 与 provisioner-secret-namespace 相同即可
allowVolumeExpansion: true         # 表示支持扩容
```

然后通过编辑 PVC 的 `spec` 字段，指定更大的存储请求，可以触发 PersistentVolume 的扩充：

```yaml {10}
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi  # 在此处指定更大的容量
```

### 访问模式 {#access-modes}

JuiceFS PV 支持 `ReadWriteMany` 和 `ReadOnlyMany` 两种访问方式。根据使用 CSI 驱动的方式不同，在上方 PV／PVC（或 `volumeClaimTemplate`）定义中，填写需要的 `accessModes` 即可。

### 回收策略 {#reclaim-policy}

静态配置下仅支持 `persistentVolumeReclaimPolicy: Retain`，无法随着删除回收。

动态配置支持 `Delete|Retain` 两种回收策略，按需使用。`Delete` 会导致 JuiceFS 内的 PVC 子目录随着 PV 删除一起释放，如果担心数据安全，可以配合 JuiceFS 的回收站功能一起使用：

* [社区版回收站文档](https://juicefs.com/docs/zh/community/security/trash)
* [企业版回收站文档](https://juicefs.com/docs/zh/cloud/trash)

### 给 Mount Pod 挂载宿主机目录 {#mount-host-path}

如果希望在 Mount Pod 中挂载宿主机文件或目录，可以声明 `juicefs/host-path`，可以在这个字段中填写多个文件映射，逗号分隔。这个字段在静态和动态配置方式中填写位置不同，以 `/data/file.txt` 这个文件为例，详见下方示范。

#### 静态配置

```yaml {17}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  ...
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/host-path: /data/file.txt
```

#### 动态配置

```yaml {7}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  juicefs/host-path: /data/file.txt
```

#### 高级用法

将 `/etc/hosts` 映射进容器，某些场景下可能需要让容器复用宿主机的 `/etc/hosts`，但通常而言，如果希望为容器添加 hosts 记录，优先考虑使用 [`HostAliases`](https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/)。

```yaml
juicefs/host-path: "/etc/hosts"
```

如果有需要，可以映射多个文件或目录，逗号分隔：

```yaml
juicefs/host-path: "/data/file1.txt,/data/file2.txt,/data/dir1"
```
