---
title: 常见问题
slug: /faq
---

## 文档没能解答我的疑问

请首先尝试使用站内搜索功能（右上角），尝试用不同的关键词进行检索，如果文档始终未能解决你的疑问，你可以：

* 如果你使用 JuiceFS 社区版，加入 [JuiceFS 开源社区](https://juicefs.com/zh-cn/community)以寻求帮助。
* 如果你使用 JuiceFS 云服务，请通过[控制台](https://juicefs.com/console)右下角的 Intercom 会话联系 Juicedata 团队。

## 如何平滑重新挂载 JuiceFS 文件系统？ {#seamless-remount}

某些情况下会需要重新挂载 JuiceFS 文件系统，比方说修改挂载参数、文件系统认证信息。如果你并不关心服务中断，那么删除 Mount Pod 令其自动重建，就能达到重新挂载的效果（注意，如果没有启用[「挂载点自动恢复」](./guide/pv.md#automatic-mount-point-recovery)，则应用容器也需要重启或重建，才能恢复容器中的挂载点）。不过在 Kubernetes 中，我们往往希望重新挂载的过程不影响业务，尽可能平滑。比如以下操作，均能实现平滑重新挂载的效果：

* [升级或降级](./administration/upgrade-csi-driver.md) CSI 驱动，并且需要伴随着 Mount Pod 镜像的变更，那么对应用进行滚动升级（或重启）时，CSI 驱动便会为其创建新的 Mount Pod。
* 对[「挂载参数」](./guide/pv.md#mount-options)进行调整，然后滚动重启或升级应用 Pod。
* 对[「文件系统认证信息」](./guide/pv.md#volume-credentials)进行修改，然后滚动重启或升级应用 Pod。
* 如果并没有修改任何配置，那么滚动重启或升级应用 Pod 时，CSI 驱动是不会重新挂载的。这种情况下如果也希望触发重新挂载的效果，可以对挂载参数进行一些无关紧要的微调（比如稍稍修改 `cache-size`），然后滚动重启或升级应用 Pod。

欲了解 CSI 驱动什么情况下会为应用 Pod 创建新的 Mount Pod，达到平滑重新挂载的效果，请参考 [`pkg/juicefs/mount/pod_mount.go`](https://github.com/juicedata/juicefs-csi-driver/blob/master/pkg/juicefs/mount/pod_mount.go)中的 `GenHashOfSetting()` 方法，正是该方法的计算结果决定着是否创建新的 Mount Pod。
