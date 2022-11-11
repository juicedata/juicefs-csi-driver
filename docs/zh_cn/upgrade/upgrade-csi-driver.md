---
slug: /upgrade-csi-driver
---

# 升级 JuiceFS CSI 驱动

请查看 JuiceFS CSI 驱动的[发布说明](https://github.com/juicedata/juicefs-csi-driver/releases)页面了解所有已发布版本的信息。

## CSI 驱动 v0.10 及以上版本

JuiceFS CSI 驱动从 v0.10.0 开始将 JuiceFS 客户端与 CSI 驱动进行了分离，升级 CSI 驱动将不会影响已存在的 PV。

### 通过 Helm 升级

请依次运行以下命令以升级 JuiceFS CSI 驱动：

```bash
helm repo update
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### 通过 kubectl 升级

请将 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml) 中 JuiceFS CSI 驱动的镜像标签修改为需要升级的版本（如 `v0.14.0`），然后运行以下命令：

```sh
kubectl apply -f ./k8s.yaml
```

## CSI 驱动 v0.10 以下版本

### 小版本升级

升级 CSI 驱动需要重启 `DaemonSet`。由于 v0.10.0 之前的版本所有的 JuiceFS 客户端都运行在 `DaemonSet` 中，重启的过程中相关的 PV 都将不可用，因此需要先停止相关的 pod。

1. 停止所有使用此驱动的 pod。
2. 升级驱动：
    * 如果您使用的是 `latest` 标签，只需运行 `kubectl rollout restart -f k8s.yaml` 并确保重启 `juicefs-csi-controller` 和 `juicefs-csi-node` pod。
    * 如果您已固定到特定版本，请将您的 `k8s.yaml` 修改为要更新的版本，然后运行 `kubectl apply -f k8s.yaml`。
    * 如果你的 JuiceFS CSI 驱动是使用 Helm 安装的，也可以通过 Helm 对其进行升级。
3. 启动 pod。

### 跨版本升级

如果你想从 CSI 驱动 v0.9.0 升级到 v0.10.0 及以上版本，请参考[这篇文档](upgrade-csi-driver-from-0.9-to-0.10.md)。

### 其他

对于 v0.10.0 之前的版本，可以不升级 CSI 驱动仅升级 JuiceFS 客户端，详情参考[这篇文档](upgrade-juicefs.md)。
