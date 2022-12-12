---
title: Customize Container Image
---

This chapter describes how to overwrite the mount pod image and how to build the CSI Driver component image by yourself.

## Overwrite mount pod image {#overwrite-mount-pod-image}

From JuiceFS CSI Driver 0.17.1 and above, modifying the default mount pod image is supported. You can find the latest mount pod image in [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v), the image tag format is `juicedata/mount:v<JUICEFS-CE-VERSION>-<JUICEFS-EE-VERSION>`, where `<JUICEFS-CE-VERSION>` stands for the JuiceFS Community Edition client version (e.g. `1.0.0`), and `<JUICEFS-EE-VERSION>` stands for JuiceFS Cloud Service client version (e.g. `4.8.0`).

When changing mount pod image, CSI Driver offers flexible control over the scope, choose a method that suits your situation.

### Configure CSI Node to overwrite mount pod image globally

Change CSI Node settings so that mount pod image is overwritten globally, choose this method if you wish to change the image for all applications.

Edit CSI Node Service (a DaemonSet), add the `JUICEFS_MOUNT_IMAGE` environment variable to the `juicefs-plugin` container:

```shell
kubectl -n kube-system edit daemonset juicefs-csi-node
```

Modify the YAML:

```yaml {11-12}
apiVersion: apps/v1
kind: DaemonSet
...
spec:
  template:
    spec:
      containers:
      - name: juicefs-plugin
        image: juicedata/juicefs-csi-driver:nightly
        env:
        - name: JUICEFS_MOUNT_IMAGE
          value: juicedata/mount:patch-some-bug
```

### Configure StorageClass to specify mount pod image

If you need to use different mount pod image for different applications, you'll need to create multiple StorageClass, and specify the desired mount pod image for each StorageClass.

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

And then in PVC definitions, reference the needed StorageClass via the `storageClassName` field, so that you may use different mount pod image for different applications.

### Static provisioning

For [Static provisioning](./pv.md#static-provisioning), you'll have to configure mount pod image inside the PV definition.

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

## Build image

### Build mount pod image {#build-mount-pod-image}

JuiceFS CSI Driver adopt a [decoupled architecture](../introduction.md#architecture), mount pod image defaults to [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount), built using [`docker/dev.juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/dev.juicefs.Dockerfile).

If you were to build your own mount pod image, refer to below code to clone the JuiceFS Community Edition repository, and then build the image using the provided Dockerfile:

```shell
# Clone JuiceFS Community Edition repository
git clone https://github.com/juicedata/juicefs
cd juicefs

# Switch to desired branch, or modify code as needed
git checkout ...

# The corresponding Dockerfile resides in the CSI Driver repository
curl -O https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/docker/dev.juicefs.Dockerfile

# Build the Docker image, and then push to your private registry
docker build -t registry.example.com/juicefs-csi-mount:latest -f dev.juicefs.Dockerfile .
docker push registry.example.com/juicefs-csi-mount:latest
```

To use the newly built image, refer to [Overwrite mount pod image](#overwrite-mount-pod-image).

### Build CSI Driver component image

JuiceFS CSI Controller / JuiceFS CSI Node pod image default to [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver), built using [`docker/Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/Dockerfile).

If you wish to make modifications and build your own JuiceFS CSI Driver, use below commands to clone the repository, and build the Docker image using the provided script:

```shell
# Clone the CSI Driver repository
git clone https://github.com/juicedata/juicefs-csi-driver
cd juicefs-csi-driver

# Switch to desired branch, or modify code as needed
git checkout ...

# Specify the image name using IMAGE environment variable, and then push to your private registry
IMAGE=registry.example.com/juicefs-csi-driver make image-dev
docker push registry.example.com/juicefs-csi-driver:dev-xxx
```
