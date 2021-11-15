# 升级 CSI Driver

## CSI Driver v0.10 及以上版本

Juicefs CSI Driver 从 v0.10.0 开始分离了 JuiceFS client 客户端，升级 CSI Driver 不会影响已存在的 PV。如果你使用的是 CSI Driver v0.10.0 及以上的版本，执行以下命令进行升级：

* 如果您使用的是 `nightly` 标签，只需运行 `kubectl rollout restart -f k8s.yaml` 并确保重启 `juicefs-csi-controller` 和 `juicefs-csi-node` pod。
* 如果您已固定到特定版本，请将您的 `k8s.yaml` 修改为要更新的版本，然后运行 `kubectl apply -f k8s.yaml`。
* 如果你的 JuiceFS CSI Driver 是使用 Helm 安装的，也可以通过 Helm 对其进行升级。

## CSI Driver v0.10 以下版本

### 小版本升级

升级 CSI Driver 需要重启 `DaemonSet`。由于 v0.10.0 之前的版本所有的 JuiceFS 客户端都运行在 `DaemonSet` 中，重启的过程中相关的 PV 都将不可用，因此需要先停止相关的 pod。

1. 停止所有使用此驱动的 pod。
2. 升级驱动：
    * 如果您使用的是 `latest` 标签，只需运行 `kubectl rollout restart -f k8s.yaml` 并确保重启 `juicefs-csi-controller` 和 `juicefs-csi-node` pod。
    * 如果您已固定到特定版本，请将您的 `k8s.yaml` 修改为要更新的版本，然后运行 `kubectl apply -f k8s.yaml`。
    * 如果你的 JuiceFS CSI Driver 是使用 Helm 安装的，也可以通过 Helm 对其进行升级。
3. 启动 pod。

### 跨版本升级

如果你想从 CSI Driver v0.9.0 升级到 v0.10.0 及以上版本，请参考[这篇文档](docs/zh_cn/upgrade-csi-driver-from-0.9-to-0.10.md)。

### 其他

对于使用较低版本的用户，你还可以在不升级 CSI 驱动的情况下升级 JuiceFS 客户端，详情参考[这篇文档](docs/zh_cn/upgrade-juicefs.md)。

访问 [Docker Hub](https://hub.docker.com/r/juicedata/juicefs-csi-driver) 查看更多版本信息。
