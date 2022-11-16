---
slug: /upgrade-juicefs
---

# 独立升级 JuiceFS 客户端

对于 v0.10.0 之前的版本，可以通过以下方法单独升级 JuiceFS 客户端，无需升级 CSI 驱动。

1. 使用以下脚本将 `juicefs-csi-node` pod 中的 `juicefs` 客户端替换为新版：

   ```bash
   #!/bin/bash

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

   :::note 注意
   将脚本中 `/path/to/kubectl` 和 `/path/to/new/juicefs` 替换为实际的值，然后执行脚本。
   :::

2. 将应用逐个重新启动，或 kill 掉已存在的 pod。
