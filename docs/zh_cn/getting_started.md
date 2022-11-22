---
title: 安装
---

## 安装 JuiceFS CSI 驱动

JuiceFS CSI 驱动需要 Kubernetes 1.14 及以上版本，通过以下方法进行安装。

### 通过 Helm 安装

Helm 是 Kubernetes 的包管理器，Chart 则是 Helm 管理的包。你可以把它看作是 Homebrew、APT 或 YUM 在 Kubernetes 中的等价物。

安装 JuiceFS CSI 驱动需要用 Helm 3.1.0 及以上版本，请参照 [Helm 文档](https://helm.sh/docs/intro/install) 进行安装，并确保 `helm` 二进制能在 `PATH` 环境变量中找到。

1. 检查 kubelet 根目录

   执行以下命令

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   如果结果不为空或者 `/var/lib/kubelet`，则代表该集群的 kubelet 的根目录（`--root-dir`）做了定制，需要在 `values.yaml` 中将 `kubeletDir` 根据实际情况进行设置：

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

2. 部署

   执行以下命令部署 JuiceFS CSI 驱动。

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

3. 检查部署状态

   用下方命令确认 CSI 驱动组件正常运行：

   ```shell
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

   阅读[「架构」](./introduction.md)了解 CSI 驱动的架构，以及各组件功能。

### 通过 kubectl 安装

1. 检查 kubelet 根目录

   在 Kubernetes 集群中任意一个非 Master 节点上执行以下命令：

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. 部署

   - 如果上一步检查命令返回的结果不为空或者 `/var/lib/kubelet`，则代表该集群 kubelet 定制了根目录（`--root-dir`），因此需要在 CSI 驱动的部署文件中更新 kubelet 根目录路径：

     ```shell
     # 请将下述命令中的 {{KUBELET_DIR}} 替换成 kubelet 当前的根目录路径

     # Kubernetes 版本 >= v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

     # Kubernetes 版本 < v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

   - 如果上方检查命令返回的结果为空，则无需修改配置，直接部署：

     ```shell
     # Kubernetes 版本 >= v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

     # Kubernetes 版本 < v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
     ```

## 创建 StorageClass {#create-storage-class}

如果你打算以[「动态配置」](./guide/pv.md#dynamic-provisioning)的方式使用 JuiceFS CSI 驱动，那么你需要提前创建 StorageClass。

阅读[「使用方式」](./introduction.md#usage)以了解「动态配置」与「静态配置」的区别。

### Helm {#helm-sc}

创建 `values.yaml`，复制并完善下列配置信息。当前只列举出较为基础的配置，更多 JuiceFS CSI 驱动的 Helm chart 支持的配置项可以参考 [Values](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/README.md#values)。

JuiceFS 社区版和云服务的配置项略有不同，下方示范面向社区版，但你可以在 [values.yaml](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml#L121) 找到全面示范。

```yaml title="values.yaml"
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  # JuiceFS 文件系统相关配置
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

用 Helm 创建 StorageClass 时，挂载配置也会一并创建，请在 Helm 里直接管理，无需再[单独创建挂载配置](./guide/pv.md#create-mount-config)。

### 通过 kubectl 安装

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

### 调整挂载参数

如果需要调整挂载参数，可以在上方的 StorageClass 定义中追加 `mountOptions` 配置。如果需要为不同应用使用不同挂载参数，则需要创建多个 StorageClass，单独添加所需参数。

```yaml
mountOptions:
  - enable-xattr
  - max-uploads=50
  - cache-size=2048
  - cache-dir=/var/foo
  - allow_other
```

JuiceFS 社区版与云服务的挂载参数有所区别，请参考文档：

- [社区版](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount)
- [云服务](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)
