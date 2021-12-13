# JuiceFS CSI Driver

[![Build Status](https://travis-ci.com/juicedata/juicefs-csi-driver.svg?token=ACsZ5AkewTgk5D5wzzds&branch=master)](https://travis-ci.com/juicedata/juicefs-csi-driver)

[English](./README.md) | 简体中文

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


2. 检查 kubelet root-dir

执行以下命令

```shell
$ ps -ef | grep kubelet | grep root-dir
```

如果结果不为空，则代表 kubelet 的 root-dir 路径不是默认值，需要在第一步准备的配置文件 `values.yaml` 中将 `kubeletDir` 设置为 kubelet 当前的 root-dir 路径：

```yaml
kubeletDir: <kubelet-dir>
```

3. 部署

依次执行以下三条命令，通过 helm 部署 JuiceFS CSI Driver。

```sh
$ helm repo add juicefs-csi-driver https://juicedata.github.io/juicefs-csi-driver/
$ helm repo update
$ helm install juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver -n kube-system -f ./values.yaml
```

4. 检查部署状态

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

1. 检查 `kubelet root-dir` 路径

在 Kubernetes 集群中任意一个非 Master 节点上执行以下命令：

```shell
$ ps -ef | grep kubelet | grep root-dir
```

2. 部署

**如果前面检查命令返回的结果不为空**，则代表 kubelet 的 root-dir 路径不是默认值，因此需要在 CSI Driver 的部署文件中更新 `kubeletDir` 路径并部署：

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> **注意**: 请将上述命令中 `{{KUBELET_DIR}}` 替换成 kubelet 当前的根目录路径。

**如果前面检查命令返回的结果为空**，无需修改配置，可直接部署：

```shell
# Kubernetes version >= v1.18
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

# Kubernetes version < v1.18
kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
```

## 故障排查

请参考 [Troubleshooting](docs/zh_cn/troubleshooting.md) 或 [FAQs](docs/zh_cn/FAQs.md) 文档。

## 升级 CSI Driver

请参考 [CSI Driver](docs/zh_cn/upgrade-csi-driver.md) 文档

## 示例

开始之前，你需要：

* 了解如何设置 Kubernetes 和 [JuiceFS](https://github.com/juicedata/juicefs)
* 确保 JuiceFS 能够被 Kuberenetes 集群访问。建议在与 Kubernetes 集群相同的区域创建文件系统。
* 参照[说明](#安装)安装 JuiceFS CSI driver。

### 目录

* [Static provisioning](docs/zh_cn/examples/static-provisioning.md)
* [Dynamic provisioning](docs/zh_cn/examples/dynamic-provisioning.md)
* [Mount options](docs/zh_cn/examples/mount-options.md)
* [ReadWriteMany and ReadOnlyMany](docs/zh_cn/examples/rwx-and-rox.md)
* [Sub path](docs/zh_cn/examples/subpath.md)
* [Mount resources](docs/zh_cn/examples/mount-resources.md)
* [Config and env](docs/zh_cn/examples/config-and-env.md)

**备注**:

* 由于 JuiceFS 是一个弹性文件系统，它不需要强制分配容量。你在 PersistentVolume 和 PersistentVolumeClaim 中指定的容量并不是实际存储容量。但是，由于存储容量是 Kubernetes 的必填字段，因此您可以使用任何有效值，例如 `10Pi` 表示容量。
* 一些示例需要使用 kustomize 3.x。

## CSI 规格兼容性

| JuiceFS CSI Driver \ CSI Version | v0.3 | v1.0 |
| -------------------------------- | ---- | ---- |
| master branch                    | no   | yes  |

### 接口

以下是已经实现的 CSI 接口：

* Node Service: NodePublishVolume, NodeUnpublishVolume, NodeGetCapabilities, NodeGetInfo, NodeGetId
* Identity Service: GetPluginInfo, GetPluginCapabilities, Probe

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

### 已知问题

- JuiceFS CSI 驱动程序 (>=v0.10.0) 中的挂载选项 `--cache-dir` 目前不支持通配符。

## 其他资源

- [通过 librados 访问 Ceph 集群](docs/zh_cn/ceph.md)

## 授权许可

该库在 Apache 2.0 许可下授权。
