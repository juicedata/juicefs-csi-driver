---
title: 缓存
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## 设置缓存路径 {#cache-dir}

Kubernetes 节点往往采用单独的数据盘作为缓存盘，因此使用 JuiceFS 时，一定要注意正确设置缓存路径，否则默认使用根分区的 `/var/jfsCache` 目录来缓存数据，极易耗尽磁盘空间。

设置缓存路径以后，Kubernetes 宿主机上的路径会以 `hostPath` 卷的形式挂载到 Mount Pod 中，因此还需要根据缓存盘参数，对缓存相关的[挂载选项](./pv.md#mount-options)进行调整（如缓存大小）。

:::note 注意
与 JuiceFS 客户端的 `--cache-dir` 参数不同，在 CSI 驱动中，`cache-dir` 不支持填写通配符，如果需要用多个设备作为缓存盘，请填写多个目录，以 `:` 连接。详见[社区版](https://juicefs.com/docs/zh/community/command_reference/#mount)与[云服务](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)文档。
:::

### 静态配置

在 PV 的 `spec.mountOptions` 中设置缓存路径：

```yaml {15}
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
  mountOptions:
    - cache-dir=/dev/vdb1
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

创建并挂载了该 PV 后，可以[打印 Mount Pod 的启动命令](../administration/troubleshooting.md#check-mount-pod)，确认参数中已经包含修改后的缓存路径，来验证配置生效。

### 动态配置

在 StorageClass 的 `mountOptions` 中配置缓存路径：

```yaml {12}
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
mountOptions:
  - cache-dir=/dev/vdb1
```

创建并挂载了 PV 后，可以[打印 Mount Pod 的启动命令](../administration/troubleshooting.md#check-mount-pod)，确认参数中已经包含修改后的缓存路径，来验证配置生效。

## 使用 PVC 作为缓存路径

JuiceFS CSI 驱动 0.15.1 及以上版本支持使用 PVC 作为缓存路径，该实践多用于托管 Kubernetes 集群的云服务商，让你可以使用单独的云盘来作为 CSI 驱动的缓存存储设备。

首先，按照所使用的托管 Kubernetes 集群的云服务商的说明，创建 PVC，比如：

* [Amazon EKS 中使用 EBS](https://docs.aws.amazon.com/zh_cn/eks/latest/userguide/ebs-csi.html)
* [阿里云 ACK 云盘存储卷](https://help.aliyun.com/document_detail/134767.html)

假设 PVC `ebs-pvc` 创建完毕，与 Mount Pod 在同一个命名空间下（默认 `kube-system`），参考下方示范，让 CSI 驱动使用该 PVC 作为缓存路径。

### 静态配置

将这个 PVC 提供给 JuiceFS PV 使用：

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
      juicefs/mount-cache-pvc: "ebs-pvc"
```

### 动态配置

将 `ebs-pvc` 填入 StorageClass 中：

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
  juicefs/mount-cache-pvc: "ebs-pvc"
```

## 缓存预热

JuiceFS 客户端运行在 Mount Pod 中，也正因此，缓存预热也同样需要在 Mount Pod 中进行，参考下方的命令进入 Mount Pod 中，然后运行预热命令：

```shell
# 提前将应用 pod 信息存为环境变量
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 一行命令进入 Mount Pod
kubectl -n kube-system exec -it $(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')) -- bash

# 确定 JuiceFS 在容器内的挂载点
df -h | grep JuiceFS
```

社区版和云服务 JuiceFS 客户端在 Mount Pod 中的路径不同，注意甄别：

<Tabs>
  <TabItem value="community-edition" label="社区版">

```shell
/usr/local/bin/juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data
```

  </TabItem>
  <TabItem value="cloud-service" label="云服务">

```shell
/usr/bin/juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data
```

  </TabItem>
</Tabs>

特别地，如果你的应用容器中也安装有 JuiceFS 客户端，那么也可以直接在应用容器中运行预热命令。

## Mount Pod 退出时清理缓存 {#mount-pod-clean-cache}

在不少大规模场景下，已建立的缓存是宝贵的，因此 JuiceFS CSI 驱动默认并不会在 Mount Pod 退出时清理缓存。如果这对你的场景不适用，可以对 PV 进行配置，令 Mount Pod 退出时直接清理自己的缓存。

:::note 注意
此特性需使用 0.14.1 及以上版本的 JuiceFS CSI 驱动
:::

### 静态配置

在 PV 的资源定义中修改 `volumeAttributes`，添加 `juicefs/clean-cache: "true"`：

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
      juicefs/clean-cache: "true"
```

### 动态配置

在 StorageClass 中配置 `parameters`，添加 `juicefs/clean-cache: "true"`：

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
  juicefs/clean-cache: "true"
```
