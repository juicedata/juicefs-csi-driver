---
title: Production Recommendations
sidebar_position: 1
---

Best practices and recommended settings when going production.

## PV settings {#pv-settings}

Below settings are recommended for a production environment.

* [Configure more readable names for PV directory](../guide/pv.md#using-path-pattern)
* Enable [Automatic Mount Point Recovery](../guide/pv.md#automatic-mount-point-recovery)
* The `--writeback` option is strongly advised against, as it can easily cause data loss especially when used inside containers, if not properly managed. See ["Write Cache in Client (Community Edition)"](https://juicefs.com/docs/community/cache_management/#writeback) and ["Write Cache in Client (Cloud Service)"](https://juicefs.com/docs/cloud/guide/cache/#client-write-cache).
* When cluster is low on resources, refer to optimization techniques in [Resource Optimization](../guide/resource-optimization.md).

## Configure mount pod monitoring {#monitoring}

By default (not using `hostNetwork`), the mount pod provides a metrics API through port 9567 (you can also add [`metrics`](https://juicefs.com/docs/community/command_reference#mount) option in [`mountOptions`](../guide/pv.md#mount-options) to customize the port number), the port name is `metrics`, so the monitoring configuration of Prometheus can be configured as follows.

### Collect data in Prometheus {#collect-metrics}

Add below scraping config into `prometheus.yml`:

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
        regex: metrics  # The metrics API port name of Mount Pod
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

Above example assumes that Prometheus runs within the cluster, if that isn't the case, apart from properly configure your network to allow Prometheus accessing the Kubernetes nodes, you'll also need to add `api_server` and `tls_config`:

```yaml
scrape_configs:
  - job_name: 'juicefs'
    kubernetes_sd_configs:
    # Refer to https://github.com/prometheus/prometheus/issues/4633
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

### Prometheus Operator {#prometheus-operator}

For [Prometheus Operator](https://prometheus-operator.dev/docs/user-guides/getting-started), add a new `PodMonitor`:

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
      # Set to CSI Driver's namespace, default to kube-system
      - <namespace>
  selector:
    matchLabels:
      app.kubernetes.io/name: juicefs-mount
  podMetricsEndpoints:
    - port: metrics  # The metrics API port name of Mount Pod
      path: '/metrics'
      scheme: 'http'
      interval: '5s'
```

And then reference this PodMonitor in the Prometheus definition:

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

### Grafana visualization {#grafana}

Once metrics data is collected, refer to the following documents to set up Grafana dashboard:

* [JuiceFS Community Edition](https://juicefs.com/docs/community/administration/monitoring/#grafana)
* [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/administration/monitor/#prometheus-api)

## Collect mount pod logs using EFK {#collect-mount-pod-logs}

Troubleshooting CSI Driver usually involves reading mount pod logs, if [checking mount pod logs in real time](./troubleshooting.md#check-mount-pod) isn't enough, consider deploying an EFK (Elasticsearch + Fluentd + Kibana) stack (or other suitable systems) in Kubernetes Cluster to collect pod logs for query. Taking EFK for example:

- Elasticsearch: index logs and provide a complete full-text search engine, which can facilitate users to retrieve the required data from the log. For installation, please refer to the [official documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html).
- Fluentd: fetch container log files, filter and transform log data, and then deliver the data to the Elasticsearch cluster. For installation, please refer to the [official documentation](https://docs.fluentd.org/installation).
- Kibana: visual analysis of logs, including log search, processing, and gorgeous dashboard display, etc. For installation, please refer to the [official documentation](https://www.elastic.co/guide/en/kibana/current/install.html).

Mount pod is labeled `app.kubernetes.io/name: juicefs-mount`. Add below config to the Fluentd configuration:

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

And add the following parser plugin to the Fluentd configuration file:

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
