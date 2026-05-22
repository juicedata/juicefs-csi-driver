---
title: Customize Container Image
sidebar_position: 7
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
juicedata/mount:ce-v1.3.1

# Tag of enterprise mount image begin with ee-
juicedata/mount:ee-5.3.8-fc708b6

# Prior to 0.19.0, tag contains both CE and EE version string
# This won't be maintained and updated in the future
juicedata/mount:v1.0.3-4.8.3
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

To use the newly built image, refer to [Upgrade JuiceFS Client](../administration/upgrade-juicefs-client.md).

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
