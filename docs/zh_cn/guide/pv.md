---
title: 创建和使用 PV
sidebar_position: 1
---

## 创建挂载配置 {#create-mount-config}

在 JuiceFS CSI 驱动中，挂载文件系统所需的认证信息以及挂载参数，均存在 Kubernetes Secret 中。所以为了使用 JuiceFS 文件系统，首先需要创建 Kubernetes Secret。

:::note
如果你已经在[用 Helm 管理 StorageClass](../getting_started.md#helm-sc)，那么 Kubernetes Secret 其实已经一并创建，此时我们推荐你继续直接用 Helm 管理 StorageClass 与 Secret，而不是用 kubectl 再单独创建和管理 Secret。
:::

### 社区版

使用 PV 前，请先[创建好 JuiceFS 文件系统](https://juicefs.com/docs/zh/community/quick_start_guide#%E5%88%9B%E5%BB%BA%E6%96%87%E4%BB%B6%E7%B3%BB%E7%BB%9F)。例如：

```shell
juicefs format \
    --storage=s3 \
    --bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --access-key=<ACCESS_KEY> --secret-key=<SECRET_KEY> \
    <META_URL> \
    <NAME>
```

然后创建 Kubernetes Secret：

```yaml {7-16}
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
  # 设置 Mount Pod 时区，默认为 UTC。
  # envs: "{TZ: Asia/Shanghai}"
  # 如需在 Mount Pod 中创建文件系统，也可以直接填入 format-options。
  # format-options: trash-days=1,block-size=4096
```

字段说明：

- `name`：JuiceFS 文件系统名称
- `metaurl`：元数据服务的访问 URL。更多信息参考[「如何设置元数据引擎」](https://juicefs.com/docs/zh/community/databases_for_metadata) 。
- `storage`：对象存储类型，比如 `s3`，`gs`，`oss`。更多信息参考[「如何设置对象存储」](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `bucket`：对象存储 Bucket URL。更多信息参考[「如何设置对象存储」](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `access-key`/`secret-key`：对象存储的认证信息
- `envs`：Mount Pod 的环境变量
- `format-options`：创建文件系统的选项，详见 [`juicefs format`](https://juicefs.com/docs/zh/community/command_reference#format)。该选项仅在 v0.13.3 及以上可用。

如遇重复参数（比如 `access-key`），既可以在 Kubernetes Secret 中填写，同时也可以在 `format-options` 下填写，此时 `format-options` 的参数优先级最高。

### 云服务

操作之前，请先在 JuiceFS 云服务中[创建文件系统](https://juicefs.com/docs/zh/cloud/getting_started#create-file-system)。

创建 Kubernetes Secret：

```yaml {7-14}
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
  # 设置 Mount Pod 时区，默认为 UTC。
  # envs: "{TZ: Asia/Shanghai}"
  # 如需在 Mount Pod 中运行 juicefs auth 命令以调整挂载配置，可以将 auth 命令参数填写至 format-options。
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

字段说明：

- `name`：JuiceFS 文件系统名称
- `token`：访问 JuiceFS 文件系统所需的 token。更多信息参考[访问令牌](https://juicefs.com/docs/zh/cloud/acl/#%E8%AE%BF%E9%97%AE%E4%BB%A4%E7%89%8C)。
- `access-key`/`secret-key`：对象存储的认证信息
- `envs`：Mount Pod 的环境变量
- `format-options`：云服务 [`juicefs auth`](https://juicefs.com/docs/zh/cloud/commands_reference#auth) 命令所使用的的参数，作用是认证，以及生成挂载的配置文件。该选项仅在 v0.13.3 及以上可用。

如遇重复参数（比如 `access-key`），既可以在 Kubernetes Secret 中填写，同时也可以在 `format-options` 下填写，此时 `format-options` 的参数优先级最高。

云服务的 `juicefs auth` 命令作用类似于社区版的 `juicefs format` 命令，因此字段名依然叫做 `format-options`。

## 动态配置 {#dynamic-provisioning}

阅读[「使用方式」](../introduction.md#usage)以了解什么是「动态配置」。动态配置方式会自动为你创建 PV，而创建 PV 的基础配置参数在 StorageClass 中定义，因此你需要先行[创建 StorageClass](../getting_started.md#create-storage-class)。

### 部署

创建 PersistentVolumeClaim（PVC）和示例 pod：

```yaml
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: juicefs-pv
  volumes:
  - name: juicefs-pv
    persistentVolumeClaim:
      claimName: juicefs-pvc
EOF
```

确认容器顺利创建运行后，可以钻进容器里确认数据正常写入 JuiceFS：

```shell
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

## 使用通用临时卷 {#general-ephemeral-storage}

[通用临时卷](https://kubernetes.io/zh-cn/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes)类似于 `emptyDir`，为 pod 提供临时数据存放目录。当应用容器需要大容量临时存储时，可以考虑这样使用 JuiceFS CSI 驱动。

JuiceFS CSI 驱动的通用临时卷用法与「动态配置」类似，因此也需要先行[创建 StorageClass](../getting_started.md#create-storage-class)。不过与「动态配置」不同，临时卷使用 `volumeClaimTemplate`，能直接为你自动创建 PVC。

在 Pod 定义中声明使用通用临时卷：

```yaml {19-30}
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: juicefs-pv
  volumes:
  - name: juicefs-pv
    ephemeral:
      volumeClaimTemplate:
        metadata:
          labels:
            type: juicefs-ephemeral-volume
        spec:
          accessModes: [ "ReadWriteMany" ]
          storageClassName: "juicefs-sc"
          resources:
            requests:
              storage: 1Gi
```

:::note
在回收策略方面，临时卷与动态配置一致，因此如果将[默认 PV 回收策略](./resource-optimization.md#reclaim-policy)设置为 `Retain`，那么临时存储将不再是临时存储，PV 需要手动释放。
:::

## 静态配置 {#static-provisioning}

阅读[「使用方式」](../introduction.md#usage)以了解什么是「静态配置」。

所谓「静态配置」，指的是自行创建 PV 和 PVC，流程类似[「配置 Pod 以使用 PersistentVolume 作为存储」](https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/configure-persistent-volume-storage/)。

动态配置免除了手动创建和管理 PV 的麻烦，但如果你在 JuiceFS 中已经有了大量数据，希望能在 Kubernetes 中直接挂载到容器中使用，则需要选用静态配置的方式来使用。

### 部署

创建 PersistentVolume（PV）、PersistentVolumeClaim（PVC）和示例 pod：

:::note 注意
PV 的 `volumeHandle` 需要保证集群内唯一，因此一般直接用 PV name 即可。
:::

```yaml
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
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  storageClassName: ""
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: data
    resources:
      requests:
        cpu: 10m
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: juicefs-pvc
```

当所有的资源创建好之后，可以用以下命令确认一切符合预期：

```shell
# 确认 PV 正常创建
kubectl get pv juicefs-pv

# 确认 pod 正常运行
kubectl get pods juicefs-app

# 确认数据被正确地写入 JuiceFS 文件系统中（也可以直接在宿主机挂载 JuiceFS，确认 PV 对应的子目录已经在文件系统中创建）
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

如果需要调整挂载参数，可以在上方的 PV 定义中追加 `mountOptions` 配置：

```yaml {8-13}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  mountOptions:
    - enable-xattr
    - max-uploads=50
    - cache-size=2048
    - cache-dir=/var/foo
    - allow_other
  ...
```

JuiceFS 社区版与云服务的挂载参数有所区别，请参考文档：

- [社区版](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount)
- [云服务](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)

## 常用 PV 设置

### 挂载点自动恢复

JuiceFS CSI Driver v0.10.7 开始支持挂载点自动恢复：即使 Mount Pod 遭遇故障，重启或重新创建以后，应用容器便能继续工作。

需要在应用 pod 的 `volumeMounts` 中[设置 `mountPropagation` 为 `HostToContainer` 或 `Bidirectional`](https://kubernetes.io/zh-cn/docs/concepts/storage/volumes/#mount-propagation)，从而将 host 的挂载信息传播给 pod：

```yaml
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

### PV 容量分配 {#storage-capacity}

目前而言，JuiceFS CSI 驱动不支持设置存储容量。在 PersistentVolume 和 PersistentVolumeClaim 中指定的容量会被忽略，填写任意有效值即可，例如 `100Gi`：

```yaml
resources:
  requests:
    storage: 100Gi
```

### 访问模式 {#access-modes}

JuiceFS PV 支持 `ReadWriteMany` 和 `ReadOnlyMany` 两种访问方式。根据使用 CSI 驱动的方式不同，在上方 PV/PVC，（或 `volumeClaimTemplate`）定义中，填写需要的 `accessModes` 即可。
