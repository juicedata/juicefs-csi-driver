---
title: 使用 kubectl-jfs 插件
slug: /kubectl-jfs-plugin
sidebar_position: 9
---

[`kubectl-jfs`](https://github.com/juicedata/kubectl-jfs-plugin) 是 JuiceFS 提供的 kubectl 插件，用于查看、诊断和升级 Kubernetes 中的 JuiceFS CSI 工作负载。安装后，通过 `kubectl jfs` 调用插件。

## 安装

一键安装脚本支持 Linux 和 macOS，并会根据操作系统和硬件架构安装最新版本：

```shell
# 默认安装到 /usr/local/bin
curl -sSL https://d.juicefs.com/kubectl-jfs-install | sh -
```

安装完成后，检查插件版本并查看帮助：

```shell
kubectl jfs --version
kubectl jfs --help
```

## 连接集群

插件默认使用当前 kubeconfig 和 context，并支持 `--kubeconfig`、`--context`、`--namespace` 等 kubectl 通用参数。运行插件的用户需要有权限读取或修改相应的 Kubernetes 资源。

JuiceFS CSI Driver 默认安装在 `kube-system`。如果安装在其他命名空间，使用 `-m` 或 `--mount-namespace` 指定 CSI Driver 所在的命名空间：

```shell
kubectl jfs -m juicefs-system mount
```

`--namespace`（简写为 `-n`）表示应用所在的命名空间，`--mount-namespace`（简写为 `-m`）表示 JuiceFS CSI Driver 和 Mount Pod 所在的命名空间，请勿混淆。

## 查看 JuiceFS 资源

使用以下命令查看 Mount Pod、使用 JuiceFS PV 的应用 Pod，以及 JuiceFS PV 和 PVC：

```shell
# 查看 Mount Pod
kubectl jfs mount

# 查看当前命名空间中使用 JuiceFS PV 的应用 Pod
kubectl jfs pod

# 查看所有命名空间中的应用 Pod 和 PVC
kubectl jfs pod --all-namespaces
kubectl jfs pvc --all-namespaces

# 查看 JuiceFS PV 和当前命名空间中的 PVC
kubectl jfs pv
kubectl jfs pvc
```

`pod` 也可以简写为 `po`。如果只想查看某个应用命名空间，使用 `--namespace <namespace>`。

## 诊断 Pod、PVC 和 PV

`debug` 命令汇总应用 Pod、CSI Node、PVC、PV 和 Mount Pod 的状态，并提示可能的失败原因和后续排查方向：

```shell
kubectl jfs debug pod <pod-name> --namespace <namespace>
kubectl jfs debug pvc <pvc-name> --namespace <namespace>
kubectl jfs debug pv <pv-name>
```

找到异常 Mount Pod 后，可以继续查看其事件和日志：

```shell
kubectl describe pod <mount-pod-name> --namespace kube-system
kubectl logs <mount-pod-name> --namespace kube-system --all-containers
```

如果 CSI Driver 不在 `kube-system`，请相应修改命名空间。

## 采集访问日志和预热缓存

使用 `accesslog` 持续输出指定 Mount Pod 的 JuiceFS 访问日志：

```shell
kubectl jfs accesslog <mount-pod-name>
```

访问日志的分析方法参见 JuiceFS [社区版](https://juicefs.com/docs/zh/community/fault_diagnosis_and_analysis#access-log)或[云服务](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis#oplog)文档。

使用 `warmup` 将整个挂载点的数据预热到缓存：

```shell
kubectl jfs warmup <mount-pod-name>
```

:::warning
当前插件实现不会正确处理帮助信息中展示的 `<subpath>` 参数，即使指定子路径，也会预热整个挂载点。请评估对象存储流量和缓存空间后再执行。
:::

运行 `kubectl jfs accesslog --help` 或 `kubectl jfs warmup --help` 可以查看完整参数。

## 升级单个 Mount Pod

`upgrade` 支持升级一个 Mount Pod。默认只更新 JuiceFS 客户端二进制；增加 `--recreate` 后，会重建 Mount Pod 以应用镜像或其他 Pod 配置变更：

```shell
# 二进制升级
kubectl jfs upgrade <mount-pod-name>

# 重建升级
kubectl jfs upgrade <mount-pod-name> --recreate
```

两种方式的区别、版本要求和前置条件参见[「升级 JuiceFS 客户端」](./upgrade-juicefs-client.md#smooth-upgrade)。

## 批量升级

批量升级会创建 Kubernetes Job 来升级 Mount Pod 或注入式 JuiceFS Sidecar。推荐按以下顺序操作：

1. 使用 `batch diff` 预览变更；
2. 使用 `batch upgrade` 创建升级 Job；
3. 使用 `batch list` 和 `batch describe` 监控任务；
4. 必要时暂停、恢复、停止或删除任务。

### 预览 Mount Pod 变更

```shell
# 列出所有需要升级的 Mount Pod
kubectl jfs batch diff

# 查看一个 Mount Pod 的配置差异
kubectl jfs batch diff <mount-pod-name>

# 只查看一个节点上的 Mount Pod
kubectl jfs batch diff --node <node-name>
```

### 批量升级 Mount Pod

```shell
# 升级所有需要更新的 Mount Pod
kubectl jfs batch upgrade

# 只升级一个节点上的 Mount Pod
kubectl jfs batch upgrade --node <node-name>

# 只升级与指定 PVC 关联的 Mount Pod
kubectl jfs batch upgrade --pvc <pvc-name>

# 最多同时升级 5 个目标
kubectl jfs batch upgrade --worker 5

# 单个目标失败后继续升级
kubectl jfs batch upgrade --ignore-error

# 跳过确认提示
kubectl jfs batch upgrade --quiet
kubectl jfs batch upgrade -y
```

`batch upgrade` 会先打印变更计划并请求确认，然后创建升级 Job。批量升级 Mount Pod 时会采用重建升级方式。已经被其他运行中的批量升级 Job 覆盖的目标会被自动跳过。

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--node <name>` | 所有节点 | 只升级指定节点上的目标。 |
| `--pvc <name>` | 所有 PVC | 只升级与指定 PVC 关联的 Mount Pod；PVC 名称必须在集群中唯一。 |
| `--worker <number>` | `1` | 最大并发升级数量。 |
| `--ignore-error` | `false` | 单个目标失败后继续升级。 |
| `-y`、`--quiet` | `false` | 跳过确认提示。 |

生产环境中建议先运行 `batch diff`，从较小范围和默认并发数开始升级。确认升级过程符合预期后，再逐步扩大范围或提高并发数。

### 配置批量升级 Job

批量升级 Job 会运行 `juicefs-csi-dashboard upgrade`。默认情况下，插件从 CSI Driver 命名空间中的 `juicefs-csi-dashboard` Deployment 读取 Job 使用的镜像和 ServiceAccount。

如果无法读取该 Deployment，或者需要覆盖默认值，可以设置以下环境变量：

| 环境变量 | 说明 |
| --- | --- |
| `DASHBOARD_IMAGE` | 批量升级 Job 使用的容器镜像。 |
| `JUICEFS_CSI_DASHBOARD_SA` | 批量升级 Job 使用的 ServiceAccount；未能从 Dashboard Deployment 读取时，默认使用 `juicefs-csi-dashboard-sa`。 |
| `BATCH_UPGRADE_TIMEOUT_SECONDS` | 批量升级超时时间，单位为秒，必须为整数。 |

建议同时设置镜像和 ServiceAccount，避免插件继续读取 Dashboard Deployment：

```shell
DASHBOARD_IMAGE=juicedata/csi-dashboard:<version> \
JUICEFS_CSI_DASHBOARD_SA=juicefs-csi-dashboard-sa \
  kubectl jfs batch upgrade
```

例如，将超时时间设置为一小时：

```shell
BATCH_UPGRADE_TIMEOUT_SECONDS=3600 kubectl jfs batch upgrade
```

如果只设置了 `DASHBOARD_IMAGE` 或 `JUICEFS_CSI_DASHBOARD_SA` 中的一个，插件仍会从 `juicefs-csi-dashboard` Deployment 读取另一个值。

### 批量升级注入式 Sidecar

使用该功能前，需先将 JuiceFS CSI 驱动升级到 v0.33.2 或以上版本。该功能仅支持升级 JuiceFS 客户端版本和修改挂载参数，升级过程中不会重启业务 Pod。修改挂载参数时，Sidecar 当前和目标 JuiceFS 客户端版本均须为 v5.4.3 或以上；其他 Sidecar 配置变更仍需重建业务 Pod。

在 `batch` 命令后、子命令前增加 `--sidecar`，即可操作注入到应用 Pod 中的 JuiceFS Sidecar：

```shell
kubectl jfs batch --sidecar <subcommand>
```

Sidecar 批量升级必须指定应用命名空间：

```shell
# 预览命名空间中所有待升级的 Sidecar
kubectl jfs batch --sidecar diff --namespace <namespace>

# 查看一个 Pod 中 Sidecar 的客户端版本和挂载参数差异
kubectl jfs batch --sidecar diff <pod-name> --namespace <namespace>

# 创建 Sidecar 批量升级 Job
kubectl jfs batch --sidecar upgrade --namespace <namespace>

# 限制节点和并发数
kubectl jfs batch --sidecar upgrade \
  --namespace <namespace> \
  --node <node-name> \
  --worker 5
```

Sidecar 批量升级有以下限制：

- 必须指定 `--namespace`；
- `--pvc` 不能与 `--sidecar` 一起使用；
- `--worker` 必须大于零；
- 未就绪或正在删除的 Pod 会被跳过；
- 已被运行中的批量升级 Job 覆盖的 Sidecar 会被跳过。

插件会根据 PVC、PV、Secret 和 Node 配置推导目标状态，并按检测到的差异更新 JuiceFS 客户端版本、挂载参数或两者。升级 JuiceFS 社区版客户端时，原有挂载参数会被保留。

### 监控和管理批量升级 Job

```shell
# 查看 Mount Pod 或 Sidecar 批量升级 Job
kubectl jfs batch list
kubectl jfs batch --sidecar list

# 查看 Job 详情
kubectl jfs batch describe <job-name>

# 管理 Job
kubectl jfs batch pause <job-name>
kubectl jfs batch resume <job-name>
kubectl jfs batch stop <job-name>
kubectl jfs batch delete <job-name>
```

运行中的 Job 必须先暂停或停止才能删除；暂停的 Job 可以恢复或删除。

如果 CSI Driver 不在 `kube-system`，所有批量升级命令同样需要通过 `-m` 指定其命名空间：

```shell
kubectl jfs -m juicefs-system batch diff
kubectl jfs -m juicefs-system batch upgrade
kubectl jfs -m juicefs-system batch describe <job-name>

kubectl jfs -m juicefs-system batch --sidecar upgrade \
  --namespace <application-namespace>
```

## 获取命令帮助

本文只介绍常用参数。插件版本更新后，命令和参数可能发生变化，请以当前安装版本的帮助信息为准：

```shell
kubectl jfs --help
kubectl jfs <command> --help
kubectl jfs batch <subcommand> --help
```
