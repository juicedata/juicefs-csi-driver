---
title: 离线集群
sidebar_position: 5
---

所谓离线集群，就是节点无法访问公网，无法下载 CSI Controller 所需的组件容器镜像。如果你的节点无法顺利访问 [Docker Hub](https://hub.docker.com) 和 [Quay](https://quay.io)，也视为离线集群，按照本章所介绍的方法处理。

## 用 Helm 离线安装 {#helm}

在访问 GitHub 不顺利的环境，运行 Helm 安装命令可能会出错：

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
Error: Get "https://github.com/juicedata/charts/releases/doownload/helm-chart-juicefs-csi-driver-0.20.0/juicefs-csi-driver-0.20.0.tgz": unexpected EOF
```

如果反复重试也于事无补，只好将 Helm chart 下载到本地，转移到集群的操作节点上，作为本地目录安装。步骤如下：

* 下载官方 [Helm chart](https://github.com/juicedata/charts) 到工作电脑，既可以下载压缩包，也可以直接 `git clone`。考虑到升级维护方便，更推荐后者；
* 将文档里介绍的安装命令，统统修改参数，改为以本地目录作为 chart 地址。

```shell
# 进入 chart 目录
cd juicefs-csi-driver

# 将安装目标改为 "." 代表安装当前目录的 chart，无需访问 GitHub 下载
helm upgrade --install juicefs-csi-driver . -n kube-system -f ./values-mycluster.yaml
```

## 搬运镜像 {#copy-images}

搬运镜像指的就是将镜像从公网下载到本地，然后上传至你的私有镜像仓库的过程。为了搬运镜像，你需要一台能顺利访问外网的工作机，以及能够与之传输文件的集群内操作节点（具备 kubectl 管理员权限）。

### 准备资源

1. 对于 Helm 安装方式（对于生产集群，这无疑是更为推荐的安装方式），可以直接用下方命令获取需要搬运的镜像列表：

  ```shell
  helm template . | grep -E ' *image:' | sed 's/ *image: //' | sort | uniq > images.txt
  ```

  而如果你直接使用 kubectl 进行安装，也可以使用类似上方的文本处理流程，提取出需要搬运的镜像列表。但由于生产集群[不推荐使用 kubectl 直接安装 `k8s.yaml`](./upgrade-csi-driver.md#kubectl-upgrade)，此处不过多介绍。

  上述步骤完成后，`images.txt` 中就已经包含了所有需要搬运的镜像，编写该文档时执行结果如下：

  ``` title="image.txt"
  juicedata/juicefs-csi-driver:v0.21.0
  registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.9.0
  registry.k8s.io/sig-storage/csi-provisioner:v2.2.2
  registry.k8s.io/sig-storage/csi-resizer:v1.9.0
  registry.k8s.io/sig-storage/livenessprobe:v2.12.0
  ```

  除此之外，由于 CSI 驱动的[「分离架构」](../introduction.md#architecture)，你还需要将 Mount Pod 镜像也纳入其中，一并搬运。你可以在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags) 中找到最新版，用下方命令添加镜像：

  ```shell
  # 社区版镜像
  echo juicedata/mount:ce-v1.1.2 >> images.txt

  # 商业版镜像
  echo juicedata/mount:ee-5.0.17-8ba7611 >> images.txt
  ```

1. 将所有镜像下载到本地，并统一重命名。注意**拉取镜像的机器，CPU 架构需要和线上环境相同**，否则需要使用 [`--platform`](https://docs.docker.com/engine/reference/commandline/pull/#options) 参数指定运行环境。

  ```shell
  cat images.txt | xargs --max-procs=10 --max-lines=1 docker pull --platform=linux/amd64

  # 为了后续脚本中操作方便，将所有镜像重命名为 juicedata 这个分组下
  cat images.txt | awk '{image = $0; gsub("quay.io/k8scsi", "juicedata", image); print $0,image}' | xargs -L 1 docker tag
  sed -i 's@quay.io/k8scsi@juicedata@' images.txt
  ```

  不同 CPU 架构的镜像无法跨环境使用，会出现 `exec user process caused: exec format error` 的容器启动报错，因此务必注意镜像的 CPU 架构需要匹配。

1. 如果你拉取机器的工作机无法直接推送镜像到私有镜像仓库，那么需要将镜像导出到文件，传输到集群内，准备后续的导入操作：

  ```shell
  # 将所有镜像统一输出打包
  cat images.txt | xargs docker image save -o juicefs-k8s-images.tar

  # 将所有牵涉的资源打包，准备传输
  tar -czf juicefs-k8s.tar.gz juicefs-k8s-images.tar k8s.yaml images.txt

  # 自行将压缩包传输到 Kubernetes 集群的 master 节点，或者其他具备 kubectl 管理员操作权限的节点
  scp juicefs-k8s.tar.gz ...
  ```

### 导入资源

1. 将容器镜像导入到本地，以 Docker 为例：

  ```shell
  mkdir juicefs-k8s && tar -xzf juicefs-k8s.tar.gz -C juicefs-k8s && cd juicefs-k8s
  docker image load -i juicefs-k8s-images.tar
  ```

  如果该离线集群不使用私有镜像仓库，那么上边的步骤便需要在所有 Kubernetes 节点上运行。除此外，你还需要保证这些镜像会持续存在于节点上，不会被意外清理（比如 [kubelet 默认在磁盘空间高于 80% 的时候清理镜像](https://kubernetes.io/zh-cn/docs/concepts/architecture/garbage-collection/#containers-images)）。

  而如果你为离线集群搭建了内网镜像仓库，那么需要将镜像推送到该镜像仓库，继续参考下方步骤进行镜像搬运。

1. 上传镜像到私有仓库

  将导入的镜像统一重命名：

  ```shell
  REGISTRY=registry.example.com:32000
  cat images.txt | awk "{print \$0,\"$REGISTRY/\"\$0}" | xargs -L 1 docker tag
  ```

  执行完后，确认 Docker 中已经载入以下镜像：

  ```shell
  $ docker image ls
  registry.example.com:32000/juicedata/juicefs-csi-driver:v0.10.5
  registry.example.com:32000/juicedata/csi-provisioner:v1.6.0
  registry.example.com:32000/juicedata/livenessprobe:v1.3.0
  registry.example.com:32000/juicedata/csi-node-driver-registrar:v1.1.0
  registry.example.com:32000/juicedata/mount:xx-xx
  ```

  将镜像推送到私有仓库，请确保 Docker 有权限推送镜像：

  ```shell
  cat images.txt | sed "s@^@$REGISTRY/@" | xargs --max-procs=10 --max-lines=1 docker push
  ```

1. 更改 `k8s.yaml` 中的容器镜像地址

  将 `k8s.yaml` 中所有的容器镜像（`image:` 后面的内容）改成上一步中推送到私有仓库相应镜像：

  ```shell
  sed -i.orig \
    -e "s@juicedata/juicefs-csi-driver@$REGISTRY/juicedata/juicefs-csi-driver@g" \
    -e "s@quay.io/k8scsi@$REGISTRY/juicedata@g" \
    k8s.yaml
  ```

  :::note
  由于修改了 Mount Pod 容器镜像的 tag，因此你需要一并更改 CSI 驱动设置，让 CSI 驱动从内网镜像仓库下载 Mount Pod 容器镜像。详见[覆盖默认容器镜像](../guide/custom-image.md#overwrite-mount-pod-image)。
  :::

至此，镜像搬运已经完成，请继续 CSI 驱动的安装。

## 修改 SA 以拉取镜像 {#mount-pod-sa}

离线集群往往使用私有镜像仓库，而私有仓库往往需要认证才能访问。Mount Pod 默认的 ServiceAccount（SA）是 `juicefs-csi-node-sa`，这个默认的用户可能没有权限从私有仓库拉取镜像，你可以按照下方步骤，为 Mount Pod 配置特定的 SA，让 Mount Pod 镜像能够正常拉取。

下方的示范中，假定 `juicefs-mount-sa` 这个 SA 已经恰当配置了拉取镜像的认证信息，你也可以将其替换为集群中真正有权限拉取镜像的 SA（参考 [Kubernetes 文档](https://kubernetes.io/zh-cn/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account)）。

### 静态配置

需要在 PV 定义中修改 `volumeAttributes` 字段，添加 `juicefs/mount-service-account`：

```yaml {10}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  ...
spec:
  csi:
    ...
    volumeAttributes:
      juicefs/mount-service-account: juicefs-mount-sa
  ...
```

### 动态配置

需要在 StorageClass 定义中修改 `parameters` 字段，添加 `juicefs/mount-service-account`：

```yaml {8}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  ...
  juicefs/mount-service-account: juicefs-mount-sa
```
