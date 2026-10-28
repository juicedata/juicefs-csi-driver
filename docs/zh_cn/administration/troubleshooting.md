---
title: 问题排查方法
slug: /troubleshooting
sidebar_position: 6
---

阅读本章以了解如何对 JuiceFS CSI 驱动进行问题排查。不论面临何种错误，排查过程都需要你熟悉 CSI 驱动的各个组件及其作用，因此继续阅读前，请确保你已了解 [JuiceFS CSI 驱动架构](../introduction.md#architecture)。

## 排查工具 {#tools}

### CSI 控制台 {#csi-dashboard}

Dashboard 将 CSI 场景下各种排查操作汇集在网页 UI 上，使用起来非常方便，是我们最推荐的排查方式。请阅读[「CSI Dashboard」](../guide/dashboard.md)。

### kubectl 插件 {#kubectl-plugin}

对于不方便使用 [CSI Dashboard](../guide/dashboard.md) 的环境，可以使用 [`kubectl-jfs`](./kubectl-jfs-plugin.md) 插件在命令行中查看 JuiceFS 资源、诊断 Pod/PVC/PV、采集访问日志、预热缓存，以及升级 Mount Pod 或 Sidecar。

使用一键安装脚本安装最新版插件：

```shell
# 默认安装到 /usr/local/bin
curl -sSL https://d.juicefs.com/kubectl-jfs-install | sh -
```

安装后，可以直接诊断有问题的应用 Pod：

```shell
kubectl jfs debug pod <pod-name> --namespace <namespace>
```

详细的安装说明、命令示例、批量升级流程和注意事项参见[「使用 kubectl-jfs 插件」](./kubectl-jfs-plugin.md)。

### 诊断脚本（不推荐） {#csi-doctor}

:::note
由于已经有功能更为强大的 [kubectl 插件](#kubectl-plugin)，本小节介绍的脚本已经不再推荐使用。
:::

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

在[「动态配置」](../guide/pv.md#dynamic-provisioning)下，PVC 创建之后，CSI Controller 会同时配合 kubelet 自动创建 PV。在此期间，CSI Controller 会在 JuiceFS 文件系统中创建以 PV ID 为名的子目录（如果不希望以 PV ID 命名子目录，可以通过 [`pathPattern`](../guide/configurations.md#using-path-pattern) 来调整）。

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

在 CSI 驱动的架构下，JuiceFS 客户端运行在 Mount Pod 中。因此每一个应用 Pod 都伴随着一个对应的 Mount Pod。

CSI Node 会负责创建 Mount Pod 并在其中挂载 JuiceFS 文件系统，最终将挂载点 bind 到应用 Pod 内。因此如果应用 Pod 创建失败，既可能是 CSI Node 的问题，也可能是 Mount Pod 的问题，需要逐一排查。

#### 查看应用 Pod 事件

若挂载期间有报错，报错信息往往出现在应用 Pod 事件中：

```shell {7}
$ kubectl describe po dynamic-ce-1
...
Events:
  Type     Reason       Age               From               Message
  ----     ------       ----              ----               -------
  Normal   Scheduled    53s               default-scheduler  Successfully assigned default/ce-static-1 to ubuntu-node-2
  Warning  FailedMount  4s (x3 over 37s)  kubelet            MountVolume.SetUp failed for volume "ce-static" : rpc error: code = Internal desc = Could not mount juicefs: juicefs status 16s timed out
```

通过应用 Pod 事件确认创建失败的原因与 JuiceFS 有关以后，可以按照下面的步骤逐一排查。

#### 检查 CSI Node {#check-csi-node}

首先，我们需要检查应用 Pod 所在节点的 CSI Node 容器是否存活，以及是否存在异常日志：

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

或者直接用一行命令打印出应用 Pod 对应的 CSI Node Pod 日志（需要设置好 `APP_NS` 和 `APP_POD_NAME` 环境变量）：

```shell
kubectl -n kube-system logs $(kubectl -n kube-system get po -o jsonpath='{..metadata.name}' -l app=juicefs-csi-node --field-selector spec.nodeName=$(kubectl get po -o jsonpath='{.spec.nodeName}' -n $APP_NS $APP_POD_NAME)) -c juicefs-plugin
```

#### 检查 Mount Pod {#check-mount-pod}

如果 CSI Node 一切正常，则需要检查 Mount Pod 是否存在异常。

你可以方便地通过 [`kubectl jfs pod`](./kubectl-jfs-plugin.md#查看-juicefs-资源) 命令来快速定位到应用 Pod 对应的 Mount Pod：

```shell
$ kubectl jfs po --all-namespaces
NAME               NAMESPACE    MOUNT PODS               STATUS   AGE
juicefs-app        default      juicefs-k8s-001-pvc-xxx  Running  6s
```

根据上方打印出来的 Mount Pod 名称，用 `kubectl describe` 和 `kubectl logs --all-containers` 命令查看容器事件和日志，进行深入排查。

如果你需要脱离脚本、直接用 kubectl 进行排查，我们也准备了一系列快捷命令，帮你方便地获取信息：

```shell
# 通过 App Pod 反查 Mount Pod，可以通过下方命令先提取 Pod UID
# 注意替换最后的 juicefs-app 为实际应用 Pod 名称
k get po -n default -ojsonpath='{..metadata.uid}{"\n"}' juicefs-app
# 复制上一条命令打印的 UID，然后在结果中搜索，UID 匹配的记录，就是该应用 Pod 所关联的 Mount Pod
kubectl get pods -o=custom-columns=NAME:.metadata.name,ANNOTATIONS:.metadata.annotations -l app.kubernetes.io/name=juicefs-mount -n kube-system|grep UID

# 定位到 Mount Pod 以后，查看 Events 和日志
kubectl -n kube-system describe [Mount Pod Name]
kubectl -n kube-system logs --all-containers [Mount Pod Name]

# 如果有需要，还可以直接用下方命令打印所有 Mount Pod 错误日志
kubectl -n kube-system logs -l app.kubernetes.io/name=juicefs-mount | grep -v "<WARNING>" | grep -v "<INFO>"
```

#### 排查 Mount Pod {#debug-mount-pod}

一个处于 `CrashLoopBackOff` 状态的容器是无法进行交互式排查的，此时可以使用 `kubectl debug` 命令，创建一个可交互排查的副本：

```shell
kubectl -n <namespace> debug <mount-pod> -it  --copy-to=jfs-mount-debug --container=jfs-mount --image=<mount-image> -- bash
```

在上方示范中，`<mount-image>` 设置为该 Mount Pod 的镜像，这样一来，`debug` 命令会创建一个一模一样的专供交互式排查的容器，你可以在这个环境中尝试复现、排查问题。

排查完毕以后，记得清理环境：

```shell
kubectl -n <namespace> delete po jfs-mount-debug
```

### 性能问题

如果使用 CSI 驱动时，各组件均无异常，但却遇到了性能问题，则需要用到本节介绍的排查方法。

#### 查看实时统计数据以及访问日志 {#accesslog-and-stats}

JuiceFS 文件系统的根目录下有一些提供特殊功能的隐藏文件，假设挂载点为 `/jfs`：

* `cat /jfs/.accesslog` 实时打印文件系统的访问日志，用于分析应用程序对文件系统的访问模式，详见[「访问日志（社区版）」](https://juicefs.com/docs/zh/community/fault_diagnosis_and_analysis#access-log)和[「访问日志（云服务）」](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#oplog)
* `juicefs stats -l 1 /jfs` 打印文件系统的实时统计数据，当 JuiceFS 性能不佳时，可以通过实时统计数据判断问题所在。详见[「实时统计数据（社区版）」](https://juicefs.com/docs/zh/community/performance_evaluation_guide/#juicefs-stats)和[「实时统计数据（云服务）」](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#stats)

如果已经安装了 CSI Dashboard，则可直接通过网页功能采集日志和 `stats` 信息：

![dashboard-mount-troubleshooting](../images/dashboard-mount-pod-troubleshooting.png)

对于 JuiceFS 企业版用户，如果并未安装 CSI Dashboard，也可以直接通过控制台进入文件系统的客户端列表页，点击最右侧对应的按钮直接采集和下载日志：

![web-console-access-log-button](../images/web-console-access-log-button.png)

如果无法通过网页功能完成上述功能，还可以使用 [`kubectl jfs accesslog`](./kubectl-jfs-plugin.md#采集访问日志和预热缓存) 命令来提取访问日志。

JuiceFS 挂载点的性能是一个单独的话题，请额外参考以下内容：

* CSI 驱动问题排查案例：[读性能差](./troubleshooting-cases.md#bad-read-performance)
* 访问日志：[社区版](https://juicefs.com/docs/zh/community/fault_diagnosis_and_analysis#access-log)、[云服务](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#oplog)
* 实时统计数据：[社区版](https://juicefs.com/docs/zh/community/performance_evaluation_guide/#juicefs-stats)、[云服务](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#stats)
