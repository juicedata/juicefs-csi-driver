---
title: 监控 JuiceFS CSI Driver
slug: /monitoring
sidebar_position: 8
---

JuiceFS CSI Driver 通过 [Prometheus](https://prometheus.io) 暴露内部状态和一些指标，以便进行监控和告警。

## 如何暴露和抓取 Metrics

JuiceFS CSI Driver 的 Controller 和 Node 服务都暴露了一个 `/metrics` HTTP 端点，默认端口为 `9567`, 可以通过 helm values 更改

```yaml
node:
  metricsPort: "9567"
controller:
  metricsPort: "9567"
```

### 配置 Prometheus 抓取

启用 metrics 端点后，你需要配置 Prometheus 来抓取这些指标。这通常通过创建一个 `ServiceMonitor` 或 `PodMonitor` CRD (如果你的集群中安装了 Prometheus Operator) 来实现，或者直接在 Prometheus 配置文件中添加抓取任务。

以下是一个使用 `PodMonitor` 的例子：

```yaml
# csi-podmonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: juicefs-csi
  namespace: kube-system
  labels:
    app: juicefs-csi
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: juicefs-csi-driver
  podMetricsEndpoints:
  - port: metrics
    interval: 15s
```

或者使用 `ServiceMonitor`：

```yaml
# csi-servicemonitor.yaml
apiVersion: v1
kind: Service
metadata:
  name: juicefs-csi
  namespace: kube-system
  labels:
    app: juicefs-csi
spec:
  selector:
    app.kubernetes.io/name: juicefs-csi-driver
  ports:
    - name: metrics
      port: 9567
      targetPort: 9567
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: juicefs-csi
  namespace: kube-system
  labels:
    app: juicefs-csi
spec:
  selector:
    matchLabels:
      app: juicefs-csi
  endpoints:
    - port: metrics
      interval: 15s
```

将以上 YAML 文件应用到你的集群后，Prometheus 就会自动开始抓取 JuiceFS CSI Driver 的指标。

## 指标含义

JuiceFS CSI Driver 暴露的指标主要用于追踪 CSI 操作的错误计数。

### Controller Metrics

这些指标由 `juicefs-csi-controller` Pod 暴露。

| 指标名称                   | 类型    | 描述                              |
| :------------------------- | :------ | :-------------------------------- |
| `juicefs_provision_errors` | Counter | 卷创建 (Provision) 失败的总次数。 |

- **`juicefs_provision_errors`**: 这是一个计数器，记录了 CSI `Provision` 操作失败的次数。如果这个值持续增长，说明存储卷的动态创建过程存在问题。可能的原因包括：
  - JuiceFS 文件系统授权失败。
  - 访问对象存储或元数据引擎出现网络问题。

### Node Metrics

这些指标由 `juicefs-csi-node` DaemonSet 的 Pod 暴露。

| 指标名称                    | 类型    | 描述                                   |
| :-------------------------- | :------ | :------------------------------------- |
| `juicefs_volume_errors`     | Counter | 卷挂载 (Volume Mount) 失败的总次数。   |
| `juicefs_volume_del_errors` | Counter | 卷卸载 (Volume Unmount) 失败的总次数。 |

- **`juicefs_volume_errors`**: 这是一个计数器，记录了将 JuiceFS 卷挂载到节点上时发生错误的次数。这对应于 CSI 的 `NodePublishVolume` 操作。如果这个值持续增长，可能表示：
  - 节点上的 JuiceFS 客户端无法正常启动。
  - 挂载点目录创建失败或权限不正确。
  - 从 Secret 中获取的 JuiceFS 认证信息有误。
  - bind 失败。

- **`volume_del_errors`**: 这是一个计数器，记录了从节点上卸载 JuiceFS 卷时发生错误的次数。这对应于 CSI 的 `NodeUnpublishVolume` 操作。如果这个值持续增长，可能表示：
  - 卸载操作被阻塞（例如，卷仍在使用中）。
  - 挂载点信息丢失或不一致。

除了以上自定义指标，Prometheus 还会抓取标准的 Go 进程指标 (如 `go_goroutines`, `go_memstats_*` 等) 和进程指标 (如 `process_cpu_seconds_total`, `process_resident_memory_bytes` 等)。
