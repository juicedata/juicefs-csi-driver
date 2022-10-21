# Quick reference
- **Maintained by**:
    [the JuiceFS Community](https://github.com/juicedata/juicefs)
- **Repository**:
    [Dockerfile](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile)
- **Where to get help**:
   [Documents](https://www.juicefs.com/docs/community/juicefs_on_docker/#mount-juicefs-in-docker), [the JuiceFS Community Slack](https://join.slack.com/t/juicefs/shared_invite/zt-n9h5qdxh-YD7e0JxWdesSEa9vY_f_DA), [Server Fault](https://serverfault.com/help/on-topic), [Unix & Linux](https://unix.stackexchange.com/help/on-topic), or [Stack Overflow](https://stackoverflow.com/help/on-topic)

# JuiceFS
JuiceFS is a high-performance shared file system designed for cloud-native use and released under the Apache License 2.0. It provides full [POSIX](https://en.wikipedia.org/wiki/POSIX) compatibility, allowing almost all kinds of object storage to be used locally as massive local disks and to be mounted and read on different cross-platform and cross-region hosts at the same time.

![JuiceFS Logo](https://www.juicefs.com/docs/img/logo.svg)

# How to use this image
Both of the community edition and the cloud service client are packaged in this image, the program paths are:
-   **Commnity Edition**: `/usr/local/bin/juicefs`
-   **Cloud Service**：`/usr/bin/juicefs`

The mirror provides the following labels:
-   **latest** - Latest stable version of the client included
-   **nightly** - Latest development branch client included

## Community Edition

### Create a volume

```shell
docker run --rm \
	juicedata/mount /usr/local/bin/juicefs format \
	--storage s3 \
	--bucket https://xxx.xxx.xxx \
	--access-key=ACCESSKEY \
	--secret-key=SECRETKEY \
	...
	redis://127.0.0.1/1 myjfs
```

### Mount a volume

```shell
docker run --name myjfs -d \
	juicedata/mount /usr/local/bin/juicefs mount \
	...
	redis://127.0.0.1/1 myjfs /mnt
```

## Cloud Service

```shell
docker run --name myjfs -d \
	juicedata/mount /usr/bin/juicefs mount \
	--token xxxx \
	--access-key=ACCESSKEY \
	--secret-key=SECRETKEY \
	...
	myjfs /mnt
```

## An example for Docker compose

[Mapping the mount point in container to the host](https://www.juicefs.com/docs/community/juicefs_on_docker#mapping-the-mount-point-in-container-to-the-host)