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

   如果结果不为空，则代表 kubelet 的根目录（`--root-dir`）不是默认值（`/var/lib/kubelet`），需要在第一步准备的配置文件 `values.yaml` 中将 `kubeletDir` 设置为 kubelet 当前的根目录路径：

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

2. 部署

   依次执行以下三条命令，通过 Helm 部署 JuiceFS CSI 驱动。如果没有准备 Helm 配置文件，在执行 `helm install` 命令时可以去掉最后的 `-f ./values.yaml` 选项。

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

3. 检查部署状态

   部署过程会启动一个名为 `juicefs-csi-controller` 的 `StatefulSet` 及一个 replica，以及一个名为 `juicefs-csi-node` 的 `DaemonSet`。执行命令 `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` 会看到有 `n+1` 个（`n` 指 Kubernetes 的 Node 数量）pod 在运行，例如：

   ```shell
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

### 通过 kubectl 安装

由于 Kubernetes 在版本变更过程中会废弃部分旧的 API，因此需要根据你使用 Kubernetes 版本选择适用的部署文件。

1. 检查 kubelet 根目录

   在 Kubernetes 集群中任意一个非 Master 节点上执行以下命令：

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. 部署

   - **如果上一步检查命令返回的结果不为空**，则代表 kubelet 的根目录（`--root-dir`）不是默认值（`/var/lib/kubelet`），因此需要在 CSI 驱动的部署文件中更新 kubelet 根目录路径后再部署：

     ```shell
     # 请将下述命令中的 {{KUBELET_DIR}} 替换成 kubelet 当前的根目录路径

     # Kubernetes 版本 >= v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

     # Kubernetes 版本 < v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

   - **如果前面检查命令返回的结果为空**，无需修改配置，可直接部署：

     ```shell
     # Kubernetes 版本 >= v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

     # Kubernetes 版本 < v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
     ```

## 创建 StorageClass {#create-storage-class}

如果你打算以[「动态配置」](./guide/pv.md#dynamic-provisioning)的方式使用 JuiceFS CSI 驱动，那么你需要提前创建 StorageClass。

### 通过 Helm 安装

创建配置文件 `values.yaml`，复制并完善下列配置信息。当前只列举出较为基础的配置，更多 JuiceFS CSI 驱动的 Helm chart 支持的配置项可以参考[文档](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/README.md#values)，不需要的项可以删除，或者留空。

JuiceFS 社区版和云服务的配置项略有不同，这里以社区版为例：

```yaml title="values.yaml"
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  backend:
    name: "<name>"                # JuiceFS 文件系统名
    metaurl: "<meta-url>"         # 元数据引擎的 URL
    storage: "<storage-type>"     # 对象存储类型 (例如 s3、gcs、oss、cos)
    accessKey: "<access-key>"     # 对象存储的 Access Key
    secretKey: "<secret-key>"     # 对象存储的 Secret Key
    bucket: "<bucket>"            # 存储数据的桶路径
    # 如果需要设置 JuiceFS Mount Pod 的时区请将下一行的注释符号删除，默认为 UTC 时间。
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

其中，`backend` 部分是 JuiceFS 文件系统相关的信息。如果使用的是已经提前创建好的 JuiceFS 文件系统，则只需填写 `name` 和 `metaurl` 这两项即可。

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
