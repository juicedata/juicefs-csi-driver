---
title: 问题排查方法
slug: /troubleshooting
sidebar_position: 6
---

阅读本章以了解如何对 JuiceFS CSI 驱动进行问题排查。不论面临何种错误，排查过程都需要你熟悉 CSI 驱动的各个组件及其作用，因此继续阅读前，请确保你已了解 [JuiceFS CSI 驱动架构](../introduction.md#architecture)。

## 诊断脚本 {#csi-doctor}

推荐使用诊断脚本 [`csi-doctor.sh`](https://github.com/juicedata/juicefs-csi-driver/blob/master/scripts/csi-doctor.sh) 来收集日志及相关信息，本章所介绍的排查手段中，大部分采集信息的命令，都在脚本中进行了集成，使用起来更为便捷。

在集群中任意一台可以执行 `kubectl` 的节点上，安装诊断脚本：

```shell
wget https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/csi-doctor.sh
chmod a+x csi-doctor.sh
```

如果在你的运行环境中，kubectl 被重命名（比方说需要管理多个 Kubernetes 集群时，常常用不同 alias 来调用不同集群的 kubectl），或者并未放在 `PATH` 下，你也可以方便地修改脚步，将 `$kbctl` 变量替换为实际需要运行的 kubectl：

```shell
# 假设面对两个不同集群，kubectl 分别被别名为 kubectl_1 / kubectl_2
KBCTL=kubectl_1 ./csi-doctor.sh debug my-app-pod -n default
KBCTL=kubectl_2 ./csi-doctor.sh debug my-app-pod -n default
```

诊断脚本中最为常用的功能，就是方便地获取 Mount Pod 相关信息。假设应用 Pod 为 `default` 命名空间下的 `my-app-pod`：

```shell
# 获取指定应用 Pod 所用的 Mount Pod
$ ./csi-doctor.sh get-mount my-app-pod
kube-system juicefs-ubuntu-node-2-pvc-b94bd312-f5f7-4f46-afdb-2d1bc20371b5-whrrym

# 获取使用指定 Mount Pod 的所有应用 Pod
$ ./csi-doctor.sh get-app juicefs-ubuntu-node-2-pvc-b94bd312-f5f7-4f46-afdb-2d1bc20371b5-whrrym
default my-app-pod
```

在你熟读了[「基础问题排查原则」](#basic-principles)后，还可以使用 `csi-doctor.sh debug` 命令，来快速收集组件版本和日志信息。各类常见问题，均能在命令输出中找到排查线索：

```shell
./csi-doctor.sh debug my-app-pod -n default
```

运行上方命令，检查打印出来的丰富排查信息，用下方介绍的排查原则来进行诊断。同时，该命令控制输出内容的规模，你可以根据所使用的 JuiceFS 版本，方便地拷贝并发送给开源社区，或者 Juicedata 团队，进行后续排查。

## 基础问题排查原则 {#basic-principles}

在 JuiceFS CSI 驱动中，常见错误有两种：一种是 PV 创建失败，属于 CSI Controller 的职责；另一种是应用 Pod 创建失败，属于 CSI Node 和 Mount Pod 的职责。

### PV 创建失败

在[「动态配置」](../guide/pv.md#dynamic-provisioning)下，PVC 创建之后，CSI Controller 会同时配合 kubelet 自动创建 PV。在此期间，CSI Controller 会在 JuiceFS 文件系统中创建以 PV ID 为名的子目录（如果不希望以 PV ID 命名子目录，可以通过 [`pathPattern`](../guide/pv.md#using-path-pattern) 来调整）。

#### 查看 PVC 事件

一般而言，如果创建子目录失败，CSI Controller 会将错误结果存在 PVC 事件中：

```shell {7}
$ kubectl describe pvc dynamic-ce
...
Events:
  Type     Reason       Age                From               Message
  ----     ------       ----               ----               -------
  Normal   Scheduled    27s                default-scheduler  Successfully assigned default/juicefs-app to cluster-0003
  Warning  FailedMount  11s (x6 over 27s)  kubelet            MountVolume.SetUp failed for volume "juicefs-pv" : rpc error: code = Internal desc = Could not mount juicefs: juicefs auth error: Failed to fetch configuration for volume 'juicefs-pv', the token or volume is invalid.
```

#### 检查 CSI Controller {#check-csi-controller}

若 PVC 事件中并无错误信息，我们需要检查 CSI Controller 容器是否存活，以及是否存在异常日志：

```shell
# 检查 CSI Controller 是否存活
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          8d

# 检查 CSI Controller 日志是否存在异常信息
$ kubectl -n kube-system logs juicefs-csi-controller-0 juicefs-plugin
```

### 应用 Pod 创建失败

在 CSI 驱动的架构下，JuiceFS 客户端运行在 Mount Pod 中。因此每一个应用 pod 都伴随着一个对应的 Mount Pod。

CSI Node 会负责创建 Mount Pod 并在其中挂载 JuiceFS 文件系统，最终将挂载点 bind 到应用 pod 内。因此如果应用 pod 创建失败，既可能是 CSI Node 的问题，也可能是 Mount Pod 的问题，需要逐一排查。

#### 查看应用 Pod 事件

若挂载期间有报错，报错信息往往出现在应用 pod 事件中：

```shell {7}
$ kubectl describe po dynamic-ce-1
...
Events:
  Type     Reason       Age               From               Message
  ----     ------       ----              ----               -------
  Normal   Scheduled    53s               default-scheduler  Successfully assigned default/ce-static-1 to ubuntu-node-2
  Warning  FailedMount  4s (x3 over 37s)  kubelet            MountVolume.SetUp failed for volume "ce-static" : rpc error: code = Internal desc = Could not mount juicefs: juicefs status 16s timed out
```

通过应用 pod 事件确认创建失败的原因与 JuiceFS 有关以后，可以按照下面的步骤逐一排查。

#### 检查 CSI Node {#check-csi-node}

首先，我们需要检查应用 pod 所在节点的 CSI Node 容器是否存活，以及是否存在异常日志：

```shell
# 提前将应用 pod 信息存为环境变量
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 通过应用 pod 找到节点名
NODE_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}')

# 打印出所有 CSI Node pods
kubectl -n kube-system get po -l app=juicefs-csi-node

# 打印应用 pod 所在节点的 CSI Node pod
kubectl -n kube-system get po -l app=juicefs-csi-node --field-selector spec.nodeName=$NODE_NAME

# 将下方 $CSI_NODE_POD 替换为上一条命令获取到的 CSI Node pod 名称，检查日志，确认有无异常
kubectl -n kube-system logs $CSI_NODE_POD -c juicefs-plugin
```

或者直接用一行命令打印出应用 pod 对应的 CSI Node pod 日志（需要设置好 `APP_NS` 和 `APP_POD_NAME` 环境变量）：

```shell
kubectl -n kube-system logs $(kubectl -n kube-system get po -o jsonpath='{..metadata.name}' -l app=juicefs-csi-node --field-selector spec.nodeName=$(kubectl get po -o jsonpath='{.spec.nodeName}' -n $APP_NS $APP_POD_NAME)) -c juicefs-plugin
```

#### 检查 Mount Pod {#check-mount-pod}

如果 CSI Node 一切正常，则需要检查 Mount Pod 是否存在异常。

你可以方便地通过[诊断脚本](#csi-doctor)来定位到 Mount Pod，如果你需要脱离脚本、直接用 kubectl 进行排查，我们也准备了一系列快捷命令，帮你方便地获取信息：

```shell
# 如果情况不复杂，可以直接用下方命令打印所有 mount pod 错误日志
kubectl -n kube-system logs -l app.kubernetes.io/name=juicefs-mount | grep -v "<WARNING>" | grep -v "<INFO>"

# 提前将应用 pod 信息存为环境变量
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 通过应用 pod 找到节点名、PVC 名、PV 名
NODE_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}')
# 如果应用 pod 挂载了多个 PV，以下命令将只考虑首个，请按需调整。
PVC_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}')
PV_NAME=$(kubectl -n $APP_NS get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')
PV_ID=$(kubectl get pv $PV_NAME -o jsonpath='{.spec.csi.volumeHandle}')

# 找到该应用 pod 对应的 mount pod 名
MOUNT_POD_NAME=$(kubectl -n kube-system get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $PV_ID)

# 检查 mount pod 状态是否正常
kubectl -n kube-system get po $MOUNT_POD_NAME

# 打印 mount pod 事件
kubectl -n kube-system describe po $MOUNT_POD_NAME

# 打印 mount pod 日志（其中包含 JuiceFS 客户端日志）
kubectl -n kube-system logs $MOUNT_POD_NAME

# 打印 mount pod 的启动命令，这是一个较容易忽视的排查要点。
# 如果挂载选项（mountOptions）填写格式有误，则可能造成启动命令参数错误。
kubectl get pod -o jsonpath='{..containers[0].command}' $MOUNT_POD_NAME

# 找到该 PV 对应的所有 mount pod
kubectl -n kube-system get po -l app.kubernetes.io/name=juicefs-mount -o wide | grep $PV_ID
```

或者更为快捷地，直接用下方的单行命令：

```shell
# 需要设置好 APP_NS 和 APP_POD_NAME 环境变量

# 打印 mount pod 名称
kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')

# 进入 mount pod 中，交互式运行命令
kubectl -n kube-system exec -it $(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')) -- bash
```

### 性能问题

如果使用 CSI 驱动时，各组件均无异常，但却遇到了性能问题，则需要用到本节介绍的排查方法。

#### 查看实时统计数据以及访问日志 {#accesslog-and-stats}

JuiceFS 文件系统的根目录下有一些提供特殊功能的隐藏文件，假设挂载点为 `/jfs`：

* `cat /jfs/.accesslog` 实时打印文件系统的访问日志，用于分析应用程序对文件系统的访问模式，详见[「访问日志（社区版）」](https://juicefs.com/docs/zh/community/fault_diagnosis_and_analysis#access-log)和[「访问日志（云服务）」](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#oplog)。
* `cat /jfs/.stats` 打印文件系统的实时统计数据，当 JuiceFS 性能不佳时，可以通过实时统计数据判断问题所在。详见[「实时统计数据（社区版）」](https://juicefs.com/docs/zh/community/performance_evaluation_guide/#juicefs-stats)和[「实时统计数据（云服务）」](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#stats)。

在 CSI 驱动下获取应用的访问日志，步骤稍显繁琐，你需要先找到应用容器对应的 Mount Pod，再进入容器执行打印日志的命令。推荐直接使用 [`csi-doctor.sh get-oplog APP_POD_NAME`](#csi-doctor) 快速获得相应的命令，避免使用下方繁琐的手动步骤。

Mount Pod 会将 JuiceFS 根目录挂载到形如 `/var/lib/juicefs/volume/pvc-xxx-xxx-xxx-xxx-xxx-xxx` 的目录下，再通过 Kubernetes 的 bind 机制映射到容器内，因此，对于给定应用 Pod，你可以参考下方命令，找到其 PV 的宿主机挂载点，然后查看隐藏文件：

```shell
# 提前将应用 pod 信息存为环境变量
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 如果应用 pod 挂载了多个 PV，以下命令将只考虑首个，请按需调整。
PVC_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}')
PV_NAME=$(kubectl -n $APP_NS get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')

# 通过 PV 名查找宿主机挂载点
df -h | grep $PV_NAME

# 一行命令进入宿主机挂载点
cd $(df -h | grep $PV_NAME | awk '{print $NF}')

# 访问所需要的隐藏文件，进行相关排查
cat .accesslog
cat .stats
```

本节仅介绍如何在 CSI 驱动中定位到隐藏文件，而如何通过隐藏文件进行问题排查，是一个单独的话题，请参考以下内容：

* CSI 驱动问题排查案例：[读性能差](./troubleshooting-cases.md#bad-read-performance)
* 访问日志：[社区版](https://juicefs.com/docs/zh/community/fault_diagnosis_and_analysis#access-log)、[云服务](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#oplog)
* 实时统计数据：[社区版](https://juicefs.com/docs/zh/community/performance_evaluation_guide/#juicefs-stats)、[云服务](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#stats)

## 寻求帮助

如果自行排查无果，可能需要寻求社区，或者 Juicedata 团队帮助，这时需要你先行采集一些信息，帮助后续的分析排查。

使用[诊断脚本](#csi-doctor)的 `collect` 来打包收集信息，假设应用 Pod 为 `default` 命名空间下的 `my-app-pod`：

```shell
$ ./csi-doctor.sh collect my-app-pod -n default
Results have been compressed to my-app-pod.diagnose.tar.gz
```

所有相关的 Kubernetes 资源、日志、事件都被收集和打包在了一个压缩包里，然后将此压缩包发送给相关人员。
