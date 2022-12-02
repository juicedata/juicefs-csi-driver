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

## CSI Node pod 异常

如果 CSI Node pod 异常，与 kubelet 通信的 socket 文件不复存在，应用 pod 事件中会看到如下错误日志：

```
/var/lib/kubelet/csi-plugins/csi.juicefs.com/csi.sock: connect: no such file or directory
```

此时需要[检查 CSI Node](./troubleshooting.md#check-csi-node)，确认其异常原因，并排查修复。

## Mount Pod 异常

常见错误比如 Mount Pod 一直卡在 `Pending` 状态，导致应用容器也一并卡死在 `ContainerCreating` 状态。此时需要[查看 Mount Pod 事件](./troubleshooting.md#check-mount-pod)，确定症结所在。不过对于 `Pending` 状态，大概率是资源吃紧，导致容器无法创建。

另外，当节点 kubelet 开启抢占功能，Mount Pod 启动后可能抢占应用资源，导致 Mount Pod 和应用 Pod 均反复创建、销毁，在 Pod 事件中能看到以下信息：

```
Preempted in order to admit critical pod
```

Mount Pod 默认的资源声明是 1 CPU，1GiB 内存，节点资源不足时，便无法启动，或者启动后抢占应用资源。此时需要根据实际情况[调整 Mount Pod 资源声明](../guide/resource-optimization.md#mount-pod-resources)，或者扩容宿主机。

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

你需要确认元数据引擎 URL 是否正确填写了密码，具体格式请参考[「使用 Redis 作为元数据引擎」](https://juicefs.com/docs/zh/community/databases_for_metadata#redis)。
