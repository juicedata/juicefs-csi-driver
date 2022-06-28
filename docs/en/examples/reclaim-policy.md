---
sidebar_label: Reclaim Policy of PV
---

# Reclaim Policy of PV

## Static provisioning

In static provisioning, only the Retain reclaim policy is supported, that is manual reclamation of the resource.
The configuration is as follows:

```yaml {13}
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
```

## Dynamic provisioning

In dynamic provisioning, Retain and Delete reclaim policy are supported. Retain allows for manual reclamation of the resource. Delete refers to the automatic recycling of dynamically created resources. The configuration is as follows:

```yaml {6}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
reclaimPolicy: Retain
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
```

PVs that were dynamically provisioned inherit the reclaim policy of their StorageClass, which defaults to Delete. 
