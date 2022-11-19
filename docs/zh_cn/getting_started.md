---
sidebar_label: 快速上手
---

# 快速上手

## 版本要求

- Kubernetes 1.14 及以上

## 安装

以下提供了两种安装 JuiceFS CSI 驱动的方式。

### 方法一：通过 Helm 安装

#### 版本要求

- Helm 3.1.0 及以上

#### 安装 Helm

Helm 是 Kubernetes 的包管理器，Chart 是 Helm 管理的包。你可以把它看作是 Homebrew formula，APT dpkg，或 YUM RPM 在 Kubernetes 中的等价物。

请参照 [Helm 文档](https://helm.sh/docs/intro/install) 进行安装，并确保 `helm` 二进制能在 `PATH` 环境变量中找到。

#### 安装 JuiceFS CSI 驱动

1. 准备配置文件

   :::info 说明
   若您不需要在安装 CSI 驱动时创建 StorageClass，可以忽略此步骤。
   :::

   创建一个配置文件（例如 `values.yaml`），复制并完善下列配置信息。当前只列举出较为基础的配置，更多 JuiceFS CSI 驱动的 Helm chart 支持的配置项可以参考[文档](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/README.md#values)，不需要的项可以删除，或者将它的值留空。

   这里以社区版为例：

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

   其中，`backend` 部分是 JuiceFS 文件系统相关的信息。如果使用的是已经提前创建好的 JuiceFS 卷，则只需填写 `name` 和 `metaurl` 这两项即可。更加详细的 StorageClass 使用方式可参考文档：[动态配置](./examples/dynamic-provisioning.md)。

2. 检查 kubelet 根目录

   执行以下命令

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   如果结果不为空，则代表 kubelet 的根目录（`--root-dir`）不是默认值（`/var/lib/kubelet`），需要在第一步准备的配置文件 `values.yaml` 中将 `kubeletDir` 设置为 kubelet 当前的根目录路径：

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

3. 部署

   依次执行以下三条命令，通过 Helm 部署 JuiceFS CSI 驱动。如果没有准备 Helm 配置文件，在执行 `helm install` 命令时可以去掉最后的 `-f ./values.yaml` 选项。

   ```sh
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

4. 检查部署状态

   部署过程会启动一个名为 `juicefs-csi-controller` 的 `StatefulSet` 及一个 replica，以及一个名为 `juicefs-csi-node` 的 `DaemonSet`。执行命令 `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` 会看到有 `n+1` 个（`n` 指 Kubernetes 的 Node 数量）pod 在运行，例如：

   ```sh
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

### 方法二：通过 kubectl 安装

由于 Kubernetes 在版本变更过程中会废弃部分旧的 API，因此需要根据你使用 Kubernetes 版本选择适用的部署文件。

1. 检查 kubelet 根目录

   在 Kubernetes 集群中任意一个非 Master 节点上执行以下命令：

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. 部署

   - **如果上一步检查命令返回的结果不为空**，则代表 kubelet 的根目录（`--root-dir`）不是默认值（`/var/lib/kubelet`），因此需要在 CSI 驱动的部署文件中更新 `kubeletDir` 路径并部署：

     :::note 注意
     请将下述命令中的 `{{KUBELET_DIR}}` 替换成 kubelet 当前的根目录路径。
     :::

     ```shell
     # Kubernetes 版本 >= v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

     ```shell
     # Kubernetes 版本 < v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

   - **如果前面检查命令返回的结果为空**，无需修改配置，可直接部署：

     ```shell
     # Kubernetes 版本 >= v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
     ```

     ```shell
     # Kubernetes 版本 < v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
     ```

## 使用

请参考左侧侧边栏的「使用指南」分类

## 故障排查

请参考 [故障排查](troubleshooting.md) 或 [FAQ](FAQs.md) 文档。

## 升级 CSI 驱动

请参考 [升级 JuiceFS CSI 驱动](./administration/upgrade-csi-driver.md) 文档

## 已知问题

- JuiceFS CSI 驱动 v0.10.0 及以上版本不支持在 `--cache-dir` 挂载选项中使用通配符
