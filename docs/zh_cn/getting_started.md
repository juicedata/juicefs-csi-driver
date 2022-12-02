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

   执行以下命令部署 JuiceFS CSI 驱动：

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

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

### 检查部署状态

不论你用何种方法，安装完毕以后，请用下方命令确认 CSI 驱动组件正常运行：

```shell
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

CSI Node Service 是一个 DaemonSet，默认在所有节点部署，因此在上方命令的输出中，CSI Node pod 数量应该与 worker 节点数相同。如果你注意到数量不一致，请检查是否有节点被打上了污点。视情况删除污点，或给 CSI Node Service 打上对应的容忍，来修复此问题。如果你有需要，也可以[仅在某些节点上运行 CSI Node Service](./guide/resource-optimization.md#csi-node-node-selector)。

如果你对各组件功能仍有疑惑，请详读[「架构」](./introduction.md#architecture)。
