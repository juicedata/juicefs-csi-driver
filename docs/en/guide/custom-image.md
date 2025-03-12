---
title: Customize Container Image
sidebar_position: 6
---

This chapter describes how to overwrite the Mount Pod image and how to build the CSI Driver component image by yourself.

## Mount Pod image separation {#ce-ee-separation}

JuiceFS Client runs inside the Mount Pod, and JuiceFS provides [Community Edition](https://juicefs.com/docs/community/introduction) and [Enterprise Edition](https://juicefs.com/docs/cloud), for a long period of time, mount image contains both versions:

* `/usr/local/bin/juicefs`: JuiceFS Community Edition client
* `/usr/bin/juicefs`: JuiceFS Enterprise Edition client

To avoid misuse and reduce image size, from CSI Driver 0.19.0, separated images are provided for CE/EE. You can find the latest Mount Pod image in [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags). The image tag looks like:

:::tip
If you need to move the Mount Pod image from Docker Hub to another container registry, please refer to the [documentation](../administration/offline.md#copy-images).
:::

```shell
# Tag of community mount image begin with ce-
juicedata/mount:ce-v1.2.0

# Tag of enterprise mount image begin with ee-
juicedata/mount:ee-5.0.23-8c7c134

# Prior to 0.19.0, tag contains both CE and EE version string
# This won't be maintained and updated in the future
juicedata/mount:v1.0.3-4.8.3
```

## Overwrite Mount Pod image {#overwrite-mount-pod-image}

:::tip
The JuiceFS CSI Driver supports [smooth upgrade of Mount Pods](../administration/upgrade-juicefs-client.md#smooth-upgrade) starting from version 0.25.0. It is recommended to use this method to upgrade Mount Pods first.
:::

From JuiceFS CSI Driver 0.17.1 and above, modifying the default Mount Pod image is supported. CSI Driver offers flexible control over the scope, choose a method that suits your situation.

:::tip
With Mount Pod image overwritten, note that:

* Existing Mount Pods won't be affected, new images will run only if you rolling upgrade app Pods, or delete Mount Pod.
* By default, if you [upgrad CSI Driver](../administration/upgrade-csi-driver.md), it'll use the latest stable mount image included with the release. But if you overwrite the mount image using steps provided in this section, then it'll be a fixated config and no longer related to CSI Driver upgrades

:::

### Modify ConfigMap {#overwrite-in-configmap}

From JuiceFS CSI Driver 0.24.0 and above, you can easily change the image version in the global configuration:

```yaml title="values-mycluster.yaml"
globalConfig:
  mountPodPatch:
    - pvcSelector:
        matchLabels:
          custom-image: "true"
      eeMountImage: "juicedata/mount:ee-5.0.17-0c63dc5"
      ceMountImage: "juicedata/mount:ce-v1.2.0"
```

See: [Customize Mount Pod and Sidecar containers](./configurations.md#customize-mount-pod)

### Configure Mount Pod image globally {#overwrite-in-csi-node}

If you use Helm to manage CSI Driver, changing mount image is as simple as the following one-liner:

```yaml
defaultMountImage:
  # Community Edition
  ce: "juicedata/mount:ce-v1.2.0"
  # Enterprise Edition
  ee: "juicedata/mount:ee-5.0.10-10fbc97"
```

But if you use kubectl to directly install a `k8s.yaml`, you'll have to set environment variables into the CSI Driver components:

```shell
# Community Edition
kubectl -n kube-system set env daemonset/juicefs-csi-node -c juicefs-plugin JUICEFS_CE_MOUNT_IMAGE=juicedata/mount:ce-v1.2.0
kubectl -n kube-system set env statefulset/juicefs-csi-controller -c juicefs-plugin JUICEFS_CE_MOUNT_IMAGE=juicedata/mount:ce-v1.2.0

# Enterprise Edition
kubectl -n kube-system set env daemonset/juicefs-csi-node -c juicefs-plugin JUICEFS_EE_MOUNT_IMAGE=juicedata/mount:ee-5.0.23-8c7c134
kubectl -n kube-system set env statefulset/juicefs-csi-controller -c juicefs-plugin JUICEFS_EE_MOUNT_IMAGE=juicedata/mount:ee-5.0.23-8c7c134
```

Also, don't forget to put these changes into `k8s.yaml`, to avoid losing these changes after the next installation. This is why we always recommend you use [Helm installation](../getting_started.md#helm) for production environments.

### Dynamic provisioning {#overwrite-in-sc}

:::tip
Starting from v0.24, CSI Driver can customize Mount Pods and sidecar containers in the [ConfigMap](#overwrite-in-configmap), legacy method introduced in this section is not recommended.
:::

CSI Driver allows [overriding the Mount Pod image in the StorageClass definition](#overwrite-in-sc), if you need to use different Mount Pod image for different applications, you'll need to create multiple StorageClass, and specify the desired Mount Pod image for each StorageClass.

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
  juicefs/mount-image: juicedata/mount:ce-v1.2.0
```

And then in PVC definitions, reference the needed StorageClass via the `storageClassName` field, so that you may use different Mount Pod image for different applications.

### Static provisioning

:::tip
Starting from v0.24, CSI Driver can customize Mount Pods and sidecar containers in the [ConfigMap](#overwrite-in-configmap), legacy method introduced in this section is not recommended.
:::

For [Static provisioning](./pv.md#static-provisioning), you'll have to configure Mount Pod image inside the PV definition.

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
      juicefs/mount-image: juicedata/mount:ce-v1.2.0
```

## Build image

### Build Mount Pod image {#build-mount-pod-image}

JuiceFS CSI Driver adopt a [decoupled architecture](../introduction.md#architecture), Mount Pod image defaults to [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount), and community edition built using [`docker/ce.juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/ce.juicefs.Dockerfile).

If you were to build your own Mount Pod image, refer to below code to clone the JuiceFS Community Edition repository, and then build the image using the provided Dockerfile:

```shell
# Clone JuiceFS Community Edition repository
git clone https://github.com/juicedata/juicefs
cd juicefs

# Switch to desired branch, or modify code as needed
git checkout ...

# The corresponding Dockerfile resides in the CSI Driver repository
curl -O https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/docker/dev.juicefs.Dockerfile

# Build the Docker image, and then push to your private registry
docker build -t registry.example.com/juicefs-csi-mount:ce-latest -f dev.juicefs.Dockerfile .
docker push registry.example.com/juicefs-csi-mount:ce-latest
```

To use the newly built image, refer to [Overwrite Mount Pod image](#overwrite-mount-pod-image).

### Build CSI Driver component image

JuiceFS CSI Controller / JuiceFS CSI Node Pod image default to [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver), built using [`docker/csi.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/csi.Dockerfile).

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
