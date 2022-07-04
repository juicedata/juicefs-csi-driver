---
sidebar_label: 动态配置
---

# 在 Kubernetes 中使用 JuiceFS 的动态配置方法

本文档展示了如何在 pod 内安装动态配置的 JuiceFS volume。

## 准备工作

在 Kubernetes 中创建 CSI Driver 的 `Secret`，社区版和云服务版所需字段有所区别，分别如下：

### 社区版

以 Amazon S3 为例：

```yaml {7-14}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <NAME>
  metaurl: redis://[:<PASSWORD>]@<HOST>:6379[/<DB>]
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  # 如果需要设置 JuiceFS Mount Pod 的时区请将下一行的注释符号删除，默认为 UTC 时间。
  # envs: "{TZ: Asia/Shanghai}"
```

其中：
- `name`：JuiceFS 文件系统名称
- `metaurl`：元数据服务的访问 URL (比如 Redis)。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/databases_for_metadata) 。
- `storage`：对象存储类型，比如 `s3`，`gs`，`oss`。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `bucket`：Bucket URL。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `access-key`：对象存储的 access key。
- `secret-key`：对象存储的 secret key。

用您自己的环境变量替换由 `<>` 括起来的字段。 `[]` 中的字段是可选的，它与您的部署环境相关。

您应该确保：
1. `access-key` 和 `secret-key` 对需要有对象存储 bucket 的 `GetObject`、`PutObject`、`DeleteObject` 权限。
2. Redis DB 是干净的，并且 `password`（如果有的话）是正确的

### 云服务版

```yaml {7-12}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${JUICEFS_ACCESSKEY}
  secret-key: ${JUICEFS_SECRETKEY}
  # 如果需要设置 JuiceFS Mount Pod 的时区请将下一行的注释符号删除，默认为 UTC 时间。
  # envs: "{TZ: Asia/Shanghai}"
```

其中：
- `name`：JuiceFS 文件系统名称
- `token`：JuiceFS 管理 token。更多信息参考[这篇文档](https://juicefs.com/docs/zh/cloud/metadata#令牌管理)。
- `access-key`：对象存储的 Access key。
- `secret-key`：对象存储的 Secret key。

您应该确保 `access-key` 和 `secret-key` 对需要有对象存储 bucket 的 `GetObject`、`PutObject`、`DeleteObject` 权限。

## 部署

创建 StorageClass、PersistentVolumeClaim（PVC）和示例 pod：

:::info 说明
由于 JuiceFS 是一个弹性文件系统，它不需要强制分配容量。你在 `PersistentVolume` 和 `PersistentVolumeClaim` 中指定的容量并不是实际存储容量。但是，由于存储容量是 Kubernetes 的必填字段，因此您可以使用任何有效值，例如 `10Pi` 表示容量。
:::

```yaml
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
  namespace: default
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: juicefs-pv
  volumes:
  - name: juicefs-pv
    persistentVolumeClaim:
      claimName: juicefs-pvc
EOF
```

## 检查使用的 JuiceFS 文件系统

当所有的资源创建好之后，确认 pod 状态是 running：

```sh
kubectl get pods
```

确认数据被正确地写入 JuiceFS 文件系统中：

```sh
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```
