---
title: Volume Snapshot
sidebar_position: 8
---

JuiceFS CSI Driver supports CSI Volume Snapshot feature. You can use `VolumeSnapshot` resource to create snapshots of JuiceFS volumes and restore data from snapshots.

## Prerequisites

- Kubernetes cluster version >= 1.17
- [Snapshot Controller](https://kubernetes-csi.github.io/docs/snapshot-controller.html) and CRD installed

  ```shell
  kubectl kustomize https://github.com/kubernetes-csi/external-snapshotter/client/config/crd | kubectl create -f -
  kubectl kustomize https://github.com/kubernetes-csi/external-snapshotter/deploy/kubernetes/snapshot-controller | kubectl create -f -
  ```

- JuiceFS CSI Driver version >= 0.31.0
- enable snapshot feature in JuiceFS CSI Driver Chart values:

  ```yaml
  snapshot:
    enabled: true
  ```

## How it works

JuiceFS CSI Driver uses [`juicefs clone`](https://juicefs.com/docs/cloud/reference/command_reference/#snapshot) command to implement snapshot feature.

- **Create Snapshot**: The CSI Driver starts a Job to clone the source directory to `.snapshots/<sourceVolumeID>/<snapshotID>` directory.

- **Restore Snapshot**: The CSI Driver starts a Job to clone the snapshot directory to the new PV directory.

- **Delete Snapshot**: The CSI Driver starts a Job to delete `.snapshots/<sourceVolumeID>/<snapshotID>` directory.

## Usage

### 1. Create VolumeSnapshotClass

First, you need to create a `VolumeSnapshotClass`.

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

### 2. Create VolumeSnapshot

Create a `VolumeSnapshot` resource to trigger snapshot creation.

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

### 3. Restore from Snapshot

Create a new PVC and specify the snapshot in `dataSource`.

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

## Notes

- Snapshot operations are asynchronous and executed by Kubernetes Jobs.
- Snapshot data is stored in the `.snapshots` directory of the JuiceFS file system.
- Please ensure that the JuiceFS file system has enough space to store snapshot data.
