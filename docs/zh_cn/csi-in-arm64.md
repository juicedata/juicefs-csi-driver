---
sidebar_label: 在 arm64 中安装
---

# 如何在 arm64 环境下安装 JuiceFS CSI Driver

JuiceFS CSI Driver 在 v0.11.1 及之后版本才支持 arm64 环境的镜像。以下提供了两种安装 JuiceFS CSI Driver 的方式。

## 方法一：通过 Helm 安装

### 版本要求

- Helm 3.1.0+

### 安装 Helm

请参照 [Helm 文档](https://github.com/helm/helm#install) 进行安装。

### 安装 JuiceFS CSI Driver

1. 准备配置文件

创建一个配置文件，例如：`values.yaml`，复制并完善下列配置信息。其中，`backend` 部分是 JuiceFS
文件系统相关的信息，你可以参照 [JuiceFS 快速上手指南](https://github.com/juicedata/juicefs/blob/main/docs/zh_cn/quick_start_guide.md)了解相关内容。如果使用的是已经提前创建好的
JuiceFS 卷，则只需填写 `name` 和 `metaurl` 这两项即可。`mountPod` 部分可以对使用此驱动的 Pod 设置 CPU 和内存的资源配置。不需要的项可以删除，或者将它的值留空。

```yaml
sidecars:
  livenessProbeImage:
    repository: k8s.gcr.io/sig-storage/livenessprobe
    tag: "v2.2.0"
  nodeDriverRegistrarImage:
    repository: k8s.gcr.io/sig-storage/csi-node-driver-registrar
    tag: "v2.0.1"
  csiProvisionerImage:
    repository: k8s.gcr.io/sig-storage/csi-provisioner
    tag: "v2.0.2"
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

- **检查 Pods**：部署过程会启动一个名为 `juicefs-csi-controller` 的 `StatefulSet` 及一个 replica，以及一个名为 `juicefs-csi-node` 的 `DaemonSet`
  。执行命令 `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` 会看到有 `n+1` 个 pod 在运行，例如：

```sh
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

- **检查 secret**：通过命令 `kubectl -n kube-system describe secret juicefs-sc-secret` 可以看到前面 `values.yaml` 配置文件中 `backend` 部分的
  secret 信息。

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

## 方法二：通过 kubectl 安装

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
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | \
sed -e 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' \
-e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
-e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
-e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | \
sed -e 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' \
-e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
-e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
-e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -
```

> **注意**: 请将上述命令中 `{{KUBELET_DIR}}` 替换成 kubelet 当前的根目录路径。

**如果前面检查命令返回的结果为空**，无需修改配置，可直接部署：

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | \
sed -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
-e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
-e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | \
sed -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
-e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
-e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -
```
