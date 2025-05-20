---
title: 安装
---

安装前，请先确认：

* Kubernetes 集群是 1.14 及以上版本
* 集群能从外网拉取镜像，比如 [Docker Hub](https://hub.docker.com) 和 [Quay](https://quay.io)，如果无法从这两个镜像仓库下载资源，考虑先[「搬运镜像」](./administration/offline.md#copy-images)。

:::note 注意
在 JuiceFS 企业版私有部署环境下，CSI 驱动的安装并没有特殊步骤。不过请注意，由于使用私有部署控制台，你需要在[「文件系统认证信息」](./guide/pv.md#enterprise-edition)中需要填写 `envs` 字段，指定私有部署的控制台地址。
:::

## Helm {#helm}

相比 kubectl，Helm 允许你将 CSI 驱动中的各种资源、组件作为一个整体来管理，修改配置、启用高级特性，也只需要对 values 文件做少量编辑，无疑方便了许多，是我们更为推荐的安装方式。但如果你不熟悉 Helm，而且仅仅希望体验和评估 CSI 驱动，请参考下方的 [kubectl 安装方式](#kubectl)。

安装需要 Helm 3.1.0 及以上版本，请参照 [Helm 文档](https://helm.sh/zh/docs/intro/install)进行安装。

1. 加入 JuiceFS CSI 驱动的 Helm 仓库，并且创建出集群专属的配置文件，比方说当前集群名为 mycluster，那么推荐在 `values-mycluster.yaml` 中撰写该集群专属的配置。这份文件中的内容，会递归覆盖到原始的 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml)。

    ```shell
    helm repo add juicefs https://juicedata.github.io/charts/
    helm repo update

    mkdir juicefs-csi-driver && cd juicefs-csi-driver

    vi values-mycluster.yaml
    ```

1. 检查 kubelet 根目录

    在 Kubernetes 集群中任意一个非 Master 节点上执行以下命令：

    ```shell
    ps -ef | grep kubelet | grep root-dir
    ```

    如果结果不为空或者 `/var/lib/kubelet`，则代表该集群的 kubelet 的根目录（`--root-dir`）做了定制，需要在 values 中将 `kubeletDir` 根据实际情况进行设置：

    ```yaml title="values-mycluster.yaml"
    kubeletDir: <kubelet-dir>
    ```

1. 继续阅读 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml)，如果有其他需要修改的地方，一并在上方创建的 `values-mycluster.yaml` 中进行覆盖。常见需要根据集群调整的项目有：

    * 搜索 `repository` 字样，可选地调整各组件的镜像仓库，如果需要修改为集群私有镜像仓库，那么还伴随着[镜像搬运工作](./administration/offline.md)
    * 搜索 `resources` 字样，可选地调整各组件的资源占用

  修改结果示范：

   ```yaml title="values-mycluster.yaml"
   kubeletDir: <kubelet-dir>

   image:
     repository: registry.example.com/juicefs-csi-driver
   dashboardImage:
     repository: registry.example.com/csi-dashboard
   sidecars:
     livenessProbeImage:
       repository: registry.example.com/k8scsi/livenessprobe
     nodeDriverRegistrarImage:
       repository: registry.example.com/k8scsi/csi-node-driver-registrar
     csiProvisionerImage:
       repository: registry.example.com/k8scsi/csi-provisioner
     csiResizerImage:
       repository: registry.example.com/k8scsi/csi-resizer
   ```

1. 安装 CSI 驱动：

   ```shell
   # 不论是初次安装还是后续的配置变更，都可以运行这一行命令达到效果
   helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
   ```

推荐将上方的 values 文件进行源码管理，这样一来就算配置不断变化，也能对其进行追溯和回滚。

## kubectl {#kubectl}

kubectl 是较为简单直接的安装方式，如果你只是希望体验和评估 CSI 驱动，推荐这种安装方式，**但在生产环境则不推荐这样安装**：用 kubectl 直接安装的话，意味着后续对 CSI 驱动的任何配置修改都需要手动操作，若不熟悉极容易出错。如果你希望开启某些 CSI 驱动的高级特性（例如[「启用 pathPattern」](./guide/configurations.md#using-path-pattern)），或者想要更加体系化地管理资源，请优先选用 [Helm 安装方式](#helm)。

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

CSI Node Service 是一个 DaemonSet，默认在所有节点部署，因此在上方命令的输出中，CSI Node Pod 数量应该与 worker 节点数相同。如果你注意到数量不一致，请检查是否有节点被打上了污点。视情况删除污点，或给 CSI Node Service 打上对应的容忍，来修复此问题。如果你有需要，也可以[仅在某些节点上运行 CSI Node Service](./guide/resource-optimization.md#csi-node-node-selector)。

如果你对各组件功能仍有疑惑，请详读[「架构」](./introduction.md#architecture)。

## 以 Sidecar 模式安装 {#sidecar}

Sidecar 与默认的容器挂载方式有很大不同，包括无法复用挂载客户端，以及无法设置[挂载点自动恢复](./guide/configurations.md#automatic-mount-point-recovery)。决定采纳之前，务必仔细阅读[「Sidecar 模式注意事项」](./introduction.md#sidecar)。

### Helm

:::tip Serverless 注意事项
从 v0.23.5 开始，Helm chart 支持名为 `mountMode: serverless` 的特殊模式。这种模式与 sidecar 相同，但移除了各种 serverless 环境中不支持的配置，比如 hostPath 挂载点，以及 privileged 权限。

`serverless` 模式将允许在 serverless 虚拟节点上安装 JuiceFS CSI 驱动，不再需要一个实际节点。
:::

在 values 中修改配置：

```yaml title='values-mycluster.yaml'
mountMode: sidecar
```

若集群中使用 [CertManager](https://github.com/cert-manager/cert-manager) 管理证书，需要在安装配置中添加如下内容：

```yaml title='values-mycluster.yaml'
mountMode: sidecar
webhook:
  certManager:
    enabled: true
```

重新安装，令配置生效：

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
```

:::warning
安装以后，必须等待所有组件运行正常以后，才能继续执行下一步打标签。如果 Controller 容器尚未健康运行，就为命名空间打好标签，该命名空间的所有 Pod 都将无法创建，卡死在 Webhook 的注入检查这一关。
:::

对所有需要使用 JuiceFS CSI 驱动的命名空间打上下述标签，需要注意的是普通集群和 Serverless 的标签有所不同：

```shell
# 普通集群
kubectl label namespace $NS juicefs.com/enable-injection=true --overwrite
# Serverless 集群
kubectl label namespace $NS juicefs.com/enable-serverless-injection=true --overwrite
```

### kubectl

考虑到安装文件需要用脚本生成，不便于源码管理、以及未来升级 CSI 驱动时的配置梳理，生产环境不建议用 kubectl 进行安装。

```shell
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

:::warning
安装以后，必须等待所有组件运行正常以后，才能继续执行下一步打标签。如果 Controller 容器尚未健康运行，就为命名空间打好标签，该命名空间的所有 Pod 都将无法创建，卡死在 Webhook 的注入检查这一关。
:::

对所有需要使用 JuiceFS CSI 驱动的命名空间打上下述标签，需要注意的是普通集群和 Serverless 的标签有所不同：

```shell
# 普通集群
kubectl label namespace $NS juicefs.com/enable-injection=true --overwrite
# Serverless 集群
kubectl label namespace $NS juicefs.com/enable-serverless-injection=true --overwrite
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

在进程挂载模式下，JuiceFS 客户端不再运行在独立的 Pod 中，而是运行在 CSI Node Service 容器中，所有需要挂载的 JuiceFS PV 都会在 CSI Node Service 容器中以进程模式挂载。详情可以参考[「进程挂载模式」](./introduction.md#by-process)。

### Helm

在安装配置中修改对应字段：

```YAML title='values-mycluster.yaml'
mountMode: process
```

重新安装，令配置生效：

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
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

在 `values-mycluster.yaml` 中增加 `sidecars` 配置，用于覆盖容器镜像：

```yaml title="values-mycluster.yaml"
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

## 卸载

删除是安装的逆向操作，对于 Helm 安装的，执行以下命令即可：

```shell
helm uninstall juicefs-csi-driver
```

如果使用的是 kubectl 安装方式，只需将相应安装命令中的 `apply` 替换为 `delete` 即可，例如：

```shell
kubectl delete -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```
