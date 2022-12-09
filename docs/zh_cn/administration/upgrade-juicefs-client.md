---
title: 升级 JuiceFS 客户端
slug: /upgrade-juicefs-client
sidebar_position: 3
---

我们推荐你定期升级 JuiceFS 客户端，以享受到最新特性和问题修复，请参考[「社区版客户端发布说明」](https://github.com/juicedata/juicefs/releases)或[「云服务客户端发布说明」](https://juicefs.com/docs/zh/cloud/release)了解更多版本信息。事实上，[「升级 JuiceFS CSI 驱动」](./upgrade-csi-driver.md)也可能会带来客户端更新，但如果你不希望升级 CSI 驱动，可以用本章介绍的方法单独升级 JuiceFS 客户端。

## 升级 Mount Pod 容器镜像 {#upgrade-mount-pod-image}

在 v0.17.1 及以上版本，CSI 驱动允许用户自行设置 Mount Pod 容器镜像，你可以修改配置，使用新版的 Mount Pod 容器镜像，来实现升级 JuiceFS 客户端。这也是得益于 CSI 驱动与 JuiceFS 客户端分离的[架构](../introduction.md#architecture)。

你可以在[镜像仓库](https://hub.docker.com/r/juicedata/mount/tags?page=1&ordering=last_updated&name=v)找到最新版的 Mount Pod 镜像，镜像标签格式为 `v<JUICEFS-CE-VERSION>-<JUICEFS-EE-VERSION>`，其中 `<JUICEFS-CE-VERSION>` 表示 JuiceFS 社区版客户端的版本号，`<JUICEFS-EE-VERSION>` 表示 JuiceFS 云服务客户端的版本号。

如果 JuiceFS 客户端的新版尚未正式发布，或新版本的 Mount Pod 容器镜像尚未构建，你也可以自行构建 Mount Pod 容器镜像，参考[「构建 Mount Pod 的容器镜像」](../guide/custom-image.md#build-mount-pod-image)。

### 动态配置

[「动态配置」](../guide/pv.md#dynamic-provisioning)模式下，你需要在 `StorageClass` 中定义配置容器镜像：

```yaml {11}
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
  juicefs/mount-image: juicedata/mount:v1.0.2-4.8.1
```

修改配置后，新创建的 PV 便会使用新版镜像运行 Mount Pod。

### 静态配置

[「静态配置」](../guide/pv.md#static-provisioning)模式下，需要在 `PersistentVolume` 定义中配置 Mount Pod 镜像：

```yaml {22}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/mount-image: juicedata/mount:v1.0.2-4.8.1
```

## 临时升级 JuiceFS 客户端

:::tip 提示
强烈建议升级 JuiceFS CSI 驱动至 v0.10 及以后版本，此处介绍的客户端升级方法仅作为展示用途，不建议在生产环境中长期使用。
:::

如果你在使用进程挂载模式，或者仅仅是难以升级到 v0.10 之后的版本，但又需要使用新版 JuiceFS 进行挂载，那么也可以通过以下方法，在不升级 CSI 驱动的前提下，单独升级 CSI Node Service 中的 JuiceFS 客户端。

由于这是在 CSI Node Service 容器中临时升级 JuiceFS 客户端，完全是临时解决方案，可想而知，如果 CSI Node Service 的 Pod 发生了重建，又或是新增了节点，都需要再次执行该升级过程。

1. 使用以下脚本将 `juicefs-csi-node` pod 中的 `juicefs` 客户端替换为新版：

   ```bash
   #!/bin/bash

   # 运行前请替换为正确路径
   KUBECTL=/path/to/kubectl
   JUICEFS_BIN=/path/to/new/juicefs

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system cp $JUICEFS_BIN '{}':/tmp/juicefs -c juicefs-plugin

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system exec -i '{}' -c juicefs-plugin -- \
       chmod a+x /tmp/juicefs && mv /tmp/juicefs /bin/juicefs
   ```

2. 将应用逐个重新启动，或 kill 掉已存在的 pod。
