---
sidebar_label: Build the Container Image of JuiceFS CSI Driver
---

# How to build the container image of JuiceFS CSI Driver

The JuiceFS CSI Driver contains various types of components, and different components use different container images. The following describes how to build container image for specific component.

## Build container image of JuiceFS CSI Controller and JuiceFS CSI Node

The default container image used by JuiceFS CSI Controller and JuiceFS CSI Node is [`juicedata/juicefs-csi-driver`](https://hub.docker.com/r/juicedata/juicefs-csi-driver), and the corresponding Dockerfile is [`docker/Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/Dockerfile). You can build a container image with the following command:

```shell
make image-latest
```

After the build is completed, a container image called `juicedata/juicefs-csi-driver:latest` will be generated. If you want to change the name of the container image, you can set the `IMAGE` environment variable:

```shell
IMAGE=foo/juicefs-csi-driver make image-latest
```

The container image contains the latest version of JuiceFS Community Edition and JuiceFS Cloud Service client by default. If you want to use a different version of the JuiceFS client, you can follow the steps below:

1. Clone the JuiceFS repository to the root directory of the JuiceFS CSI Driver project and switch to the branch you want to compile or modify the code as needed:

   ```shell
   git clone git@github.com:juicedata/juicefs.git
   cd juicefs
   ```

2. Execute the following command to build the image:

   ```shell
   docker build -t foo/juicefs-csi-driver:latest -f ../docker/dev.juicefs.Dockerfile .
   ```

## Build the container image of JuiceFS Mount Pod

The default container image used by JuiceFS Mount Pod is [`juicedata/mount`](https://hub.docker.com/r/juicedata/mount), and the corresponding Dockerfile is [`docker/juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile). You can build a container image with the following command:

```shell
make juicefs-image
```

The build will generate a container image called `juicedata/mount:v<JUICEFS-CE-LATEST-VERSION>-<JUICEFS-EE-LATEST-VERSION>`, where `<JUICEFS-CE-LATEST-VERSION>` means The latest version number of JuiceFS Community Edition client (e.g. `1.0.0`), `<JUICEFS-EE-LATEST-VERSION>` represents the latest version number of JuiceFS Cloud Service client (e.g. `4.8.0`). If you want to change the name of the container image you can set the `JUICEFS_IMAGE` environment variable:

```shell
JUICEFS_IMAGE=foo/mount make juicefs-image
```

The container image contains the latest version of JuiceFS Community Edition and JuiceFS Cloud Service client by default. If you want to use a different version of the JuiceFS client, you can set the `JUICEFS_REPO_URL` and `JUICEFS_REPO_REF` environment variables:

```shell
JUICEFS_REPO_URL=https://github.com/foo/juicefs JUICEFS_REPO_REF=v1.0.0 make juicefs-image
```
