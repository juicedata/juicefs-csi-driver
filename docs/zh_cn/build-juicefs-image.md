# 如何自己构建 JuiceFS CSI Driver 镜像

如果需要自己修改 JuiceFS 代码，并构建 CSI Driver 镜像，可遵循如下步骤。

clone JuiceFS 仓库，并按需要修改代码：

```shell
git clone git@github.com:juicedata/juicefs.git
```

将 `dev.juicefs.Dockerfile` 拷贝到刚才克隆的路径下，执行以下命令构建镜像：

```bash
docker build -f dev.juicefs.Dockerfile .
```
