---
title: 缓存
sidebar_position: 3
---

JuiceFS 有着强大的缓存设计，阅读[社区版文档](https://juicefs.com/docs/zh/community/guide/cache)、[云服务文档](https://juicefs.com/docs/zh/cloud/guide/cache)以了解，本章主要介绍 CSI 驱动中，缓存相关功能的配置方法，以及最佳实践。

在 CSI 驱动中，可以选择使用宿主机路径或者 PVC 作为 JuiceFS 客户端缓存，两种用法并没有性能上的区别，主要在于隔离度和数据本地性，具体而言：

* 宿主机路径（也就是 `hostPath`）简单易用，由于缓存数据就落在虚拟机本地盘，观测和管理都很直接。但是考虑到 Mount Pod 可能会随着业务容器被分配到其他节点，如果发生这种情况，缓存就会失效，并且在原节点的缓存数据可能需要安排清理（阅读下方相关章节了解如何配置缓存清理）。
* 如果你的集群中，所有节点均用于运行 Mount Pod，那么由于每个宿主机都持有大致相同的缓存（或者使用了分布式缓存），那么 Pod 漂移的问题可能也并不构成影响，完全可以使用宿主机路径作为缓存。
* 如果用 PVC 作为缓存存储，好处是不同 JuiceFS PV 可以隔离缓存数据、分别管理，并且就算 Mount Pod 随着业务被迁移到其他节点，由于 PVC 引用关系不变，所以缓存数据仍然可以访问。

## 使用宿主机目录作为缓存（`hostPath`） {#cache-settings}

Kubernetes 节点往往采用单独的数据盘作为缓存盘，因此使用 JuiceFS 时，一定要注意正确设置缓存路径，否则默认使用根分区的 `/var/jfsCache` 目录来缓存数据，极易耗尽磁盘空间。

设置缓存路径以后，Kubernetes 宿主机上的路径会以 `hostPath` 卷的形式挂载到 Mount Pod 中，因此还需要根据缓存盘参数，对缓存相关的[挂载参数](./configurations.md#mount-options)进行调整（如缓存大小）。

:::tip 注意

* 在 CSI 驱动中，`cache-dir` 不支持填写通配符，如果需要用多个设备作为缓存盘，填写多个目录，以 `:` 连接。详见[社区版](https://juicefs.com/docs/zh/community/command_reference/#mount)与[云服务](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)文档。
* 对于大量小文件写入场景，我们一般推荐临时开启客户端写缓存，但由于该模式本身带来的数据安全风险，我们尤其不推荐在 CSI 驱动中开启 `--writeback`，避免容器出现意外时，写缓存尚未完成上传，造成数据无法访问。

:::

### 使用 ConfigMap

阅读[「调整挂载参数」](./configurations.md#custom-cachedirs)。

### 在 PV 中定义（不推荐）

自 CSI 驱动 v0.25.1，ConfigMap 已经支持设置缓存路径，建议按照上一小节的指示用 ConfigMap 来对各个 JuiceFS PV 的配置进行中心化管理，避免使用下方示范，在各个 PV 定义中单独修改缓存路径。

静态配置示范：

```yaml {15-16}
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
    - cache-size=204800
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

动态配置示范：

```yaml {12-13}
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
  - cache-size=204800
```

## 使用 PVC 作为缓存路径

JuiceFS CSI 驱动 0.15.1 及以上版本支持使用 PVC 作为缓存路径，该实践多用于托管 Kubernetes 集群的云服务商，让你可以使用单独的云盘来作为 CSI 驱动的缓存存储设备。

首先，按照所使用的托管 Kubernetes 集群的云服务商的说明，创建 PVC，比如：

* [Amazon EBS CSI 驱动](https://docs.aws.amazon.com/zh_cn/eks/latest/userguide/ebs-csi.html)
* [在 Azure Kubernetes 服务（AKS）中使用 Azure 磁盘 CSI 驱动](https://learn.microsoft.com/zh-cn/azure/aks/azure-disk-csi)
* [使用 Google Compute Engine 永久性磁盘 CSI 驱动](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver)
* [阿里云 ACK 云盘存储卷](https://help.aliyun.com/document_detail/134767.html)

假设名为 `jfs-cache-pvc` 的 PVC 创建完毕，与 Mount Pod 在同一个命名空间下（默认 `kube-system`），参考下方示范，让 CSI 驱动使用该 PVC 作为缓存路径。

### 使用 ConfigMap

缓存相关配置均通过挂载参数进行调整，请阅读[「调整挂载参数」](./configurations.md#custom-cachedirs)。

### 在 PV 中定义（不推荐）

自 CSI 驱动 v0.25.1，ConfigMap 已经支持设置缓存路径，建议按照上一小节的指示用 ConfigMap 来对各个 JuiceFS PV 的配置进行中心化管理，避免使用下方示范，在各个 PV 定义中单独修改缓存路径。

静态配置示范：

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
      juicefs/mount-cache-pvc: "jfs-cache-pvc"
```

动态配置示范，将 `jfs-cache-pvc` 填入 StorageClass 中：

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
  juicefs/mount-cache-pvc: "jfs-cache-pvc"
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

# 运行预热命令
juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data
```

对于 JuiceFS 企业版的[独立缓存集群场景](https://juicefs.com/docs/zh/cloud/guide/distributed-cache)，如果需要程式化地调用预热命令，可以参考下方示范，用 Kubernetes Job 将预热过程自动化：

```yaml title="warmup-job.yaml"
apiVersion: batch/v1
kind: Job
metadata:
  name: warmup
  labels:
    app.kubernetes.io/name: warmup
spec:
  backoffLimit: 0
  activeDeadlineSeconds: 3600
  ttlSecondsAfterFinished: 86400
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: warmup
        app.kubernetes.io/name: warmup
    spec:
      serviceAccountName: default
      containers:
        - name: warmup
          command:
            - bash
            - -c
            - |
              # 下面的 shell 代码仅在私有部署需要，作用是将 envs 这一 JSON 进行展开，将键值设置为环境变量
              for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
                echo "export $keyval"
                eval export $keyval
              done

              # 认证和挂载，所有环境变量均引用包含着文件系统认证信息的 Kubernetes Secret
              # 参考文档：https://juicefs.com/docs/zh/cloud/getting_started#create-file-system
              /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

              # 以 --no-sharing 模式挂载客户端，避免预热到容器本地缓存目录
              # CACHEGROUP 需要替换成实际环境中的缓存组名
              /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --no-sharing --cache-size=0 --cache-group=CACHEGROUP

              # 判断 warmup 是否成功运行，默认如果有任何数据块下载失败，会报错，此时需要查看客户端日志，定位失败原因
              /usr/bin/juicefs warmup /mnt/jfs
              code=$?
              if [ "$code" != "0" ]; then
                cat /var/log/juicefs.log
              fi
              exit $code
          image: juicedata/mount:ee-5.0.2-69f82b3
          securityContext:
            privileged: true
          env:
            - name: VOL_NAME
              valueFrom:
                secretKeyRef:
                  key: name
                  name: juicefs-secret
            - name: ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  key: access-key
                  name: juicefs-secret
            - name: SECRET_KEY
              valueFrom:
                secretKeyRef:
                  key: secret-key
                  name: juicefs-secret
            - name: TOKEN
              valueFrom:
                secretKeyRef:
                  key: token
                  name: juicefs-secret
            - name: ENVS
              valueFrom:
                secretKeyRef:
                  key: envs
                  name: juicefs-secret
      restartPolicy: Never
```

## 清理缓存 {#mount-pod-clean-cache}

在大规模场景下，已建立的缓存是宝贵的，因此 JuiceFS CSI 驱动默认并不会在 Mount Pod 退出时清理缓存。如果这对你的场景不适用，可以对 PV 进行配置，令 Mount Pod 退出时直接清理自己的缓存。

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

## 独立缓存集群 {#dedicated-cache-cluster}

:::note 注意
独立缓存集群功能目前仅在 JuiceFS 云服务和企业版中提供，社区版暂不支持。
:::

Kubernetes 容器往往是「转瞬即逝」的，在这种情况下构建[「分布式缓存」](/docs/zh/cloud/guide/distributed-cache)，会由于缓存组成员不断更替，导致缓存利用率走低。也正因如此，JuiceFS 云服务还支持[「独立缓存集群」](/docs/zh/cloud/guide/distributed-cache#dedicated-cache-cluster)，用于优化此种场景下的缓存利用率。

在 Kubernetes 中部署分布式缓存集群目前有两种方式：

1. 对于大部分使用场景，可以通过[「缓存组 Operator」](./cache-group-operator.md)部署；
2. 对于需要灵活自定义部署配置的场景，可以通过[「自行编写 YAML 配置文件」](./generic-applications.md#distributed-cache-cluster)的方式部署。
