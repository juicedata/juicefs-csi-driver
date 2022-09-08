# How to use customized image in Mount Pod

本文档展示了如何让 JuiceFS Mount Pod 使用自定义[镜像](https://kubernetes.io/zh-cn/docs/concepts/containers/images/)。默认的镜像为[`juicedata/mount:nightly`](https://hub.docker.com/r/juicedata/mount/tags)，为使 Mount Pod 能正常运行，请使用基于[`juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile)构建的镜像。

:::note 注意
若采用进程挂载的方式启动 CSI 驱动，即 CSI Node 和 CSI Controller 的启动参数使用 `--by-process=true`，则本文档的相关配置会被忽略。
:::

## 安装 CSI 时覆盖默认镜像

## Overwrite default image when installing CSI

## 在 PersistentVolume 中配置

## 在 StorageClass 中配置