---
title: Customize Container Image
sidebar_position: 4
---

This chapter describes how to overwrite the mount pod image and how to build the CSI Driver component image by yourself.

## Mount pod image separation {#ce-ee-separation}

JuiceFS Client runs inside the mount pod, and JuiceFS provides [Community Edition](https://juicefs.com/docs/community/introduction) and [Enterprise Edition](https://juicefs.com/docs/cloud), for a long period of time, mount image contains both versions:

* `/usr/local/bin/juicefs`: JuiceFS Community Edition client
* `/usr/bin/juicefs`: JuiceFS Enterprise Edition client

To avoid misuse and reduce image size, from CSI Driver 0.19.0, separated image is provided for CE/EE, you can find the latest mount pod image in [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v), the image tag looks like:

```shell
# Tag of community mount image begin with ce-
juicedata/mount:ce-v1.1.0

# Tag of enterprise mount image begin with ee-
juicedata/mount:ee-5.0.2-69f82b3

# Prior to 0.19.0, tag contains both CE and EE version string
# This won't be maintained and updated in the future
juicedata/mount:v1.0.3-4.8.3
```

## Overwrite mount pod image {#overwrite-mount-pod-image}

From JuiceFS CSI Driver 0.17.1 and above, modifying the default mount pod image is supported. CSI Driver offers flexible control over the scope, choose a method that suits your situation.

:::tip
With mount pod image overwritten, note that:

* Existing mount pods won't be affected, new images will run only if you rolling upgrade app pods, or re-create PVC
* By default, if you [upgrad CSI Driver](../administration/upgrade-csi-driver.md), it'll use the latest stable mount image included with the release. But if you overwrite the mount image using steps provided in this section, then it'll be a fixated config and no longer related to CSI Driver upgrades
:::

### Configure mount pod image globally {#overwrite-in-csi-node}

If you use Helm to manage CSI Driver, changing mount image is as simple as the following one-liner:

```yaml
defaultMountImage:
  # Community Edition
  ce: "juicedata/mount:ce-v1.1.2"
  # Enterprise Edition
  ee: "juicedata/mount:ee-5.0.10-10fbc97"
```

But if you use kubectl to directly install a `k8s.yaml`, you'll have to set environment variables into the CSI Driver components:

```shell
# Community Edition
kubectl -n kube-system set env daemonset/juicefs-csi-node -c juicefs-plugin JUICEFS_CE_MOUNT_IMAGE=juicedata/mount:ce-v1.1.0
kubectl -n kube-system set env statefulset/juicefs-csi-controller -c juicefs-plugin JUICEFS_CE_MOUNT_IMAGE=juicedata/mount:ce-v1.1.0

# Enterprise Edition
kubectl -n kube-system set env daemonset/juicefs-csi-node -c juicefs-plugin JUICEFS_EE_MOUNT_IMAGE=juicedata/mount:ee-5.0.2-69f82b3
kubectl -n kube-system set env statefulset/juicefs-csi-controller -c juicefs-plugin JUICEFS_EE_MOUNT_IMAGE=juicedata/mount:ee-5.0.2-69f82b3
```

Also, don't forget to put these changes into `k8s.yaml`, to avoid losing these changes after the next installation. This is why we always recommend you use [Helm installation](../getting_started.md#helm) for production environments.

### Dynamic provisioning {#overwrite-in-sc}

CSI Driver allows [overriding the mount pod image in the StorageClass definition](#overwrite-in-sc), if you need to use different mount pod image for different applications, you'll need to create multiple StorageClass, and specify the desired mount pod image for each StorageClass.

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
  juicefs/mount-image: juicedata/mount:ce-v1.1.0
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
      juicefs/mount-image: juicedata/mount:ce-v1.1.0
```

## Build image

### Build mount pod image {#build-mount-pod-image}

JuiceFS CSI Driver adopt a [decoupled architecture](../introduction.md#architecture), mount pod image defaults to [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount), and community edition built using [`docker/ce.juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/ce.juicefs.Dockerfile).

If you were to build your own mount pod image, refer to below code to clone the JuiceFS Community Edition repository, and then build the image using the provided Dockerfile:

```shell
# Clone JuiceFS Community Edition repository
git clone https://github.com/juicedata/juicefs
cd juicefs

# Switch to desired branch, or modify code as needed
git checkout ...

# The corresponding Dockerfile resides in the CSI Driver repository
curl -O https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/docker/ce.juicefs.Dockerfile

# Build the Docker image, and then push to your private registry
docker build -t registry.example.com/juicefs-csi-mount:ce-latest -f ce.juicefs.Dockerfile .
docker push registry.example.com/juicefs-csi-mount:ce-latest
```

To use the newly built image, refer to [Overwrite mount pod image](#overwrite-mount-pod-image).

### Build CSI Driver component image

JuiceFS CSI Controller / JuiceFS CSI Node pod image default to [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver), built using [`docker/csi.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/csi.Dockerfile).

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
