---
sidebar_label: 静态配置
---

# 在 Kubernetes 中使用 JuiceFS 的静态配置方法

本文档展示了如何在 pod 内安装静态配置的 JuiceFS PersistentVolume (PV)。

## 准备工作

在 Kubernetes 中创建 CSI Driver 的 Secret (以 Amazon S3 为例)：

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] \
    --from-literal=storage=s3 \
    --from-literal=bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --from-literal=access-key=<ACCESS_KEY> \
    --from-literal=secret-key=<SECRET_KEY>
```

- `name`：JuiceFS 文件系统名称
- `metaurl`：元数据服务的访问 URL (比如 Redis)。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/databases_for_metadata) 。
- `storage`：对象存储类型，比如 `s3`，`gs`，`oss`。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `bucket`：Bucket URL。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/how_to_setup_object_storage) 。
- `access-key`：Access key。
- `secret-key`：Secret key。

用您自己的环境变量替换由 `<>` 括起来的字段。 `[]` 中的字段是可选的，它与您的部署环境相关。

您应该确保：
1. `access-key` 和 `secret-key` 对需要有对象存储 bucket 的 `GET`、`PUT`、`DELETE` 权限。
2. Redis DB 是干净的，并且 password(如果有的话) 是正确的

您可以执行 [`juicefs format`](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount) 命令确保 secret 是正确的。

```sh
./juicefs format --storage=s3 --bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --access-key=<ACCESS_KEY> --secret-key=<SECRET_KEY> \
    redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] <NAME>
```

## 部署

创建  PersistentVolume (PV)、PersistentVolumeClaim (PVC) 和示例 pod

```sh
kubectl apply -f - <<EOF
---
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
    volumeHandle: test-bucket
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  storageClassName: ""
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
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
      name: data
    resources:
      requests:
        cpu: 10m
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: juicefs-pvc
EOF
```

## 检查使用的 JuiceFS 文件系统

当所有的资源创建好之后，确认 10 Pi PV 创建好：

```sh
kubectl get pv
```

确认 pod 状态是 running：

```sh
kubectl get pods
```

确认数据被正确地写入 JuiceFS 文件系统中：

```sh
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

也可以通过将 JuiceFS 挂载到主机来验证在 JuiceFS 文件系统中创建了 PV 的目录：

```
juicefs mount -d redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] /jfs
```
