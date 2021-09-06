# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

[JuiceFS](https://github.com/juicedata/juicefs) CSI 驱动遵循 [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) 规范，实现了容器编排系统与 JuiceFS 文件系统之间的接口，支持动态配置 JuiceFS 卷提供给 Pod 使用。

## 版本要求

- Kubernetes 1.14+

## 安装

以下提供了两种安装 JuiceFS CSI Driver 的方式。

### 方法一：通过 Helm 安装

#### 版本要求

- Helm 3.1.0+

#### 安装 Helm

Helm 是 Kubernetes 的包管理器，Chart 是 Helm 管理的包。你可以把它看作是 Homebrew formula，Apt dpkg，或 Yum RPM 在 Kubernetes 中的等价物。

请参照 [Helm 文档](https://github.com/helm/helm#install) 进行安装。

#### 安装 JuiceFS CSI Driver

1. 准备配置文件

创建一个配置文件，例如：`values.yaml`，复制并完善下列配置信息。其中，`backend` 部分是 JuiceFS 文件系统相关的信息，你可以参照 [JuiceFS 快速上手指南](https://github.com/juicedata/juicefs/blob/main/docs/zh_cn/quick_start_guide.md)了解相关内容。如果使用的是已经提前创建好的 JuiceFS 卷，则只需填写 `name` 和 `metaurl` 这两项即可。`mountPod` 部分可以对使用此驱动的 Pod 设置 CPU 和内存的资源配置。不需要的项可以删除，或者将它的值留空。

```yaml
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  backend:
    name: "<name>"
    metaurl: "<meta-url>"
    storage: "<storage-type>"
    accessKey: "<access-key>"
    secretKey: "<secret-key>"
    bucket: "<bucket>"
  mountPod:
    resources:
      limits:
        cpu: "<cpu-limit>"
        memory: "<memory-limit>"
      requests:
        cpu: "<cpu-request>"
        memory: "<memory-request>"
```

2. 部署

依次执行以下三条命令，通过 helm 部署 JuiceFS CSI Driver。

```sh
$ helm repo add juicefs-csi-driver https://juicedata.github.io/juicefs-csi-driver/
$ helm repo update
$ helm upgrade juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver --install -f ./values.yaml
```

3. 检查部署状态

- **检查 Pods**：部署过程会启动一个名为 `juicefs-csi-controller` 的 `StatefulSet` 及一个 replica，以及一个名为 `juicefs-csi-node` 的 `DaemonSet`。执行命令 `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` 会看到有 `n+1` 个 pod 在运行，例如：

```sh
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

- **检查 secret**：通过命令 `kubectl -n kube-system describe secret juicefs-sc-secret` 可以看到前面 `values.yaml` 配置文件中 `backend` 部分的 secret 信息。

```
Name:         juicefs-sc-secret
Namespace:    kube-system
Labels:       app.kubernetes.io/instance=juicefs-csi-driver
              app.kubernetes.io/managed-by=Helm
              app.kubernetes.io/name=juicefs-csi-driver
              app.kubernetes.io/version=0.7.0
              helm.sh/chart=juicefs-csi-driver-0.1.0
Annotations:  meta.helm.sh/release-name: juicefs-csi-driver
              meta.helm.sh/release-namespace: default

Type:  Opaque

Data
====
access-key:  0 bytes
bucket:      47 bytes
metaurl:     54 bytes
name:        4 bytes
secret-key:  0 bytes
storage:     2 bytes
```

- **检查存储类（storage class）**：通过命令 `kubectl get sc juicefs-sc` 会看到类似下面的存储类信息。

```
NAME         PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
juicefs-sc   csi.juicefs.com   Retain          Immediate           false                  69m
```

### 方法二：通过 kubectl 安装

由于 Kubernetes 在版本变更过程中会废弃部分旧的 API，因此需要根据你使用 Kubernetes 版本选择适用的部署文件：

#### Kubernetes v1.18 及以上版本

```shell
$ kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```

#### Kubernetes v1.18 以下版本

```shell
$ kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
```

#### 故障排查

如果 Kubernetes 无法发现 CSI 驱动并返回类似下面的错误：

```
driver name csi.juicefs.com not found in the list of registered CSI drivers, check the root directory path of `kubelet`.
```

请尝试在集群中任何一个非 master 节点执行命令：

```shell
$ ps -ef | grep kubelet | grep root-dir
```

如果结果不为空，请手动修改 CSI 驱动的部署文件 `k8s.yaml`，替换其中的 Kubelet 根目录，然后重新进行部署。

```shell
$ curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> 注意：请将上述命令中 `{{KUBELET_DIR}}` 替换成 kubelet 实际的根目录路径。

## 升级 CSI Driver

### CSI Driver v0.10 及以上版本

Juicefs CSI Driver 从 v0.10.0 开始分离了 JuiceFS client 客户端，升级 CSI Driver 不会影响已存在的 PV。如果你使用的是 CSI Driver v0.10.0 及以上的版本，执行以下命令进行升级：

* 如果您使用的是 `latest` 标签，只需运行 `kubectl rollout restart -f k8s.yaml` 并确保重启 `juicefs-csi-controller` 和 `juicefs-csi-node` pod。
* 如果您已固定到特定版本，请将您的 `k8s.yaml` 修改为要更新的版本，然后运行 `kubectl apply -f k8s.yaml`。
* 如果你的 JuiceFS CSI Driver 是使用 Helm 安装的，也可以通过 Helm 对其进行升级。

### CSI Driver v0.10 以下版本

#### 小版本升级

升级 CSI Driver 需要重启 `DaemonSet`。由于 v0.10.0 之前的版本所有的 JuiceFS 客户端都运行在 `DaemonSet` 中，重启的过程中相关的 PV 都将不可用，因此需要先停止相关的 pod。

1. 停止所有使用此驱动的 pod。
2. 升级驱动：
	* 如果您使用的是 `latest` 标签，只需运行 `kubectl rollout restart -f k8s.yaml` 并确保重启 `juicefs-csi-controller` 和 `juicefs-csi-node` pod。
	* 如果您已固定到特定版本，请将您的 `k8s.yaml` 修改为要更新的版本，然后运行 `kubectl apply -f k8s.yaml`。
  * 如果你的 JuiceFS CSI Driver 是使用 Helm 安装的，也可以通过 Helm 对其进行升级。
3. 启动 pod。

#### 跨版本升级

如果你想从 CSI Driver v0.9.0 升级到 v0.10.0 及以上版本，请参考[这篇文档](./docs/upgrade-csi-driver.md)。

#### 其他

对于使用较低版本的用户，你还可以在不升级 CSI 驱动的情况下升级 JuiceFS 客户端，详情参考[这篇文档](./docs/upgrade-juicefs.md)。

访问 [Docker Hub](https://hub.docker.com/r/juicedata/juicefs-csi-driver) 查看更多版本信息。

## 示例

开始之前，你需要：

* 了解如何设置 Kubernetes 和 [JuiceFS](https://github.com/juicedata/juicefs)
* 确保 JuiceFS 能够被 Kuberenetes 集群访问。建议在与 Kubernetes 集群相同的区域创建文件系统。
* 参照[说明](#installation)安装 JuiceFS CSI driver。

### 目录

* [Basic](examples/basic)
* [Static provisioning](examples/static-provisioning/)
  * [Mount options](examples/static-provisioning-mount-options/)
  * [Read write many](examples/static-provisioning-rwx/)
  * [Sub path](examples/static-provisioning-subpath/)
  * [Mount resources](examples/static-provisioning-mount-resources/)
  * [Config and env](examples/static-provisioning-config-and-env/)
* [Dynamic provisioning](examples/dynamic-provisioning/)

**备注**:

* 由于 JuiceFS 是一个弹性文件系统，它不需要强制分配容量。你在 PersistentVolume 和 PersistentVolumeClaim 中指定的容量并是实际存储容量。但是，由于存储容量是 Kubernetes 的必填字段，因此您可以使用任何有效值，例如 `10Pi` 表示容量。
* 一些示例需要使用 kustomize 3.x。

## CSI 规格兼容性

| JuiceFS CSI Driver \ CSI Version | v0.3 | v1.0 |
| -------------------------------- | ---- | ---- |
| master branch                    | no   | yes  |

### 接口

以下是已经实现的 CSI 接口：

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

## 故障排查

请参考 [Troubleshooting](docs/troubleshooting.md) 文档。

## Kubernetes

以下内容是特别针对 Kubernetes 的。

### Kubernetes 版本兼容性

JuiceFS CSI Driver 兼容 Kubernetes **v1.14+**

容器镜像

| JuiceFS CSI Driver Version | Image                               |
| -------------------------- | ----------------------------------- |
| master branch              | juicedata/juicefs-csi-driver:latest |

### 特征

* **静态配置** - 首先需要手动创建 JuiceFS 文件系统，然后可以使用驱动程序将其作为 PersistentVolume (PV) 挂载到容器内。
* **挂载选项** - CSI 卷属性可以在 PersistentVolume (PV) 中指定，以定义卷应如何挂载。
* **多机读写** - 支持 `ReadWriteMany` 访问模式
* **子路径** - 在 JuiceFS 文件系统中为 PersistentVolume 提供子路径
* **挂载资源** - 可以在 PersistentVolume (PV) 中指定 CSI 卷属性来定义挂载 pod 的 CPU/内存限制/请求。
* **挂载 pod 中的配置文件和环境变量** - 支持在挂载 pod 中设置配置文件和环境变量。
* **动态配置** - 允许按需动态创建存储卷

### 版本验证

JuiceFS CSI driver 已在下列 Kubernetes 版本中验证：

| Kubernetes                 | master |
| -------------------------- | ------ |
| v1.19.2 / minikube v1.16.0 | Yes    |
| v1.20.2 / minikube v1.16.0 | Yes    |

### 已知问题

- JuiceFS CSI 驱动程序 (>=v0.10.0) 中的挂载选项 `--cache-dir` 目前不支持通配符。

## 其他资源

- [Access ceph cluster with librados](./docs/ceph.md)

## 开发

请查阅 [DEVELOP](./docs/DEVELOP.md) 文档。

## 授权许可

该库在 Apache 2.0 许可下授权。
