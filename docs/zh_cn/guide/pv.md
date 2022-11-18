---
title: 创建和使用 PV
sidebar_position: 1
---

本章详细介绍如何创建和使用 PV，以及常见用法和配置。

## 创建挂载配置 {#juicefs-secret}

在 JuiceFS CSI 驱动中，挂载所需的认证信息，以及挂载参数，均存在 Kubernetes Secret 中。所以为了使用 CSI Driver，首先需要创建名为 `juicefs-secret` 的 Kubernetes Secret。

### 社区版

建议在使用 CSI 驱动前先创建好文件系统，运行 [`juicefs format`](https://juicefs.com/docs/zh/community/command_reference#juicefs-format) 命令创建文件系统的同时，还能确保各种认证信息是正确的：

```shell
juicefs format --storage=s3 --bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --access-key=<ACCESS_KEY> --secret-key=<SECRET_KEY> \
    <META_URL>
```

创建 Kubernetes Secret：

```yaml {7-14}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <NAME>
  metaurl: <META_URL>
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  # 设置 mount pod 时区，默认为 UTC
  # envs: "{TZ: Asia/Shanghai}"
  # 如需在 mount pod 中创建文件系统，也可以直接填入 format-options
  # format-options: trash-days=1,block-size=4096
```

其中：

- `name`：JuiceFS 文件系统名称
- `metaurl`：元数据服务的访问 URL (比如 Redis)。更多信息参考[如何设置元数据引擎](https://juicefs.com/docs/zh/community/databases_for_metadata) 。
- `storage`：对象存储类型，比如 `s3`，`gs`，`oss`。更多信息参考[如何设置对象存储](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `bucket`：Bucket URL。更多信息参考[如何设置对象存储](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `access-key`/`secret-key`：对象存储的认证信息。
- `format-options`：创建文件系统（[`format` 命令](https://juicefs.com/docs/zh/community/command_reference#juicefs-format)）所使用的的参数。

  如遇重复参数，比如 access-key 既可以在 `juicefs-secret` 中填写，同时也可以在 `format-options` 下填写，此时 `format-options` 的参数优先级最高。


### 云服务

```yaml {7-12}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${JUICEFS_ACCESSKEY}
  secret-key: ${JUICEFS_SECRETKEY}
  # 如果需要设置 JuiceFS Mount Pod 的时区请将下一行的注释符号删除，默认为 UTC 时间。
  # envs: "{TZ: Asia/Shanghai}"
  # 如需在 mount pod 中运行 juicefs auth 命令，调整挂载配置，可以将 auth 命令参数填写至 format-options
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

其中：
- `name`：JuiceFS 文件系统名称
- `token`：JuiceFS 管理 token。更多信息参考[访问令牌](https://juicefs.com/docs/zh/cloud/metadata#令牌管理)。
- `access-key`/`secret-key`：对象存储的认证信息。
- `format-options`：云服务 [`auth` 命令](https://juicefs.com/docs/zh/cloud/commands_reference#auth)所使用的的参数，作用是认证，以及生成挂载的配置文件。

  如遇重复参数，比如 access-key 既可以在 `juicefs-secret` 中填写，同时也可以在 `format-options` 下填写，此时 `format-options` 的参数优先级最高。

  在 CSI 驱动看来，云服务的 `auth` 命令，作用类似于社区版的 `format`，也正因此，字段名依然叫做 `format-options`。


## 创建 StorageClass

如果你打算以动态配置（Dynamic provisioning）的方式使用 JuiceFS CSI 驱动，那么你需要提前创建 StorageClass。包括下文中[「通用临时卷」](#general-ephemeral-storage)的使用方式也在此列。

```yaml
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
```

如果需要调整挂载参数，可以在上方的 StorageClass 定义中追加 `mountOptions` 配置。可想而知，如果需要为不同应用使用不同挂载参数，则需要创建多个 StorageClass，单独添加所需参数。

```yaml
mountOptions:
  - enable-xattr
  - max-uploads=50
  - cache-size=2048
  - cache-dir=/var/foo
  - allow_other
```

社区版与云服务的挂载参数有所区别，请参考文档：

- 社区版 [`juicefs mount`](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount)
- 云服务 [`juicefs mount`](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)

## 动态配置（Dynamic provisioning）

动态配置方式需要先行[创建 StorageClass](#storageclass)。

### 部署

创建 `PersistentVolumeClaim`（PVC）和示例 pod：

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

```sh
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

## 使用通用临时卷 {#general-ephemeral-storage}

Kubernetes 的[通用临时卷](https://kubernetes.io/zh-cn/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes)类似于 `emptyDir`，为 pod 提供临时数据存放目录。当容器需要大容量临时存储时，可以考虑这样使用 JuiceFS CSI 驱动。

JuiceFS CSI 驱动的通用临时卷用法与「动态配置」类似，因此也需要先行[创建 StorageClass](#storageclass)。不过与「动态配置」不同，临时卷使用 `volumeClaimTemplate`，能直接为你自动创建 PVC。

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

:::note 注意
临时卷的用法原理与动态配置一致，因此如果将 [默认 PV 回收策略](./resource-optimization.md#reclaim-policy)设置为 `Retain`，那么临时存储将不再是临时存储，PV 需要手动释放。
:::

## 静态配置

所谓「静态配置」，在本文档指的就是手动创建 PV、PVC，流程类似[「配置 Pod 以使用 PersistentVolume 作为存储」](https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/configure-persistent-volume-storage/)。

动态配置免除了手动创建和管理 PV、PVC 的麻烦，但如果你在 JuiceFS 中已经有了大量数据，希望能在 Kubernetes 中直接挂载到容器中使用，则需要选用静态配置的方式来使用。

### 部署

创建 `PersistentVolume`（PV）、`PersistentVolumeClaim`（PVC）和示例 pod：

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
# 确认 PV 正常创建，容量显示正确
kubectl get pv
# 确认 pod 正常运行
kubectl get pods
# 确认数据被正确地写入 JuiceFS 文件系统中
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
# 与此同时，也可以直接在宿主机挂载 JuiceFS，确认 PV 对应的子目录已经在文件系统中创建
```

如果需要调整挂载参数，可以在上方的 PV 定义中追加 `mountOptions` 配置：

```yaml
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

社区版与云服务的挂载参数有所区别，请参考文档：

- 社区版 [`juicefs mount`](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount)
- 云服务 [`juicefs mount`](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)

## 常用 PV 设置

### PV 容量分配 {#storage-capacity}

目前而言，JuiceFS CSI 驱动不支持设置存储配额。在 `PersistentVolume` 和 `PersistentVolumeClaim` 中指定的容量会被忽略，填写任意有效值即可，例如 `100Gi`：

```yaml
resources:
  requests:
    storage: 100Gi
```

### 访问模式 {#access-modes}

JuiceFS PV 支持 `ReadWriteMany` 和 `ReadOnlyMany` 两种访问方式。根据使用 CSI 驱动的方式不同，在 PV、PVC，或者 `volumeClaimTemplate` 中填写需要的 `accessModes` 即可。详见上方各小节中代码块，搜索 `accessModes`。
