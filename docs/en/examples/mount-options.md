---
sidebar_label: Config Mount Options
---

# How to use Mount Options in Kubernetes

This document shows how to apply mount options to JuiceFS.

The CSI driver support the `juicefs mount` command line options and _fuse_ mount options (`-o` for `juicefs mount`
command).

```
juicefs mount --max-uploads=50 --cache-dir=/var/foo --cache-size=2048 --enable-xattr -o allow_other <META-URL> <MOUNTPOINT>
```

## Static provisioning

You can use mountOptions in PV:

```yaml {14}
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
    - enable-xattr
    - max-uploads=50
    - cache-size=2048
    - cache-dir=/var/foo
    - allow_other
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

Refer to [JuiceFS mount command](https://juicefs.com/docs/community/command_reference#juicefs-mount) for all supported
options.

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
  name: juicefs-app-mount-options
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

### Check mount options are customized

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app-mount-options
```

Also you can verify that mount options are customized in the mounted JuiceFS file system, refer
to [this document](../troubleshooting.md#get-mount-pod) to find mount pod and run this command as follows::

```sh
kubectl get po juicefs-kube-node-3-test-bucket -oyaml | grep mount.juicefs
```

## Dynamic provisioning

You can use mountOptions in StorageClass:

```yaml {12}
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
  - enable-xattr
  - max-uploads=50
  - cache-size=2048
  - cache-dir=/var/foo
  - allow_other
```

Refer to [JuiceFS mount command](https://juicefs.com/docs/community/command_reference#juicefs-mount) for all supported
options.

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
  name: juicefs-app-mount-options
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
kubectl get pods juicefs-app-mount-options
```

Also you can verify that mount options are customized in the mounted JuiceFS file system, refer
to [this document](../troubleshooting.md#get-mount-pod) to find mount pod :

```sh
kubectl get po juicefs-kube-node-2-pvc-f052a1bd-65b3-471c-8a7a-4263f12b2131 -oyaml grep mount.juicefs
```
