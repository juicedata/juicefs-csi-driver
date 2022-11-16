---
sidebar_label: Customize the Container Image of Mount Pod
---

# How to customize the container image of Mount Pod

:::note
This feature requires JuiceFS CSI Driver version 0.17.1 and above.
If the CSI Driver is started by process mounting, that is, the startup parameters of CSI Node and CSI Controller use `--by-process=true`, all relevant settings described in this document will be ignored.
:::

By default, the container image of the JuiceFS Mount Pod is `juicedata/mount:v<JUICEFS-CE-LATEST-VERSION>-<JUICEFS-EE-LATEST-VERSION>`, where `<JUICEFS-CE-LATEST-VERSION>` means The latest version number of JuiceFS Community Edition client (e.g. `1.0.0`), `<JUICEFS-EE-LATEST-VERSION>` represents the latest version number of JuiceFS Cloud Service client (e.g. `4.8.0`). You can view all image tags on [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags).

This document shows how to customize the container image of Mount Pod. For how to build the container image of Mount Pod, please refer to [document](../development/build-juicefs-image.md#build-the-container-image-of-juicefs-mount-pod).

## Overwrite default container image when installing CSI Driver

When the JuiceFS CSI Node starts, setting the `JUICEFS_MOUNT_IMAGE` environment variable in the `juicefs-plugin` container can override the default Mount Pod image:

:::note
Once the `juicefs-plugin` container is started, the default Mount Pod image cannot be modified. If you need to modify it, please recreate the container and set the new `JUICEFS_MOUNT_IMAGE` environment variable.
:::

```yaml {12-13}
apiVersion: apps/v1
kind: DaemonSet
# metadata: ...
spec:
  template:
    # metadata: ...
    spec:
      containers:
      - name: juicefs-plugin
        image: juicedata/juicefs-csi-driver:nightly
        env:
        - name: JUICEFS_MOUNT_IMAGE
          value: juicedata/mount:patch-some-bug
```

## Set container image in `PersistentVolume`

You can also set image for the Mount Pod in `PersistentVolume`:

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
      juicefs/mount-image: juicedata/mount:patch-some-bug
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
```

### Check image of the Mount Pod

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

Also you can verify that mount image are customized in the Mount Pod:

```sh
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-mount -o yaml | grep 'image: '
```

## Set container image in `StorageClass`

You can also set image for the Mount Pod in `StorageClass`:

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
  juicefs/mount-image: juicedata/mount:patch-some-bug
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
```

### Check image of the Mount Pod

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

Also you can verify that mount image are customized in the Mount Pod:

```sh
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-mount -o yaml | grep 'image: '
```
