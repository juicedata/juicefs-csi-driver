# 升级 JuiceFS CSI Driver

## CSI Driver v0.10 及以上版本

JuiceFS CSI Driver 从 v0.10.0 开始将 JuiceFS 客户端与 CSI Driver 进行了分离，升级 CSI Driver 将不会影响已存在的 PV。

### v0.13.0

v0.13.0 相比于 v0.12.0 更新了 CSI node & controller 的 ClusterRole，不可以直接更新 image。 

1. 请将您的 `k8s.yaml` 修改为要 [v0.13.0 版本](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml) ，然后运行 `kubectl apply -f k8s.yaml`。
2. 如果你的 JuiceFS CSI Driver 是使用 Helm 安装的，也可以通过 Helm 对其进行升级：

   ```bash
   helm repo update
   helm upgrade juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

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

如果你想从 CSI Driver v0.9.0 升级到 v0.10.0 及以上版本，请参考[这篇文档](upgrade-csi-driver-from-0.9-to-0.10.md)。

### 其他

对于 v0.10.0 之前的版本，可以不升级 CSI Driver 仅升级 JuiceFS 客户端，详情参考[这篇文档](upgrade-juicefs.md)。

访问 [Docker Hub](https://hub.docker.com/r/juicedata/juicefs-csi-driver) 查看更多版本信息。
