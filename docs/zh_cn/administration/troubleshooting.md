---
title: 问题排查方法
slug: /troubleshooting
sidebar_position: -2
---

阅读本章以了解如何对 JuiceFS CSI Driver 进行问题排查。不论面临何种错误，排查过程都需要你熟悉 CSI Driver 的各组件及其作用，因此继续阅读前，请确保你已了解 [JuiceFS CSI Driver 架构](../introduction.md)。

## 基础问题排查思路 {#troubleshooting}

在 JuiceFS CSI Driver 中，常见错误有两种：一种是 PVC 创建失败，属于 CSI Controller 的职责；另一种是 Pod 创建失败，属于 CSI Node 的职责。

### PVC 创建失败

在[「动态配置」](../guide/pv.md#dynamic-provisioning)下，PVC 创建之后，CSI Controller 会同时配合 Kubelet 自动创建 PV，在此期间，CSI Controller 会在 JuiceFS 文件系统中创建以 PV id 为名的子路径。

#### 查看 Event

一般而言，如果创建子路径失败，CSI Controller 会将错误结果存在 PVC event 中：

```
$ kubectl describe pvc dynamic-ce
Name:          dynamic-ce
…
Events:
  Type    Reason                 Age   From                                                                           Message
  ----    ------                 ----  ----                                                                           -------
  Normal  ExternalProvisioning   6s    persistentvolume-controller                                                    waiting for a volume to be created, either by external provisioner "csi.juicefs.com" or manually created by system administrator
  Normal  Provisioning           6s    csi.juicefs.com_juicefs-csi-controller-0_a842701e-bf46-4f4f-a298-16be30c45180  External provisioner is provisioning volume for claim "default/dynamic-ce"
  Normal  ProvisioningSucceeded  3s    csi.juicefs.com_juicefs-csi-controller-0_a842701e-bf46-4f4f-a298-16be30c45180  Successfully provisioned volume pvc-9987e624-48ce-4913-94c8-2ec0789c1994
```

#### 检查组件

若上一步没有明显报错，我们需要确认 CSI Controller 容器存活，且没有异常日志：

```
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          8d

$ kubectl -n kube-system logs juicefs-csi-controller-0 juicefs-plugin
```

### Pod 创建失败

CSI Node 的职责是挂载 JuiceFS，将客户端运行在 Mount Pod 中，并将挂载点 bind 到应用 pod 内。

#### 查看 event

同样，若挂载期间有报错，报错信息往往出现在应用 pod event 中：

```
$ kubectl describe po dynamic-ce-1
Name:         dynamic-ce
…
Events:
  Type     Reason       Age               From               Message
  ----     ------       ----              ----               -------
  Normal   Scheduled    53s               default-scheduler  Successfully assigned default/ce-static-1 to ubuntu-node-2
  Warning  FailedMount  4s (x3 over 37s)  kubelet            MountVolume.SetUp failed for volume "ce-static" : rpc error: code = Internal desc = Could not mount juicefs: juicefs status 16s timed out
```

#### 检查组件

首先，我们需要检查应用 pod 所在节点的 CSI Node 组件是否存活：

```
# 提前将应用 pod 信息存为环境变量
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 打印出所有 CSI Node pods
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver

# 打印应用 pod 所在节点的 CSI Node pod
kubectl -n kube-system get po -l app.kubernetes.io/name=juicefs-csi-driver --field-selector spec.nodeName=$(kubectl get po -ojsonpath={.spec.nodeName} -n $APP_NS $APP_POD_NAME)

# 将下方 $CSI_NODE_POD 替换为 CSI Node pod 名称，检查日志，确认有无异常
kubectl -n kube-system logs $CSI_NODE_POD -c juicefs-plugin

# 或者直接用一行命令打印出应用 pod 对应的 CSI Node pod 日志
kubectl -n kube-system logs $(kubectl -n kube-system get po -ojsonpath={..metadata.name} -l app.kubernetes.io/name=juicefs-csi-driver --field-selector spec.nodeName=$(kubectl get po -ojsonpath={.spec.nodeName} -n $APP_NS $APP_POD_NAME)) -c juicefs-plugin
```

#### 检查 Mount Pod

如果 Mount Pod 已经创建，可以查看其状态、event、日志等。

由于 CSI 驱动的分离架构，每个应用 pod 都对应着 mount pod（可复用），通过应用 pod 定位到 mount pod 的步骤稍显繁琐，因此我们准备了一系列快捷命令，帮你方便地获取信息：

```
# 下方命令或多或少都需要反复引用应用信息，因此请提前存为环境变量

APP_NS=default  # 应用所在的 Kubernetes 命名空间
# 如果需要从 PVC 查找 mount pod，请提前将 PVC name 存为环境变量
PVC_NAME=dynamic-ce
# 如果需要从 pod 信息查找 mount pod，请提前将应用 pod 信息存为环境变量
APP_POD_NAME=example-app-xxx-xxx

# 从 PVC 查找 mount pod
kubectl -n kube-system get po -l app.kubernetes.io/name=juicefs-mount | grep $(kubectl -n $APP_NS get pvc $PVC_NAME -n $APP_NS -ojsonpath={.spec.volumeName})

# 通过应用 pod 找到 PV name
kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get pod $APP_POD_NAME -ojsonpath={..persistentVolumeClaim.claimName}) -n $APP_NS -ojsonpath={.spec.volumeName}

# 找到该应用 pod 对应的 mount pod
kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl get pod -ojsonpath={.spec.nodeName} -n $APP_NS $APP_POD_NAME) -l app.kubernetes.io/name=juicefs-mount | grep $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get pod $APP_POD_NAME -ojsonpath={..persistentVolumeClaim.claimName}) -n $APP_NS -ojsonpath={.spec.volumeName})

# 打印 mount pod 日志
kubectl -n kube-system logs $(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl get pod -ojsonpath={.spec.nodeName} -n $APP_NS $APP_POD_NAME) -l app.kubernetes.io/name=juicefs-mount | grep $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get pod $APP_POD_NAME -ojsonpath={..persistentVolumeClaim.claimName}) -n $APP_NS -ojsonpath={.spec.volumeName}) | awk '{print $1}')

# 找到该 PV 对应的所有 mount pod
kubectl -n kube-system get po -l app.kubernetes.io/name=juicefs-mount | grep $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get pod $APP_POD_NAME -ojsonpath={..persistentVolumeClaim.claimName}) -n $APP_NS -ojsonpath={.spec.volumeName})
```

## 寻求帮助

如果自行排查无果，可能需要寻求社区，或者 Juicedata 团队帮助，这时需要你先行采集一些信息，帮助后续的分析排查。

### 查看 JuiceFS CSI 驱动版本

通过以下命令获取当前版本：

```shell
kubectl -n kube-system get pod -l app=juicefs-csi-controller -o jsonpath="{.items[*].spec.containers[*].image}"
```

以上命令会有类似 `juicedata/juicefs-csi-driver:v0.13.2` 这样的输出，最后的 `v0.13.2` 即为 JuiceFS CSI 驱动的版本。

### 诊断脚本

你也可以使用[诊断脚本](https://github.com/juicedata/juicefs-csi-driver/blob/master/scripts/diagnose.sh)来收集日志及相关信息。

1. 在你的集群中可以执行 `kubectl` 的节点上，下载诊断脚本

   ```shell
   wget https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/diagnose.sh
   ```

2. 给脚本添加执行权限

   ```shell
   chmod a+x diagnose.sh
   ```

3. 使用诊断脚本来收集信息。比如，你的 JuiceFS CSI Driver 部署在 `kube-system` 这个 namespace 下，并且你想收集 `kube-node-2` 这台节点上的信息。

   ```shell
   $ ./diagnose.sh
   Usage:
       ./diagnose.sh COMMAND [OPTIONS]
   COMMAND:
       help
           Display this help message.
       collect
           Collect pods logs of juicefs.
   OPTIONS:
       -no, --node name
           Set the name of node.
       -n, --namespace name
           Set the namespace of juicefs csi driver.

   $ ./diagnose.sh -n kube-system -no kube-node-2 collect
   Start collecting, node-name=kube-node-2, juicefs-namespace=kube-system
   ...
   please get diagnose_juicefs_1628069696.tar.gz for diagnostics
   ```

   所有相关的信息都被收集和打包在了一个压缩包里。
