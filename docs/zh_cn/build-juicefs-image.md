---
sidebar_label: 构建 JuiceFS CSI Driver 镜像
---

# 如何自己构建 JuiceFS CSI Driver 镜像

如果需要自己修改 JuiceFS 代码，并构建 CSI Driver 镜像，可遵循如下步骤。

克隆 JuiceFS 仓库，并按需要修改代码：

```shell
git clone git@github.com:juicedata/juicefs.git
```

将 JuiceFS CSI Driver 仓库中的 [`dev.juicefs.Dockerfile`](https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/docker/dev.juicefs.Dockerfile) 文件拷贝到刚才克隆的路径下，执行以下命令构建镜像：

```bash
docker build -f dev.juicefs.Dockerfile .
```
