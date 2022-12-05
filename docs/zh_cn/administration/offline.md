---
title: 离线集群
sidebar_position: 5
---

所谓离线集群，就是节点无法访问公网，无法下载 CSI Controller 所需的组件容器镜像。如果你的节点无法顺利访问 [Docker Hub](https://hub.docker.com) 和 [Quay](https://quay.io)，也视为离线集群，按照本章所介绍的方法处理。

## 搬运镜像 {#copy-images}

为了搬运镜像，你需要一台能顺利访问外网的工作机，以及能够与之传输文件的集群内操作节点（具备 kubectl 管理员权限）。

### 准备资源

1. 在能访问外网的工作机上下载 Kubernetes 部署文件：

   ```shell
   # Kubernetes 1.18 及以后版本
   curl -LO https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

   # Kubernetes 1.18 之前版本
   curl -L -o k8s.yaml https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
   ```

2. 获取需要下载的镜像列表：

   ```shell
   grep -E ' *image:' k8s.yaml | sed 's/ *image: //' | sort | uniq > images.txt
   ```

   编写该文档时执行结果如下，随着迭代，结果可能有别：

   ```
   juicedata/juicefs-csi-driver:v0.17.3
   quay.io/k8scsi/csi-node-driver-registrar:v2.1.0
   quay.io/k8scsi/csi-provisioner:v1.6.0
   quay.io/k8scsi/livenessprobe:v1.1.0
   ```

   除此之外，由于 CSI 驱动的[「分离架构」](../introduction.md#architecture)，你还需要将 Mount Pod 镜像也纳入其中，一并搬运。你可以在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v) 中找到最新版，其版本号命名规则为 `v<社区版最新版本>-<云服务最新版本>`，编写此文档时，社区版最新版本为 1.0.2，云服务最新版本为 4.8.2，则用下方命令添加镜像：

   ```shell
   echo juicedata/mount:v1.0.2-4.8.2 >> images.txt
   ```

3. 将所有镜像下载到本地，并统一重命名：

   ```shell
   cat images.txt | xargs --max-procs=5 --max-lines=1 docker pull

   # 为了后续脚本中操作方便，将所有镜像重命名为 juicedata 这个分组下
   cat images.txt | awk '{image = $0; gsub("quay.io/k8scsi", "juicedata", image); print $0,image}' | xargs -L 1 docker tag
   sed -i 's@quay.io/k8scsi@juicedata@' images.txt
   ```

4. 将镜像导出到文件，传输到集群内，准备后续的导入操作：

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

   而如果你为离线集群搭建了内网镜像仓库，那么需要将镜像推送到该镜像仓库，请继续参考下方步骤进行镜像搬运。

2. 上传镜像到私有仓库

   将导入的镜像统一重命名：

   ```shell
   REGISTRY=registry.example.com:32000
   cat images.txt | awk "{print \$0,\"$REGISTRY/\"\$0}" | xargs -L 1 docker tag
   ```

   执行完后，确认 Docker 中已经载入以下镜像：

   ```
   $ docker image ls
   registry.example.com:32000/juicedata/juicefs-csi-driver:v0.10.5
   registry.example.com:32000/juicedata/csi-provisioner:v1.6.0
   registry.example.com:32000/juicedata/livenessprobe:v1.3.0
   registry.example.com:32000/juicedata/csi-node-driver-registrar:v1.1.0
   registry.example.com:32000/juicedata/mount:v1.0.0-4.8.0
   ```

   将镜像推送到私有仓库，请确保 Docker 有权限推送镜像：

   ```shell
   cat images.txt | sed "s@^@$REGISTRY/@" | xargs --max-procs=5 --max-lines=1 docker push
   ```

3. 更改 `k8s.yaml` 中的容器镜像地址

   将 `k8s.yaml` 中所有的容器镜像（`image:` 后面的内容）改成上一步中推送到私有仓库相应镜像：

   ```shell
   sed -i.orig \
     -e "s@juicedata/juicefs-csi-driver@$REGISTRY/juicedata/juicefs-csi-driver@g" \
     -e "s@quay.io/k8scsi@$REGISTRY/juicedata@g" \
     k8s.yaml
   ```

   :::note 注意
   由于修改了 Mount Pod 容器镜像的 tag，因此你需要一并更改 CSI 驱动设置，让 CSI 驱动从内网镜像仓库下载 Mount Pod 容器镜像。详见[覆盖默认容器镜像](../examples/mount-image.md#overwrite-mount-pod-image)。
   :::

至此，镜像搬运已经完成，请继续 CSI 驱动的安装。
