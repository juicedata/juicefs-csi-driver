---
title: 卷快照
sidebar_position: 8
---

JuiceFS CSI Driver 支持 CSI 卷快照功能。你可以使用 `VolumeSnapshot` 资源来创建 JuiceFS 卷的快照，并使用快照来恢复数据。

## 前提条件

- Kubernetes 集群版本 >= 1.17
- 已安装 [Snapshot Controller](https://kubernetes-csi.github.io/docs/snapshot-controller.html) 和 CRD

  ```shell
  kubectl kustomize https://github.com/kubernetes-csi/external-snapshotter/client/config/crd | kubectl create -f -
  kubectl kustomize https://github.com/kubernetes-csi/external-snapshotter/deploy/kubernetes/snapshot-controller | kubectl create -f -
  ```

- JuiceFS CSI Driver 版本 >= 0.31.0
- 在 JuiceFS CSI Driver Chart 的 values 中启用快照功能：

  ```yaml
  snapshot:
    enabled: true
  ```

## 工作原理

JuiceFS CSI Driver 使用 [`juicefs clone`](https://juicefs.com/docs/zh/cloud/reference/command_reference/#snapshot) 命令来实现快照功能。

- **创建快照**：CSI Driver 会启动一个 Job，将源目录克隆到 `.snapshots/<sourceVolumeID>/<snapshotID>` 目录。

- **恢复快照**：CSI Driver 会启动一个 Job，将快照目录克隆到新的 PV 目录。
  
- **删除快照**：CSI Driver 会启动一个 Job，删除 `.snapshots/<sourceVolumeID>/<snapshotID>` 目录。

## 使用方法

### 1. 创建 VolumeSnapshotClass

首先，你需要创建一个 `VolumeSnapshotClass`。

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: juicefs-snapshot-class
driver: csi.juicefs.com
deletionPolicy: Delete
parameters:
  csi.storage.k8s.io/snapshotter-secret-name: juicefs-secret
  csi.storage.k8s.io/snapshotter-secret-namespace: kube-system
---
```

### 2. 创建 VolumeSnapshot

创建一个 `VolumeSnapshot` 资源来触发快照创建。

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
spec:
  volumeSnapshotClassName: juicefs-snapshot-class
  source:
    persistentVolumeClaimName: my-pvc
```

### 3. 从快照恢复

创建一个新的 PVC，并在 `dataSource` 中指定快照。

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-restore-pvc
spec:
  storageClassName: juicefs-sc
  dataSource:
    name: my-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
```

## 注意事项

- 快照操作是异步的，由 Kubernetes Job 执行。
- 快照数据存储在 JuiceFS 文件系统的 `.snapshots` 目录下。
- 请确保 JuiceFS 文件系统有足够的空间来存储快照数据。
