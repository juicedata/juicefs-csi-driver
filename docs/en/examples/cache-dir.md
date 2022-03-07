---
sidebar_label: cache directory
---

# How to set cache directory in Kubernetes

This document shows how to set the cache directory for JuiceFS in Kubernetes. When CSI deploys mount pod (JuiceFS client),
the cache directory on the corresponding node will be mounted to the mount pod. If you need to set the disk path on the node as the cache path of the client, you can follow this document.

## Static provisioning

By default, the cache path is `/var/jfsCache`, which CSI will mount into the mount pod. You can set cache directory in `spec.mountOptions` of PV (Persistent Volume):

```yaml
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
    volumeHandle: test-bucket
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

For PVC (PersistentVolumeClaim) and sample pod, Refer to [This document](./static-provisioning.md) for more details.

### Check cache directory

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

You can also verify that the JuiceFS client has the expected cache path set. Refer
to [this document](../troubleshooting.md#get-mount-pod) to find mount pod and run this command as follows:

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-test-bucket -oyaml | grep mount.juicefs
```

## Dynamic provisioning

By default, the cache path is `/var/jfsCache`, which CSI will mount into the mount pod. You can set cache directory in `mountOptions` of StorageClass:

```yaml
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
mountOptions:
  - cache-dir=/dev/vdb1
```

For PVC (PersistentVolumeClaim) and sample pod, Refer to [This document](./dynamic-provisioning.md) for more details.

### Check cache directory

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

You can also verify that the JuiceFS client has the expected cache path set. Refer
to [this document](../troubleshooting.md#get-mount-pod) to find mount pod and run this command as follows:

```sh
kubectl -n kube-system get po juicefs-172.16.2.87-pvc-5916988b-71a0-4494-8315-877d2dbb8709 -oyaml | grep mount.juicefs
```
