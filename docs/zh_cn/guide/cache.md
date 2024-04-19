---
title: 缓存
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

JuiceFS 有着强大的缓存设计，阅读[社区版文档](https://juicefs.com/docs/zh/community/cache_management)、[云服务文档](https://juicefs.com/docs/zh/cloud/guide/cache)以了解，本章主要介绍 CSI 驱动中，缓存相关功能的配置方法，以及最佳实践。

## 缓存设置 {#cache-settings}

Kubernetes 节点往往采用单独的数据盘作为缓存盘，因此使用 JuiceFS 时，一定要注意正确设置缓存路径，否则默认使用根分区的 `/var/jfsCache` 目录来缓存数据，极易耗尽磁盘空间。

设置缓存路径以后，Kubernetes 宿主机上的路径会以 `hostPath` 卷的形式挂载到 Mount Pod 中，因此还需要根据缓存盘参数，对缓存相关的[挂载参数](./pv.md#mount-options)进行调整（如缓存大小）。

:::tip 注意

* 在 CSI 驱动中，`cache-dir` 不支持填写通配符，如果需要用多个设备作为缓存盘，填写多个目录，以 `:` 连接。详见[社区版](https://juicefs.com/docs/zh/community/command_reference/#mount)与[云服务](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)文档。
* 对于大量小文件写入场景，我们一般推荐临时开启客户端写缓存，但由于该模式本身带来的数据安全风险，我们尤其不推荐在 CSI 驱动中开启 `--writeback`，避免容器出现意外时，写缓存尚未完成上传，造成数据无法访问。
:::

缓存相关配置均通过挂载参数进行调整，因此方法如同[「调整挂载参数」](./pv.md#mount-options)，也可以直接参考下方示范。如果你需要验证参数生效，可以创建并挂载了 PV 后，[打印 Mount Pod 的启动命令](../administration/troubleshooting.md#check-mount-pod)，确认参数中已经包含修改后的缓存路径，来验证配置生效。

* 静态配置：

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

* 动态配置

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

## 独立缓存集群 {#dedicated-cache-cluster}

:::note 注意
独立缓存集群功能目前仅在 JuiceFS 云服务和企业版中提供，社区版暂不支持。
:::

Kubernetes 容器往往是「转瞬即逝」的，在这种情况下构建[「分布式缓存」](/docs/zh/cloud/guide/distributed-cache)，会由于缓存组成员不断更替，导致缓存利用率走低。也正因如此，JuiceFS 云服务还支持[「独立缓存集群」](/docs/zh/cloud/guide/distributed-cache#dedicated-cache-cluster)，用于优化此种场景下的缓存利用率。

在 Kubernetes 中部署分布式缓存集群，配置示范详见[「部署分布式缓存集群」](./generic-applications.md#distributed-cache-cluster)。
