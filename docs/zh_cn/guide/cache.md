---
title: 缓存
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## 设置缓存路径 {#cache-dir}

Kubernetes 节点往往采用单独的数据盘作为缓存盘，因此使用 JuiceFS 时，一定要注意正确设置缓存路径，否则默认使用根分区的 `/var/jfsCache` 目录来缓存数据，极易耗尽磁盘空间。

设置缓存路径以后，Kubernetes 宿主机上的路径会以 `hostPath` 卷的形式挂载到 Mount Pod 中，因此还需要根据缓存盘参数，对缓存相关的[挂载参数](./pv.md#mount-options)进行调整（如缓存大小）。

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

* [Amazon EBS CSI 驱动](https://docs.aws.amazon.com/zh_cn/eks/latest/userguide/ebs-csi.html)
* [在 Azure Kubernetes 服务（AKS）中使用 Azure 磁盘 CSI 驱动](https://learn.microsoft.com/zh-cn/azure/aks/azure-disk-csi)
* [使用 Google Compute Engine 永久性磁盘 CSI 驱动](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver)
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

## 缓存预热 {#warmup}

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

Mount Pod 中社区版和云服务 JuiceFS 客户端的路径不同，注意分辨：

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

## 独立缓存集群

:::note 注意
独立缓存集群功能目前仅在 JuiceFS 云服务和企业版中提供，社区版暂不支持。
:::

Kubernetes 容器往往是「转瞬即逝」的，在这种情况下构建[「分布式缓存」](https://juicefs.com/docs/zh/cloud/guide/cache#client-cache-sharing)，会由于缓存组成员不断更替，导致缓存利用率走低。也正因如此，JuiceFS 云服务还支持[「独立缓存集群」](https://juicefs.com/docs/zh/cloud/guide/cache#dedicated-cache-cluster)，用于优化此种场景下的缓存利用率。

为了在 Kubernetes 集群部署一个稳定的缓存集群，可以参考以下示范，用 StatefulSet 在集群内挂载 JuiceFS 客户端，形成一个稳定的缓存组。

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  # 名称、命名空间可自定义
  name: juicefs-cache-group
  namespace: kube-system
spec:
  # 缓存组客户端数量
  replicas: 1
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: juicefs-cache-group
      juicefs-role: cache
  serviceName: juicefs-cache-group
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: juicefs-cache-group
        juicefs-role: cache
    spec:
      # 一个 Kubernetes 节点上只运行一个缓存组客户端
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: jfs-role
                operator: In
                values:
                - cache
            topologyKey: kubernetes.io/hostname
      # 使用 hostNetwork，让 Pod 以固定 IP 运行，避免容器重建更换 IP，导致缓存数据失效
      hostNetwork: true
      # 初始化容器负责执行 juicefs auth 命令
      # 参考文档：https://juicefs.com/docs/zh/cloud/reference/commands_reference#auth
      initContainers:
      - name: jfs-format
        command:
        - sh
        - -c
        # 请将 $VOL_NAME 换成控制台上创建的文件系统名
        # 参考文档：https://juicefs.com/docs/zh/cloud/getting_started#create-file-system
        - /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} $VOL_NAME
        env:
        # 存放文件系统认证信息的 Secret，必须和该 StatefulSet 在同一个命名空间下
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
        - name: ACCESS_KEY
          valueFrom:
            secretKeyRef:
              key: access-key
              name: jfs-secret-ee
        - name: SECRET_KEY
          valueFrom:
            secretKeyRef:
              key: secret-key
              name: jfs-secret-ee
        - name: TOKEN
          valueFrom:
            secretKeyRef:
              key: token
              name: jfs-secret-ee
        image: juicedata/mount:v1.0.2-4.8.2
        volumeMounts:
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      # 负责挂载 JuiceFS 文件系统并组建独立缓存集群的容器
      # 参考文档：https://juicefs.com/docs/zh/cloud/guide/cache#dedicated-cache-cluster
      containers:
      - name: juicefs-cache
        command:
        - sh
        - -c
        # 请将 $VOL_NAME 换成控制台上创建的文件系统名
        # 由于在容器中常驻，必须用 --foreground 模式运行，其它挂载选项（特别是 --cache-group）请适当调整
        # 参考文档：https://juicefs.com/docs/zh/cloud/reference/commands_reference#mount
        - /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --foreground --cache-dir=/data/jfsCache --cache-size=512000 --cache-group=jfscache
        # 使用 Mount Pod 的容器镜像
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:v1.0.2-4.8.2
        lifecycle:
          # 容器退出时卸载文件系统
          preStop:
            exec:
              command:
              - sh
              - -c
              - umount /mnt/jfs
        # 请适当调整资源请求和约束
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        # 挂载文件系统必须启用的权限
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /dev/shm
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # 请适当调整缓存目录的路径，如有多个缓存目录需要定义多个 volume
      # 参考文档：https://juicefs.com/docs/zh/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /dev/shm
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

上方示范便是在集群中启动了 JuiceFS 缓存集群，其缓存组名为 `jfscache`，那么为了让应用程序的 JuiceFS 客户端使用该缓存集群，需要让他们一并加入这个缓存组，并额外添加 `--no-sharing` 这个挂载参数，这样一来，应用程序的 JuiceFS 客户端虽然加入了缓存组，但却不参与缓存数据的构建，避免了客户端频繁创建、销毁所导致的缓存数据不稳定。

以动态配置为例，按照下方示范修改挂载参数即可，关于在 `mountOptions` 调整挂载配置，请详见[「挂载参数」](../guide/pv.md#mount-options)。

```yaml {13-14}
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
  ...
  - cache-group=jfscache
  - no-sharing
```
