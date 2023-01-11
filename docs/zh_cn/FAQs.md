---
title: 常见问题
slug: /faq
---

## 文档没能解答我的疑问

请首先尝试使用站内搜索功能（右上角），尝试用不同的关键词进行检索，如果文档始终未能解决你的疑问，你可以：

* 如果你使用 JuiceFS 社区版，加入 [JuiceFS 开源社区](https://juicefs.com/zh-cn/community)以寻求帮助。
* 如果你使用 JuiceFS 云服务，请通过[控制台](https://juicefs.com/console)右下角的 Intercom 会话联系 Juicedata 团队。

## 如何平滑重新挂载 JuiceFS 客户端？

特定情况下会需要重新挂载 JuiceFS 客户端，比方说在商业版 JuiceFS 私有部署中，控制台的地址发生迁移。此时需要重新运行 `juicefs auth` 命令，方可获得最新的元数据服务配置。在 Kubernetes 中，我们往往希望重新挂载的过程不影响业务，尽可能平滑。

不过注意，如果你修改了[「文件系统认证信息」](./guide/pv.md#volume-credentials)或者[「挂载参数」](./guide/pv.md#mount-options)，这种情况是无法用平滑重新挂载的方式令配置变更生效的，必须删除 PVC，重建应用 Pod。

目前而言，由于 Mount Pod 复用的设计，CSI 驱动并不支持真正的平滑重新挂载，但在以下情况，可以达成平滑重新挂载的效果：

* CSI 驱动进行了[升级](./administration/upgrade-csi-driver.md)，并且此次升级伴随着 Mount Pod 镜像更新，那么对应用进行滚动升级（或重启）时，CSI 驱动便会为其创建新的 Mount Pod。
* Mount Pod 并未被复用（并且未启用[「延迟删除」](./guide/resource-optimization.md#delayed-mount-pod-deletion)），也就是在每个节点上，Mount Pod 仅服务一个应用 Pod。这时如果对应用 Pod 进行滚动升级，CSI 驱动便会为其创建新的 Mount Pod，达到重新挂载的效果。删除重建此应用 Pod，也会触发重新创建 Mount Pod，虽然这样操作并不「平滑」。
