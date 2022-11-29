---
title: 问题排查案例
slug: /troubleshooting-cases
sidebar_position: 6
---

这里收录常见问题的具体排查步骤，你可以直接在本文搜索报错关键字以检索问题。同时，我们也推荐你先掌握[「基础问题排查思路」](./troubleshooting.md#basic-principles)。

## CSI 驱动未安装 / 安装失败

如果 JuiceFS CSI 驱动压根没安装，或者配置错误导致安装失败，那么试图使用 JuiceFS CSI 驱动时，便会有下方报错：

```
driver name csi.juicefs.com not found in the list of registered CSI drivers
```

请回顾[「安装 JuiceFS CSI 驱动」](../getting_started.md)，尤其注意确认 kubelet 根目录正确设置。

## PVC 配置互相冲突，创建失败

常见情况比如：两个 pod 分别使用各自的 PVC，但只有一个能创建成功。

请检查每个 PVC 对应的 PV，每个 PV 的 `volumeHandle` 必须保证唯一。可以通过以下命令检查 `volumeHandle`：

```yaml {12}
$ kubectl get pv -o yaml juicefs-pv
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  ...
spec:
  ...
  csi:
    driver: csi.juicefs.com
    fsType: juicefs
    volumeHandle: juicefs-volume-abc
    ...
```

## 文件系统创建错误（社区版）

如果你选择在 mount pod 中动态地创建文件系统，也就是执行 `juicefs format` 命令，那么当创建失败时，应该会在 CSI Node pod 中看到如下错误：

```
format: ERR illegal address: xxxx
```

这里的 `format`，指的就是 `juicefs format` 命令，以上方的报错，多半是访问元数据引擎出现了问题，请检查你的安全组设置，确保所有 Kubernetes 集群的节点都能访问元数据引擎。

如果使用 Redis 作为元数据引擎，且启用了密码认证，那么可能遇到如下报错：

```
format: NOAUTH Authentication requested.
```

你需要确认元数据引擎 URL 是否正确填写了密码，具体格式请参考 [Redis 元数据引擎](https://juicefs.com/docs/zh/community/databases_for_metadata#redis)。
