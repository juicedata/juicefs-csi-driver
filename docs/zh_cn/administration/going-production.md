---
title: 生产环境部署建议
sidebar_position: 1
---

本章介绍在生产环境中使用 CSI 驱动的一系列最佳实践，以及注意事项。

## PV 设置

在生产环境中，推荐这样设置 PV：

* [配置更加易读的 PV 目录名称](../guide/pv.md#using-path-pattern)
* 启用[「挂载点自动恢复」](../guide/pv.md#automatic-mount-point-recovery)
* 不建议使用 `--writeback`，容器场景下，如果配置不当，极易引发丢数据等事故，详见[「客户端写缓存（社区版）」](https://juicefs.com/docs/zh/community/cache_management#writeback)或[「客户端写缓存（云服务）」](https://juicefs.com/docs/zh/cloud/guide/cache/#client-write-cache)
* 如果资源吃紧，参照[「资源优化」](../guide/resource-optimization.md)以调优

## 配置 Mount Pod 的监控信息

JuiceFS CSI 驱动默认会在 Mount Pod 的 9567 端口提供监控指标，也可以通过在 mountOptions 中添加 metrics 选项自定义（请参考[文档](../guide/pv.md#mount-options)）。
同时 JuiceFS CSI 驱动会将 Metrics 接口设置为 containerPort，因此 Prometheus 的监控配置信息可以按如下方式配置。

### Prometheus 收集监控指标

若您的 Prometheus 服务是单独部署的，可以新增一个抓取任务到 prometheus.yml 来收集监控指标：

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
        regex: metrics
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

这里假设 Prometheus 服务运行在 Kubernetes 集群中，如果你的 Prometheus 服务运行在 Kubernetes 集群之外，请确保 Prometheus 服务可以访问 Kubernetes 节点，请参考[这个 issue](https://github.com/prometheus/prometheus/issues/4633) 添加 api_server 和 tls_config 配置到以上文件：

```yaml
scrape_configs:
  - job_name: 'juicefs'
    kubernetes_sd_configs:
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

### Prometheus Operator 收集监控指标

若您通过 [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) 来管理监控服务，可以新增一个 PodMonitor 来收集监控指标：

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
      - <namespace> # 设置成您的 CSI 驱动所在的 namespace，默认为 kube-system
  selector:
    matchLabels:
      app.kubernetes.io/name: juicefs-mount
  podMetricsEndpoints:
    - port: metrics
      path: '/metrics'
      scheme: 'http'
      interval: '5s'
```

同时在 `Prometheus` 资源中设置上述的 `PodMonitor`：

```yaml
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

更多关于如果使用 Prometheus Operator 的信息请参考[官方文档](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/user-guides/getting-started.md)。

以上是关于如何在您已有的监控服务中配置 Mount Pod 的监控信息，Grafana 仪表盘的配置请参考[文档](https://juicefs.com/docs/zh/community/administration/monitoring#%E5%8F%AF%E8%A7%86%E5%8C%96%E7%9B%91%E6%8E%A7%E6%8C%87%E6%A0%87)。

## 在 EFK 中收集 Mount Pod 日志

CSI 驱动的问题排查，往往涉及到查看 Mount Pod 日志。如果[实时查看 Mount Pod 日志](./troubleshooting.md#check-mount-pod)无法满足你的需要，考虑搭建 EFK（Elasticsearch + Fluentd + Kibana），或者其他合适的容器日志收集系统，用来留存和检索 Pod 日志。以 EFK 为例：

- Elasticsearch：负责对日志进行索引，并提供了一个完整的全文搜索引擎，可以方便用户从日志中检索需要的数据。安装方法请参考[官方文档](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html)。
- Fluentd：负责获取容器日志文件、过滤和转换日志数据，然后将数据传递到 Elasticsearch 集群。安装方法请参考[官方文档](https://docs.fluentd.org/installation)。
- Kibana：负责对日志进行可视化分析，包括日志搜索、处理以及绚丽的仪表板展示等。安装方法请参考[官方文档](https://www.elastic.co/guide/en/kibana/current/install.html)。

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
