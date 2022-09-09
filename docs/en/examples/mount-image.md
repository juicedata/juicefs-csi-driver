---
sidebar_label: Specify image for Mount Pod
---

# How to use customized image in Mount Pod

This document shows how to specify a customized [image](https://kubernetes.io/docs/concepts/containers/images/) for the Mount Pod. The default image used by the Mount Pod is [`juicedata/mount:nightly`](https://hub.docker.com/r/juicedata/mount/tags), to ensure the Mount Pod works well, please use images built by Dockerfile based on the [`juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile).

:::note 注意
If the CSI driver is started by process mounting, that is, the startup parameters of CSI Node and CSI Controller use `--by-process=true`, all relevant settings described in this document will be ignored.
:::

## Overwrite default image when installing CSI

When the CSI nodes start, we can overwrite the default Mount Pod image by settring the env variable `JUICEFS_MOUNT_IMAGE` for the container `juicefs-plugin`. We already set `JUICEFS_MOUNT_IMAGE` to the latest stable Mount Pod image to `juicedata/mount:{latest ce version}-{latest ee version}` when building the CSI image.

:::note 注意
Once the container `juicefs-plugin` starts, you can never modify the default image of the Mount Pod. If you persist to modify it, please re-create the container and set the env variable `JUICEFS_MOUNT_IMAGE` again.
:::

```yaml {12-13}
apiVersion: apps/v1
kind: DaemonSet
# metadata:
spec:
  template:
    # metadata:
    spec:
      containers:
      - name: juicefs-plugin
        image: juicedata/juicefs-csi-driver:nightly
        env:
        - name: JUICEFS_MOUNT_IMAGE
          value: juicedata/mount:patch-some-bug
```

## Set image in `PersistentVolume`

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
kubectl get -n kube-system po juicefs-{k8s-node}-juicefs-pv-{hash id} -o yaml | grep image
```

## Set image in `StorageClass`

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
kubectl get -n kube-system po juicefs-{k8s-node}-juicefs-pv-{hash id} -o yaml | grep image
```
