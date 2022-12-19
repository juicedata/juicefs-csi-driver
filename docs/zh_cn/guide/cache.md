---
title: 缓存
sidebar_position: 2
---

## 设置缓存路径 {#cache-dir}

Kubernetes 节点往往采用单独的数据盘作为缓存盘，因此使用 JuiceFS 时，一定要注意正确设置缓存路径，否则默认使用根分区的 `/var/jfsCache` 目录来缓存数据，极易耗尽磁盘空间。

:::note
与 JuiceFS 客户端的 `--cache-dir` 参数不同，在 CSI 驱动中，`cache-dir` 不支持填写通配符，如果需要用多个设备作为缓存盘，请填写多个目录，以 `:` 连接。详见[社区版](https://juicefs.com/docs/zh/community/command_reference/#mount)与[商业版](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)文档。
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

如有需要，JuiceFS CSI 驱动 0.15.1 及以上版本支持使用 PVC 作为缓存路径，该实践多用于托管 Kubernetes 集群的云服务商，让你可以使用单独的云盘来作为 JuiceFS CSI 驱动的缓存存储设备。

首先，按照所使用的托管 Kubernetes 集群的云服务商的说明，创建 PVC，比如：

* [阿里云 ACK 云盘存储卷](https://help.aliyun.com/document_detail/134767.html)
* [AWS EKS 的持久性存储](https://aws.amazon.com/cn/premiumsupport/knowledge-center/eks-persistent-storage/)

假设 PVC `ebs-pvc` 创建完毕，与 Mount Pod 在同一个命名空间下（默认 kube-system），参考下方示范，让 CSI 驱动使用该 PVC 作为缓存路径。

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

JuiceFS 客户端运行在 Mount Pod 中，也正因此，缓存预热也同样需要在 Mount Pod 中进行，参考下方的命令钻进 Mount Pod 中，然后运行预热命令：

```shell
# 提前将应用 pod 信息存为环境变量
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 一行命令钻进 Mount Pod
kubectl -n kube-system exec -it $(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')) -- bash

# 确定 JuiceFS 在容器内的挂载点
df -h

# 社区版和商业版 JuiceFS 客户端路径不同，注意甄别
/usr/local/bin/juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data  # 社区版
/usr/bin/juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data  # 商业版
```

特别地，如果你的应用容器中也安装有 JuiceFS 客户端，也完全可以直接在应用容器中运行预热命令，操作甚至更加便捷。

## 独立缓存集群（商业版）

Kubernetes 容器往往是“转瞬即逝”的，在这种情况下构建[「分布式缓存」](https://juicefs.com/docs/zh/cloud/guide/cache#client-cache-sharing)，会由于缓存组成员不断更替，导致缓存利用率走低。也正因如此，JuiceFS 商业版还支持[「独立缓存集群」](https://juicefs.com/docs/zh/cloud/guide/cache#dedicated-cache-cluster)，用于优化此种场景下的缓存利用率。
