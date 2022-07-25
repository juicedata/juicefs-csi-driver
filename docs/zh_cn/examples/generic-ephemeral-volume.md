---
sidebar_label: 使用通用临时卷
---

# 在 Kubernetes 中使用 JuiceFS 的通用临时卷

Kubernetes 的[通用临时卷](https://kubernetes.io/zh-cn/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes)类似于 `emptyDir`，为 pod 提供临时数据存放目录。本文档将展示如何使用 JuiceFS 的通用临时卷。

## 准备工作

### 创建 Secret

在 Kubernetes 中创建 `Secret`，具体可参考文档：[创建 Secret](./dynamic-provisioning.md#准备工作)。

### 创建 StorageClass

根据上一步创建的 `Secret`，创建 `StorageClass`：

```yaml
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
```

## 在 Pod 中使用通用临时卷

在 Pod 中可以直接申明通用临时卷，指定 `storageClassName` 即可：

:::note 注意
以下示例中的 `storage: 1Gi` 并不会真正限制通用临时卷的最大容量为 1GiB，因为 JuiceFS 暂不支持对子目录设置容量配额。
:::

```yaml {19-30}
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
    ephemeral:
      volumeClaimTemplate:
        metadata:
          labels:
            type: juicefs-ephemeral-volume
        spec:
          accessModes: [ "ReadWriteMany" ]
          storageClassName: "juicefs-sc"
          resources:
            requests:
              storage: 1Gi
```

使用通用临时卷，Kubernetes 会自动为 pod 创建 PVC，在 pod 销毁时，PVC 也会被一同销毁。
