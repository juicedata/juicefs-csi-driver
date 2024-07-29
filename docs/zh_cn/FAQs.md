---
title: 常见问题
slug: /faq
---

## 文档没能解答我的疑问

请首先尝试使用站内搜索功能（右上角），尝试用不同的关键词进行检索，如果文档始终未能解决你的疑问，你可以：

* 使用我们的 [kubectl-jfs-plugin](./administration/troubleshooting.md#kubectl-plugin) kubectl 插件自助排查；
* 如果你使用 JuiceFS 社区版，加入 [JuiceFS 开源社区](https://juicefs.com/zh-cn/community)以寻求帮助；
* 如果你使用 JuiceFS 云服务，请通过[控制台](https://juicefs.com/console)右下角的 Intercom 会话联系 Juicedata 团队。

## 如何平滑重新挂载 JuiceFS 文件系统？ {#seamless-remount}

如果你并不关心服务中断，那么删除 Mount Pod 令其自动重建，就能达到重新挂载的效果（注意，如果没有启用[「挂载点自动恢复」](./guide/configurations.md#automatic-mount-point-recovery)，则应用容器也需要重启或重建，才能恢复容器中的挂载点）。不过在 Kubernetes 中，我们往往希望重新挂载的过程不影响业务，尽可能平滑。比如以下操作，均能实现平滑重新挂载的效果：

* [升级或降级](./administration/upgrade-csi-driver.md) CSI 驱动，并且需要伴随着 Mount Pod 镜像的变更，那么对应用进行滚动升级（或重启）时，CSI 驱动便会为其创建新的 Mount Pod。
* 在 PV 级别对[「挂载参数」](./guide/configurations.md#mount-options)进行调整，然后滚动重启或升级应用 Pod。注意，对于动态配置，虽然可以在 [StorageClass](./guide/pv.md#create-storage-class)下修改挂载参数，但修改之后，改动并不会反映到已创建的 PV 上，因此对于动态配置，即便在 StorageClass 中修改挂载参数，滚升也不会引发 Mount Pod 重建。
* 对[「文件系统认证信息」](./guide/pv.md#volume-credentials)进行修改，然后滚动重启或升级应用 Pod。
* 如果并没有修改任何配置，那么滚动重启或升级应用 Pod 时，CSI 驱动是不会重新挂载的。这种情况下如果也希望触发重新挂载的效果，可以对挂载参数进行一些无关紧要的微调（比如稍稍修改 `cache-size`），然后滚动重启或升级应用 Pod。

欲了解 CSI 驱动什么情况下会为应用 Pod 创建新的 Mount Pod，达到平滑重新挂载的效果，请参考 [`pkg/juicefs/mount/pod_mount.go`](https://github.com/juicedata/juicefs-csi-driver/blob/master/pkg/juicefs/mount/pod_mount.go)中的 `GenHashOfSetting()` 方法，正是该方法的计算结果决定着是否创建新的 Mount Pod。

## `exec format error` {#format-error}

如果运行容器出现此类错误，最常见的问题是 CPU 架构不同。比如在 x86 机器上运行 ARM64 平台的镜像。因此务必注意镜像的 CPU 架构需要匹配，避免出现类似在 Mac 上搬运镜像、推送到私有仓库，然后在 x86 机器上运行这些镜像的错误。如果你必须使用不同于运行环境的工作电脑来拉取镜像，那么务必注意指定镜像架构：

```shell
# 将 platform 设置为实际运行环境的架构
docker pull --platform=linux/amd64 juicedata/mount:ee-5.0.17-0c63dc5
```

除了因架构问题产生此类错误，我们也见过因容器运行环境异常而导致的此类错误。在这种情况下，容器镜像和宿主机的确是相同的架构，但依然出现 `exec format error` 错误，此时需要卸载重装容器运行环境，以 containerd 为例：

```shell
systemctl stop kubelet
rm -rf /var/lib/containerd
# 重装 containerd
```

## MountPod 一直处于 pending 状态

使用 `kubectl describe <MountPod Name>` 查看当前 Pod Event。

可能的原因：

- 节点资源是否足够

```
kubectl describe node <nodeName>
```

节点资源不足时，便无法启动。此时需要根据实际情况[调整 Mount Pod 资源声明](./guide/resource-optimization.md#mount-pod-resources)，或者扩容宿主机。

- 集群 IP 资源是否充足

mount pod 默认以 `HostNetwork: false` 的形式启动，可能会占用大量的集群 IP 资源，如果集群资源 IP 不足可能会导致 mount pod 启动不成功。

联系对应的云厂商扩容，或者使用 `HostNetwork: true` 形式启动 mount pod，参阅：[定制 Mount Pod 和 Sidecar 容器](./guide/configurations.md#customize-mount-pod)

## MountPod 没有创建

使用 `kubectl describe <App Pod Name>` 查看当前业务 Pod Event。

确认已经进入挂载流程，而不是调度失败或者其他非 mount 错误。

- `driver name csi.juicefs.com not found` 或者 `csi.sock no such file`

  检查对应节点上的 `csi-node` 是否运行正常

- `Unable to attach or mount volumes: xxx`

  查看对应的 CSI Node 日志中 过滤出对应 PV 的相关日志

  如果没有找到类似于 `NodepublishVolume: volume_id is <pv name>` 日志，并且 K8s 版本低于 `v1.26.0`, `1.25.1`, `1.24.5`, `1.23.11` 可能是因为 kubelet 的一个 bug 导致没有触发 volume publish 请求，详见 [#109047](https://github.com/kubernetes/kubernetes/issues/109047)

  此时可以尝试
  - 重启 kubelet
  - 联系对应的云厂商或者运维。

  总之 JuiceFS CSI 需要收到请求才能开始挂载流程。
