---
title: 创建和使用 PV
sidebar_position: 1
---

## 文件系统认证信息 {#volume-credentials}

在 JuiceFS CSI 驱动中，挂载文件系统所需的认证信息，均存在 Kubernetes Secret 中。所谓「认证信息」，在 JuiceFS 社区版和云服务有着不同的含义：

* 对于社区版而言，「认证信息」包含元数据引擎 URL、对象存储密钥，以及 [`juicefs format`](https://juicefs.com/docs/zh/community/command_reference#format) 命令所支持的其他参数。
* 对于云服务而言，「认证信息」包含文件系统名称、Token、对象存储密钥，以及 [`juicefs auth`](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#auth) 命令所支持的其他参数。

:::note 注意

* 如果你已经在[用 Helm 管理 StorageClass](#helm-sc)，那么 Kubernetes Secret 其实已经一并创建，不需要再用 kubectl 单独创建和管理 Secret。
* 修改了文件系统认证信息后，还需要滚动升级或重启应用 Pod，CSI 驱动重新创建 Mount Pod，配置变更方能生效。
* Secret 中只存储文件系统认证信息（也就是社区版 `juicefs format` 和云服务 `juicefs auth` 命令所需的参数），并不支持填写挂载参数，如果你希望修改挂载参数，参考[「挂载参数」](#mount-options)。

:::

### 社区版 {#community-edition}

创建 Kubernetes Secret：

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
  # 如需在 Mount Pod 中创建文件系统，也可以将更多 juicefs format 参数填入 format-options。
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

### 云服务 {#cloud-service}

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
  # 如需指定更多认证参数，可以将 juicefs auth 命令参数填写至 format-options。
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

字段说明：

- `name`：JuiceFS 文件系统名称
- `token`：访问 JuiceFS 文件系统所需的 token。更多信息参考[访问令牌](https://juicefs.com/docs/zh/cloud/acl/#%E8%AE%BF%E9%97%AE%E4%BB%A4%E7%89%8C)。
- `access-key`/`secret-key`：对象存储的认证信息
- `envs`：Mount Pod 的环境变量
- `format-options`：云服务 [`juicefs auth`](https://juicefs.com/docs/zh/cloud/commands_reference#auth) 命令所使用的的参数，作用是认证，以及生成挂载的配置文件。该选项仅在 v0.13.3 及以上可用

如遇重复参数（比如 `access-key`），既可以在 Kubernetes Secret 中填写，同时也可以在 `format-options` 下填写，此时 `format-options` 的参数优先级最高。

云服务的 `juicefs auth` 命令作用类似于社区版的 `juicefs format` 命令，因此字段名依然叫做 `format-options`。

### 企业版（私有部署） {#enterprise-edition}

JuiceFS Web 控制台负责着客户端的挂载认证、配置文件下发等工作。而在私有部署环境中，控制台的地址不再是 [https://juicefs.com/console](https://juicefs.com/console)，因此需要在文件系统认证信息中通过 `envs` 字段额外指定控制台地址。

```yaml {12-13}
apiVersion: v1
metadata:
  name: juicefs-secret
  namespace: default
kind: Secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${ACCESS_KEY}
  secret-key: ${SECRET_KEY}
  # 不需要对 `%s` 进行任何替换更改，在执行文件系统挂载时，客户端会用实际文件系统名来替换该占位符
  envs: '{"BASE_URL": "$JUICEFS_CONSOLE_URL/static", "CFG_URL": "$JUICEFS_CONSOLE_URL/volume/%s/mount"}'
  # 如需指定更多认证参数，可以将 juicefs auth 命令参数填写至 format-options
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

字段说明：

- `name`：JuiceFS 文件系统名称
- `token`：访问 JuiceFS 文件系统所需的 token。更多信息参考[访问令牌](https://juicefs.com/docs/zh/cloud/acl/#%E8%AE%BF%E9%97%AE%E4%BB%A4%E7%89%8C)。
- `access-key`/`secret-key`：对象存储的认证信息
- `envs`：Mount Pod 的环境变量，在私有部署中需要额外填写 `BASE_URL`、`CFG_URL`，指向实际控制台地址
- `format-options`：云服务 [`juicefs auth`](https://juicefs.com/docs/zh/cloud/commands_reference#auth) 命令所使用的的参数，作用是认证，以及生成挂载的配置文件。该选项仅在 v0.13.3 及以上可用

### 为 Mount Pod 额外添加文件、环境变量 {#mount-pod-extra-files}

部分对象存储服务（比如 Google 云存储）在访问时需要提供额外的认证文件，这就需要你用创建单独的 Secret 保存这些文件，然后在认证信息（下方示范中的 `juicefs-secret`）中引用。这样一来，CSI 驱动便会将这些文件挂载进 Mount Pod，然后在 Mount Pod 中添加对应的环境变量，令 JuiceFS 挂载时使用该文件进行对象存储的认证。

除此外，如果希望单独为 Mount Pod 添加环境变量，也可以在认证信息的 `envs` 中声明。比方说使用 MinIO 时，可能需要为客户端设定 `MINIO_REGION` 环境变量。

下方以 Google 云存储为例，演示如何为 Mount Pod 额外添加文件、环境变量。

获取 Google 云存储所需要的[服务帐号密钥文件](https://cloud.google.com/docs/authentication/production#create_service_account)，需要先了解如何进行[身份验证](https://cloud.google.com/docs/authentication)和[授权](https://cloud.google.com/iam/docs/overview)。假设你已经获取到了密钥文件 `application_default_credentials.json`，用下方命令将该配置文件创建成 Kubernetes Secret：

```shell
kubectl create secret generic gc-secret \
  --from-file=application_default_credentials.json=application_default_credentials.json
```

运行上方命令，密钥文件就被保存在 `gc-secret` 中了，接下来需要在 `juicefs-secret` 中加以引用，让 CSI 驱动将该文件挂载到 Mount Pod 中，并添加相应的环境变量：

```yaml {8-11}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  ...
  # 在 configs 中填写 Secret 名称和挂载目录，将该 Secret 整体挂载进指定的目录
  configs: "{gc-secret: /root/.config/gcloud}"
  # 定义挂载认证所需的环境变量
  envs: "{GOOGLE_APPLICATION_CREDENTIALS: /root/.config/gcloud/application_default_credentials.json}"
```

添加完毕以后，新创建的 PV 便会使用此配置了，你可以[进入 Mount Pod 里](../administration/troubleshooting.md#check-mount-pod)，确认配置文件挂载正确，然后用 `env` 命令确认环境变量也设置成功。

## 静态配置 {#static-provisioning}

静态配置是最简单直接地在 Kubernetes 中使用 JuiceFS PV 的方式，阅读[「使用方式」](../introduction.md#usage)以了解「动态配置」与「静态配置」的区别。

创建所需的资源定义示范如下，字段含义请参考注释：

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  # 目前 JuiceFS CSI 驱动不支持设置存储容量，填写任意有效值即可
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    # 在先前的安装步骤中，已经创建了名为 csi.juicefs.com 的 CSIDriver
    driver: csi.juicefs.com
    # volumeHandle 需要保证集群内唯一，因此一般直接用 PV 名即可
    volumeHandle: juicefs-pv
    fsType: juicefs
    # 在先前的步骤中已经创建好文件系统认证信息（Secret），在这里引用
    # 如果要在静态配置下使用不同的认证信息，甚至使用不同的 JuiceFS 文件系统，则需要创建不同的 Secret
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
  # 静态配置下必须指定 storageClassName 为空字符串
  # 代表该 PV 不采用任何 StorageClass，而是直接使用 selector 所指定的 PV
  storageClassName: ""
  # 由于目前 JuiceFS CSI 驱动不支持设置存储容量，此处 requests.storage 填写任意小于 PV capacity 的有效值即可
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
```

创建应用 Pod，并在其中引用 PVC，示范：

```yaml
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

Pod 创建完成后，你就能在 JuiceFS 挂载点看到上方容器写入的 `out.txt` 了。在静态配置下，如果没有显式指定[挂载子目录](#mount-subdirectory)，文件系统的根目录将会被挂载进容器，因此如果对应用有数据隔离的要求，请挂载子目录，或者使用[动态配置](#dynamic-provisioning)。

## 创建 StorageClass {#create-storage-class}

[StorageClass](https://kubernetes.io/zh-cn/docs/concepts/storage/storage-classes)（存储类）里指定了创建 PV 所需的各类配置，你可以将其理解为动态配置下的「Profile」：不同的 StorageClass 就是不同的 Profile，可以在其中指定不同的文件系统认证信息、挂载配置，让动态配置下可以同时使用不同的文件系统，或者指定不同的挂载。因此如果你打算以[「动态配置」](#dynamic-provisioning)或[「通用临时卷」](#general-ephemeral-storage)的方式使用 JuiceFS CSI 驱动，那么你需要提前创建 StorageClass。

注意，StorageClass 仅仅是动态配置下用于创建 PV 的「模板」，也正因此，**在 StorageClass 中修改挂载配置，不影响已经创建的 PV。**如果你需要调整挂载配置，需要删除 PVC 重建，或者直接[在 PV 级别调整挂载配置](#static-mount-options)

### 通过 Helm 创建 {#helm-sc}

:::tip

* 通过 Helm 创建 StorageClass，要求用户将认证信息明文填入 `values.yaml`，考虑到安全性，生产环境一般推荐[用 kubectl 创建](#kubectl-sc)。
* 如下方示范中 `backend` 字段所示，用 Helm 创建 StorageClass 时，文件系统认证信息也会一并创建，请在 Helm 里直接管理，无需再[单独创建文件系统认证信息](#volume-credentials)。
:::

JuiceFS 社区版和云服务的配置项略有不同，下方示范面向社区版，但你可以在 [Helm chart](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml#L122) 中找到全面示范。

```yaml title="values.yaml"
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  # JuiceFS 文件系统认证信息
  # 如果已经提前创建好文件系统，则只需填写 `name` 和 `metaurl` 这两项
  backend:
    name: "<name>"                # JuiceFS 文件系统名
    metaurl: "<meta-url>"         # 元数据引擎的 URL
    storage: "<storage-type>"     # 对象存储类型 (例如 s3、gcs、oss、cos)
    accessKey: "<access-key>"     # 对象存储的 Access Key
    secretKey: "<secret-key>"     # 对象存储的 Secret Key
    bucket: "<bucket>"            # 存储数据的桶路径
    # 设置 Mount Pod 时区，默认为 UTC。
    # envs: "{TZ: Asia/Shanghai}"
  mountPod:
    resources:                    # Mount pod 的资源配置
      requests:
        cpu: "1"
        memory: "1Gi"
      limits:
        cpu: "5"
        memory: "5Gi"
```

### 通过 kubectl 创建 {#kubectl-sc}

用 kubectl 创建 StorageClass，需要提前创建好[「文件系统认证信息」](#volume-credentials)，创建完毕后，将相关信息按照下方示范填入对应字段。

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

## 动态配置 {#dynamic-provisioning}

阅读[「使用方式」](../introduction.md#usage)以了解什么是「动态配置」。动态配置方式会自动为你创建 PV，而创建 PV 的基础配置参数在 StorageClass 中定义，因此你需要先行[创建 StorageClass](#create-storage-class)。

创建 PVC 和应用 Pod，示范如下：

```yaml {13}
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

[通用临时卷](https://kubernetes.io/zh-cn/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes)类似于 `emptyDir`，为每个 Pod 单独提供临时数据存放目录。当应用容器需要大容量，并且是每个 Pod 单独的临时存储时，可以考虑这样使用 JuiceFS CSI 驱动。

JuiceFS CSI 驱动的通用临时卷用法与「动态配置」类似，因此也需要先行[创建 StorageClass](#create-storage-class)。不过与「动态配置」不同，临时卷使用 `volumeClaimTemplate`，为每个 Pod 自动创建 PVC。

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

如果要额外添加 FUSE 相关选项（也就是挂载命令的 `-o` 参数），请直接在 YAML 列表中追加，每行一个选项：

```yaml
mountOptions:
  - cache-size=204800
  # 额外的 FUSE 相关选项
  - writeback_cache
  - debug
```

## 应用间共享存储 {#share-directory}

如果你在 JuiceFS 文件系统已经存储了大量数据，希望挂载进容器使用，或者希望让多个应用共享同一个 JuiceFS 目录，有以下做法：

### 静态配置

#### 挂载子目录 {#mount-subdirectory}

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

除此外，还可以使用 `csi.volumeAttributes.subPath` 来指定 PV 在 JuiceFS 上的目录名称，例如：

```yaml {10-11}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  csi:
    volumeAttributes:
      # 不支持多级目录，只能填写一个根目录名称
      subPath: my-sub-dir
  ...
```

需要指出的是，调整 `subdir` 挂载参数，是更为推荐的方式。`subPath` 方式灵活性较差，除了不支持多级目录外，在缓存预热场景中，或者子目录存在权限限制，`subPath` 方式均会遭遇错误，此参数多用于 CSI 内部调试，请勿用于生产环境。

#### 跨命名空间（namespace）共享同一个文件系统 {#sharing-same-file-system-across-namespaces}

如果想要在不同命名空间中共享同一个文件系统，只需要让不同 PV 使用相同的文件系统认证信息（Secret）即可：

```yaml {10-12,24-26}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: mypv1
  namespace: ns1
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
  namespace: ns2
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

## 配置更加易读的 PV 目录名称 {#using-path-pattern}

:::tip

[进程挂载模式](../introduction.md#by-process)不支持该功能。
:::

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

如果你的场景需要在动态配置下，让多个应用使用同一个 JuiceFS 子目录，也可以合理配置 `pathPattern`，让多个 PV 对应着 JuiceFS 文件系统中相同的子目录，实现多应用共享存储。顺带一提，[「静态配置」](#share-directory)是更为简单直接的实现多应用共享存储的方式（多个应用复用同一个 PVC 即可），如果条件允许，不妨优先采用静态配置方案。

此特性默认关闭，需要手动启用。启用的方式就是为 CSI Controller 增添 `--provisioner=true` 启动参数，并且删去原本的 sidecar 容器，相当于让 CSI Controller 主进程自行监听资源变更，并执行相应的初始化操作。请根据 CSI Controller 的安装方式，按照下方步骤启用。

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

手动编辑 CSI Controller：

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

### 使用方式

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
  pathPattern: "${.PVC.namespace}-${.PVC.name}"
```

命名模板中可以引用任意 PVC 元数据，例如标签、注解、名称或命名空间，比如：

1. `${.PVC.namespace}-${.PVC.name}`，则 PV 目录为 `<PVC 命名空间>-<PVC 名称>`。
1. `${.PVC.labels.foo}`，则 PV 目录为 PVC 中 `foo` 标签的值。
1. `${.PVC.annotations.bar}`，则 PV 目录为 PVC 中 `bar` 注解（annotation）的值。

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

### PV 容量分配 {#storage-capacity}

目前而言，JuiceFS CSI 驱动不支持设置存储容量。在 PersistentVolume 和 PersistentVolumeClaim 中指定的容量会被忽略，填写任意有效值即可，例如 `100Gi`：

```yaml
resources:
  requests:
    storage: 100Gi
```

### 访问模式 {#access-modes}

JuiceFS PV 支持 `ReadWriteMany` 和 `ReadOnlyMany` 两种访问方式。根据使用 CSI 驱动的方式不同，在上方 PV/PVC，（或 `volumeClaimTemplate`）定义中，填写需要的 `accessModes` 即可。
