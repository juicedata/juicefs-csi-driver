---
sidebar_label: 在 ARM64 环境中安装
---

# 如何在 ARM64 环境下安装 JuiceFS CSI Driver

JuiceFS CSI Driver 在 v0.11.1 及之后版本才支持 ARM64 环境的容器镜像，因此请确保你使用的是正确的版本。相比[「介绍」](introduction.md)文档中的安装方法，在 ARM64 环境中的稍有不同，下面分别针对不同的安装方式进行介绍。

## 方法一：通过 Helm 安装

:::note 注意
请使用 v0.7.1 及之后版本的 Helm chart 进行安装
:::

在 ARM64 环境中安装最主要的区别是[「第 1 步准备配置文件」](introduction.md#安装-juicefs-csi-驱动)，需要在 YAML 文件中新增 `sidecars` 配置，具体内容如下：

```yaml {1-10}
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

之后的安装步骤请按照[「介绍」](introduction.md#方法一通过-helm-安装)文档中的说明进行。

## 方法二：通过 kubectl 安装

在 ARM64 环境中安装最主要的区别是[「第 2 步部署」](introduction.md#方法二通过-kubectl-安装)，需要替换几个 sidecar 容器的镜像地址。假设已经将 [`k8s.yaml`](https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml) 文件下载到本地目录，具体命令如下：

```shell
cat ./k8s.yaml | \
sed -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
-e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
-e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | \
kubectl apply -f -
```

其它安装步骤请按照[「介绍」](introduction.md#方法二通过-kubectl-安装)文档中的说明进行。
