---
title: 升级 JuiceFS 客户端
slug: /upgrade-juicefs-client
sidebar_position: 3
---

我们推荐你定期升级 JuiceFS 客户端，以享受到最新特性和问题修复，请参考[「社区版客户端发布说明」](https://github.com/juicedata/juicefs/releases)或[「云服务客户端发布说明」](https://juicefs.com/docs/zh/cloud/release)了解更多版本信息。

事实上，[「升级 JuiceFS CSI 驱动」](./upgrade-csi-driver.md)也会带来客户端更新，这是因为每次 CSI 驱动更新发版，都会例行在配置中采用最新版的 [Mount Pod 镜像](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v)，但如果你希望提前采纳最新版的 Mount Pod，可以用本章介绍的方法单独升级 JuiceFS 客户端。

## 升级 Mount Pod 容器镜像 {#upgrade-mount-pod-image}

在 [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v) 找到新版 Mount Pod 容器镜像，然后[「修改 Mount Pod 容器镜像」](../guide/custom-image.md#overwrite-mount-pod-image)即可。

注意，覆盖 Mount Pod 容器镜像后，JuiceFS 客户端将不会随着[升级 CSI 驱动](./upgrade-csi-driver.md)而升级。你需要删除重建 PVC，才能令新的 Mount Pod 容器镜像生效。

## 临时升级 JuiceFS 客户端

:::tip 提示
强烈建议升级 JuiceFS CSI 驱动至 v0.10 及以后版本，此处介绍的客户端升级方法仅作为展示用途，不建议在生产环境中长期使用。
:::

如果你在使用进程挂载模式，或者仅仅是难以升级到 v0.10 之后的版本，但又需要使用新版 JuiceFS 进行挂载，那么也可以通过以下方法，在不升级 CSI 驱动的前提下，单独升级 CSI Node Service 中的 JuiceFS 客户端。

由于这是在 CSI Node Service 容器中临时升级 JuiceFS 客户端，完全是临时解决方案，可想而知，如果 CSI Node Service 的 Pod 发生了重建，又或是新增了节点，都需要再次执行该升级过程。

1. 使用以下脚本将 `juicefs-csi-node` pod 中的 `juicefs` 客户端替换为新版：

   ```bash
   #!/bin/bash

   # 运行前请替换为正确路径
   KUBECTL=/path/to/kubectl
   JUICEFS_BIN=/path/to/new/juicefs

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system cp $JUICEFS_BIN '{}':/tmp/juicefs -c juicefs-plugin

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system exec -i '{}' -c juicefs-plugin -- \
       chmod a+x /tmp/juicefs && mv /tmp/juicefs /bin/juicefs
   ```

2. 将应用逐个重新启动，或 kill 掉已存在的 pod。
