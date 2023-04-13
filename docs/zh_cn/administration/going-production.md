---
title: 生产环境部署建议
sidebar_position: 1
---

本章介绍在生产环境中使用 CSI 驱动的一系列最佳实践，以及注意事项。

## PV 设置 {#pv-settings}

在生产环境中，推荐这样设置 PV：

* [配置更加易读的 PV 目录名称](../guide/pv.md#using-path-pattern)
* 启用[「挂载点自动恢复」](../guide/pv.md#automatic-mount-point-recovery)
* 不建议使用 `--writeback`，容器场景下，如果配置不当，极易引发丢数据等事故，详见[「客户端写缓存（社区版）」](https://juicefs.com/docs/zh/community/cache_management#writeback)或[「客户端写缓存（云服务）」](https://juicefs.com/docs/zh/cloud/guide/cache/#client-write-cache)
* 如果资源吃紧，参照[「资源优化」](../guide/resource-optimization.md)以调优

## 监控 Mount Pod {#monitoring}

默认设置下（未使用 `hostNetwork`），Mount Pod 通过 9567 端口提供监控 API（也可以通过在 [`mountOptions`](../guide/pv.md#mount-options) 中添加 [`metrics`](https://juicefs.com/docs/zh/community/command_reference#mount) 选项来自定义端口号），端口名为 `metrics`，因此可以按如下方式配置 Prometheus 的监控配置。

### Prometheus 收集监控指标 {#collect-metrics}

在 `prometheus.yml` 添加相应的抓取配置，来收集监控指标：

```yaml
scrape_configs:
  - job_name: 'juicefs'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_phase]
        separator: ;
        regex: (Failed|Succeeded)
        replacement: $1
        action: drop
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name, __meta_kubernetes_pod_labelpresent_app_kubernetes_io_name]
        separator: ;
        regex: (juicefs-mount);true
        replacement: $1
        action: keep
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        separator: ;
        regex: metrics  # Mount Pod 监控 API 端口名
        replacement: $1
        action: keep
      - separator: ;
        regex: (.*)
        target_label: endpoint
        replacement: metrics
        action: replace
      - source_labels: [__address__]
        separator: ;
        regex: (.*)
        modulus: 1
        target_label: __tmp_hash
        replacement: $1
        action: hashmod
      - source_labels: [__tmp_hash]
        separator: ;
        regex: "0"
        replacement: $1
        action: keep
```

上方的示范假定 Prometheus 服务运行在 Kubernetes 集群中，如果运行在集群外，除了确保正确设置安全组，允许 Prometheus 访问 Kubernetes 节点，还需要额外添加 `api_server` 和 `tls_config`：

```yaml
scrape_configs:
  - job_name: 'juicefs'
    kubernetes_sd_configs:
    # 详见 https://github.com/prometheus/prometheus/issues/4633
    - api_server: <Kubernetes API Server>
      role: pod
      tls_config:
        ca_file: <...>
        cert_file: <...>
        key_file: <...>
        insecure_skip_verify: false
    relabel_configs:
    ...
```

### Prometheus Operator 收集监控指标 {#prometheus-operator}

对于 [Prometheus Operator](https://prometheus-operator.dev/docs/user-guides/getting-started)，可以新增一个 `PodMonitor` 来收集监控指标：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: juicefs-mounts-monitor
  labels:
    name: juicefs-mounts-monitor
spec:
  namespaceSelector:
    matchNames:
      # 设置成 CSI 驱动所在的 namespace，默认为 kube-system
      - <namespace>
  selector:
    matchLabels:
      app.kubernetes.io/name: juicefs-mount
  podMetricsEndpoints:
    - port: metrics  # Mount Pod 监控 API 端口名
      path: '/metrics'
      scheme: 'http'
      interval: '5s'
```

同时在 `Prometheus` 资源中设置上述的 `PodMonitor`：

```yaml {7-9}
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
spec:
  serviceAccountName: prometheus
  podMonitorSelector:
    matchLabels:
      name: juicefs-mounts-monitor
  resources:
    requests:
      memory: 400Mi
  enableAdminAPI: false
```

### 在 Grafana 中进行数据可视化 {#grafana}

按照上方步骤搭建好容器指标收集后，参考下方文档配置 Grafana 仪表盘：

* [JuiceFS 社区版](https://juicefs.com/docs/zh/community/administration/monitoring#grafana)
* [JuiceFS 云服务](https://juicefs.com/docs/zh/cloud/administration/monitor/#prometheus-api)

## 在 EFK 中收集 Mount Pod 日志 {#collect-mount-pod-logs}

CSI 驱动的问题排查，往往涉及到查看 Mount Pod 日志。如果[实时查看 Mount Pod 日志](./troubleshooting.md#check-mount-pod)无法满足你的需要，考虑搭建 EFK（Elasticsearch + Fluentd + Kibana），或者其他合适的容器日志收集系统，用来留存和检索 Pod 日志。以 EFK 为例：

- Elasticsearch：负责对日志进行索引，并提供了一个完整的全文搜索引擎，可以方便用户从日志中检索需要的数据。安装方法参考[官方文档](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html)。
- Fluentd：负责获取容器日志文件、过滤和转换日志数据，然后将数据传递到 Elasticsearch 集群。安装方法参考[官方文档](https://docs.fluentd.org/installation)。
- Kibana：负责对日志进行可视化分析，包括日志搜索、处理以及绚丽的仪表板展示等。安装方法参考[官方文档](https://www.elastic.co/guide/en/kibana/current/install.html)。

Mount Pod 均包含固定的 `app.kubernetes.io/name: juicefs-mount` 标签。在 Fluentd 的配置文件中可以配置收集对应标签的日志：

```html
<filter kubernetes.**>
  @id filter_log
  @type grep
  <regexp>
    key $.kubernetes.labels.app_kubernetes_io/name
    pattern ^juicefs-mount$
  </regexp>
</filter>
```

然后在 Fluentd 的配置文件中加上如下解析插件：

```html
<filter kubernetes.**>
  @id filter_parser
  @type parser
  key_name log
  reserve_data true
  remove_key_name_field true
  <parse>
    @type multi_format
    <pattern>
      format json
    </pattern>
    <pattern>
      format none
    </pattern>
  </parse>
</filter>
```

## CSI Controller 的高可用设置 {#leader-election}

CSI Driver 在 0.19.0 及以上版本支持并默认启用 CSI Controller 高可用模式，能够有效避免单点故障。默认为双副本，竞选间隔（Lease duration）为 15s，这意味着当 CSI Controller 服务节点出现意外后，至多需要 15s 来恢复服务。考虑到 CSI Controller 的异常并不会直接影响已有挂载点继续正常运作，正常情况下无需调整竞选间隔时间。

### Helm

在 `values.yaml` 中，高可用相关设置如下：

```yaml {3-5}
controller:
  leaderElection:
    enabled: true # 开启 Leader 选举
    leaseDuration: "15s" # Leader 的间隔，默认为 15s
  replicas: 2 # 副本数，高可用模式下至少需要 2 副本
```

### kubectl

用 kubectl 直接安装 CSI 驱动时，高可用相关的选项如下：

```yaml {2, 8-9, 12-13}
spec:
  replicas: 2 # 副本数，高可用模式下至少需要 2 副本
  template:
    spec:
      containers:
      - name: juicefs-plugin
        args:
        - --leader-election # 开启 Leader 选举
        - --leader-election-lease-duration=15s # Leader 的间隔，默认为 15s
        ...
      - name: csi-provisioner
        args:
        - --enable-leader-election # 开启 Leader 选举
        - --leader-election-lease-duration=15s # Leader 的间隔，默认为 15s
        ...
```
