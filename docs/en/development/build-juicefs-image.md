---
slug: /build-juicefs-image
sidebar_label: Build the Container Image of JuiceFS CSI Driver
---

# How to build the container image of JuiceFS CSI Driver

The JuiceFS CSI Driver contains various types of components, and different components use different container images. The following describes how to build container image for specific component.

## Build container image of JuiceFS CSI Controller and JuiceFS CSI Node

The default container image used by JuiceFS CSI Controller and JuiceFS CSI Node is [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver), and the corresponding Dockerfile is [`docker/Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/Dockerfile).

The container image contains the latest version of JuiceFS Community Edition and JuiceFS Cloud Service client by default. If you want to modify code and build your own container image, you can execute the following commands:

```shell
IMAGE=foo/juicefs-csi-driver make image-dev
```

## Build the container image of JuiceFS Mount Pod

The default container image used by JuiceFS Mount Pod is [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount), and the corresponding Dockerfile is [`docker/juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile).

If you want to build your own image, you can follow these steps:

1. Clone the JuiceFS repository to the root directory of the JuiceFS CSI driver project and switch to the branch you want to compile or modify the code as needed:

   ```shell
   git clone git@github.com:juicedata/juicefs.git
   cd juicefs
   ```

2. Execute the following command to build the image:

   ```shell
   docker build -t foo/mount:latest -f ../docker/dev.juicefs.Dockerfile .
   ```

After the image is built, you can refer to [this document](../examples/mount-image.md) to specify the image of the Mount Pod in PV/StorageClass.
