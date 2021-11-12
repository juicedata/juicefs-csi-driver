# 升级 juicefs 二进制程序

如果我们只需要升级 JuiceFS 二进制文件，操作如下：

1. 使用下面这个脚本将 `juicefs-csi-node` pod 中的 `juicefs` 二进制文件替换成新的：

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

将 `/path/to/kubectl` 和 `/path/to/new/juicefs` 替换成您环境中的变量，然后执行脚本。

2. 将应用逐个重新启动，或 kill 掉已存在的 pod。
