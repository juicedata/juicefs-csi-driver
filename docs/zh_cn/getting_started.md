---
title: 安装
---

安装前，请先确认：

* Kubernetes 集群是 1.14 及以上版本
* 集群能从外网拉取镜像，比如 [Docker Hub](https://hub.docker.com) 和 [Quay](https://quay.io)，如果无法从这两个镜像仓库下载资源，考虑先[「搬运镜像」](./administration/offline.md#copy-images)。

:::note 注意
在 JuiceFS 企业版私有部署环境下，CSI 驱动的安装并没有特殊步骤。不过请注意，由于使用私有部署控制台，你需要在[「文件系统认证信息」](./guide/pv.md#enterprise-edition)中需要填写 `envs` 字段，指定私有部署的控制台地址。
:::

## Helm

相比 kubectl，Helm 允许你将 CSI 驱动中的各种资源、组件作为一个整体来管理，修改配置、启用高级特性，也只需要对 `values.yaml` 做少量编辑，无疑方便了许多，是我们更为推荐的安装方式。但如果你不熟悉 Helm，而且仅仅希望体验和评估 CSI 驱动，请参考下方的 [kubectl 安装方式](#kubectl)。

安装需要 Helm 3.1.0 及以上版本，请参照 [Helm 文档](https://helm.sh/zh/docs/intro/install)进行安装。

1. 下载 JuiceFS CSI 驱动的 Helm chart

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm fetch --untar juicefs/juicefs-csi-driver
   cd juicefs-csi-driver
   # values.yaml 中包含安装 CSI 驱动的所有配置，安装前可以进行梳理，并按需修改
   cat values.yaml
   ```

1. 检查 kubelet 根目录

   执行以下命令

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   如果结果不为空或者 `/var/lib/kubelet`，则代表该集群的 kubelet 的根目录（`--root-dir`）做了定制，需要在 `values.yaml` 中将 `kubeletDir` 根据实际情况进行设置：

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

1. 安装 CSI 驱动：

   ```shell
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

我们推荐将 CSI 驱动的 Helm chart 纳入版本控制系统管理。这样一来，就算 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml) 中的配置不断变化，也能对其进行追溯和回滚。

## kubectl

kubectl 是较为简单直接的安装方式，如果你只是希望体验和评估 CSI 驱动，推荐这种安装方式，**但在生产环境则不推荐这样安装**：用 kubectl 直接安装的话，意味着后续对 CSI 驱动的任何配置修改都需要手动操作，若不熟悉极容易出错。如果你希望开启某些 CSI 驱动的高级特性（例如[「启用 pathPattern」](./guide/pv.md#using-path-pattern)），或者想要更加体系化地管理资源，请优先选用 [Helm 安装方式](#helm)。

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

## 检查部署状态 {#verify-installation}

用下方命令确认 CSI 驱动组件正常运行：

```shell
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

CSI Node Service 是一个 DaemonSet，默认在所有节点部署，因此在上方命令的输出中，CSI Node pod 数量应该与 worker 节点数相同。如果你注意到数量不一致，请检查是否有节点被打上了污点。视情况删除污点，或给 CSI Node Service 打上对应的容忍，来修复此问题。如果你有需要，也可以[仅在某些节点上运行 CSI Node Service](./guide/resource-optimization.md#csi-node-node-selector)。

如果你对各组件功能仍有疑惑，请详读[「架构」](./introduction.md#architecture)。

## 以 Sidecar 模式安装 {#sidecar}

### Helm

在 `values.yaml` 中修改配置：

```yaml title='values.yaml'
mountMode: sidecar
```

若集群中使用 [CertManager](https://github.com/cert-manager/cert-manager) 管理证书，需要在 `values.yaml` 中添加如下配置：

```yaml title='values.yaml'
mountMode: sidecar
webhook:
   certManager:
      enabled: true
```

重新安装，令配置生效：

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

对所有需要使用 JuiceFS CSI 驱动的命名空间打上该标签：

```shell
kubectl label namespace $NS juicefs.com/enable-injection=true --overwrite
```

### kubectl

考虑到安装文件需要用脚本生成，不便于源码管理、以及未来升级 CSI 驱动时的配置梳理，生产环境不建议用 kubectl 进行安装。

```shell
# 对所有需要使用 JuiceFS CSI 驱动的命名空间打上该标签
kubectl label namespace $NS juicefs.com/enable-injection=true --overwrite

# Sidecar 模式需要在安装过程中生成和使用证书，渲染对应的 YAML 资源，请直接使用安装脚本
wget https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/juicefs-csi-webhook-install.sh
chmod +x ./juicefs-csi-webhook-install.sh

# 用脚本生成安装文件
./juicefs-csi-webhook-install.sh print > juicefs-csi-sidecar.yaml

# 对该文件配置进行梳理，然后安装
kubectl apply -f ./juicefs-csi-sidecar.yaml
```

也可以用一行命令进行更快速的直接安装：

```shell
./juicefs-csi-webhook-install.sh install
```

若集群中使用 [CertManager](https://github.com/cert-manager/cert-manager) 管理证书，可以使用下方命令生成安装文件或直接安装：

```shell
# 生成配置文件
./juicefs-csi-webhook-install.sh print --with-certmanager > juicefs-csi-sidecar.yaml
kubectl apply -f ./juicefs-csi-sidecar.yaml

# 一键安装
./juicefs-csi-webhook-install.sh install --with-certmanager 
```

如果你不得不在生产集群使用此种方式进行安装，那么一定要将生成的 `juicefs-csi-sidecar.yaml` 进行源码管理，方便追踪配置变更的同时，也方便未来升级 CSI 驱动时，进行配置对比梳理。

## 以进程挂载模式安装 {#by-process}

### Helm

在 `values.yaml` 中修改配置：

```YAML title='values.yaml'
mountMode: process
```

重新安装，令配置生效：

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### kubectl

在 CSI Node Service 和 CSI Controller 的启动参数中添加 `--by-process=true`，就能启用进程挂载模式。

## 安装在 ARM64 环境 {#arm64}

CSI 驱动在 v0.11.1 及之后版本支持 ARM64 环境的容器镜像，如果你的集群是 ARM64 架构，需要在执行安装前，更换部分容器镜像，其他安装步骤都相同。

需要替换的镜像如下，请通过下方链接的网页，确定各镜像合适的版本（如果无法正常访问 `k8s.gcr.io`，请考虑先[「搬运镜像」](./administration/offline.md#copy-images)）：

| 原镜像名称                                      | 新镜像名称                                                                                                                                          |
|--------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `quay.io/k8scsi/livenessprobe`             | [`registry.k8s.io/sig-storage/livenessprobe`](https://kubernetes-csi.github.io/docs/livenessprobe.html#supported-versions)                     |
| `quay.io/k8scsi/csi-provisioner`           | [`registry.k8s.io/sig-storage/csi-provisioner`](https://kubernetes-csi.github.io/docs/external-provisioner.html#supported-versions)            |
| `quay.io/k8scsi/csi-node-driver-registrar` | [`registry.k8s.io/sig-storage/csi-node-driver-registrar`](https://kubernetes-csi.github.io/docs/node-driver-registrar.html#supported-versions) |
| `quay.io/k8scsi/csi-resizer:`              | [`registry.k8s.io/sig-storage/csi-resizer`](https://kubernetes-csi.github.io/docs/external-resizer.html#supported-versions)                    |

### Helm

在 `values.yaml` 中增加 `sidecars` 配置，用于覆盖容器镜像：

```yaml title="values.yaml"
sidecars:
  livenessProbeImage:
    repository: registry.k8s.io/sig-storage/livenessprobe
    tag: "v2.6.0"
  csiProvisionerImage:
    repository: registry.k8s.io/sig-storage/csi-provisioner
    tag: "v2.2.2"
  nodeDriverRegistrarImage:
    repository: registry.k8s.io/sig-storage/csi-node-driver-registrar
    tag: "v2.5.0"
  csiResizerImage:
    repository: registry.k8s.io/sig-storage/csi-resizer
    tag: "v1.8.0"
```

### kubectl

对 `k8s.yaml` 中部分镜像以及 `provisioner` sidecar 的启动参数进行替换（macOS 请换用 [gnu-sed](https://formulae.brew.sh/formula/gnu-sed)）：

```shell
sed --in-place --expression='s@quay.io/k8scsi/livenessprobe:v1.1.0@registry.k8s.io/sig-storage/livenessprobe:v2.6.0@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-provisioner:v1.6.0@registry.k8s.io/sig-storage/csi-provisioner:v2.2.2@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.5.0@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-resizer:v1.0.1@registry.k8s.io/sig-storage/csi-resizer:v1.8.0@' k8s.yaml
sed --in-place --expression='s@enable-leader-election@leader-election@' k8s.yaml
```
