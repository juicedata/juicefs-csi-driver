---
sidebar_label: Resource Management for Mount Pod
---

# How to configure resource request and limit for Mount Pod

This document shows how to [configure resource](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers) request and limit for JuiceFS Mount Pod. The resource request for the Mount Pod defaults to 1 CPU and 1GiB of memory, and the resource limit defaults to 2 CPU and 5GiB of memory.

:::note
If the CSI driver is started by process mounting, that is, the startup parameters of CSI Node and CSI Controller use `--by-process=true`, you need to increase the resource request of CSI Node `DaemonSet` to at least 1 CPU and 1GiB of memory , the resource limit is increased to at least 2 CPU and 5GiB of memory.
:::

## Static provisioning

You can set resource request and limit in `PersistentVolume`:

```yaml {22-25}
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
      juicefs/mount-cpu-limit: 5000m
      juicefs/mount-memory-limit: 5Gi
      juicefs/mount-cpu-request: 1000m
      juicefs/mount-memory-request: 1Gi
```

Apply PVC and sample pod as follows:

```yaml
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
  name: juicefs-app-resources
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
```

### Check resources of mount pod

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app-resources
```

Also you can verify that mount resources are customized in mount pod:

```sh
kubectl -n kube-system get po juicefs-kube-node-2-juicefs-pv -o yaml | grep -A 6 resources
```

## Dynamic provisioning

You can set resource request and limit in `StorageClass`:

```yaml {11-14}
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
  juicefs/mount-cpu-limit: 5000m
  juicefs/mount-memory-limit: 5Gi
  juicefs/mount-cpu-request: 1000m
  juicefs/mount-memory-request: 1Gi
```

Apply PVC and sample pod as follows:

```yaml
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
  name: juicefs-app-resources
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
```

### Check mount options are customized

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app-resources
```

Also you can verify that mount resources are customized in mount pod:

```sh
kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -o yaml | grep -A 6 resources
```
