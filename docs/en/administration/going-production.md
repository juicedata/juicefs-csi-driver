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

- Elasticsearch: index logs and provide a complete full-text search engine, which can facilitate users to retrieve the required data from the log. For installation, refer to the [official documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html).
- Fluentd: fetch container log files, filter and transform log data, and then deliver the data to the Elasticsearch cluster. For installation, refer to the [official documentation](https://docs.fluentd.org/installation).
- Kibana: visual analysis of logs, including log search, processing, and gorgeous dashboard display, etc. For installation, refer to the [official documentation](https://www.elastic.co/guide/en/kibana/current/install.html).

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

## CSI Controller high availability {#leader-election}

From 0.19.0 and above, CSI Driver supports CSI Controller HA (enabled by default), to effectively avoid single points of failure.

### Helm

HA related settings inside `values.yaml`:

```yaml {3-5}
controller:
  leaderElection:
    enabled: true # Enable Leader Election
    leaseDuration: "15s" # Interval between replicas competing for Leader, default to 15s
  replicas: 2 # At least 2 is required for HA
```

### kubectl

HA related settings inside `k8s.yaml`:

```yaml {2, 8-9, 12-13}
spec:
  replicas: 2 # At least 2 is required for HA
  template:
    spec:
      containers:
      - name: juicefs-plugin
        args:
        - --leader-election # enable Leader Election
        - --leader-election-lease-duration=15s # Interval between replicas competing for Leader, default to 15s
        ...
      - name: csi-provisioner
        args:
        - --enable-leader-election # Enable Leader Election
        - --leader-election-lease-duration=15s # Interval between replicas competing for Leader, default to 15s
        ...
```

## Enable kubelet authentication webhook {#authentication-webhook}

If authentication webhook isn't enabled, CSI Node will run into error when listing pods (this is however, a issue fixed in newer versions, continue reading for more):

```
kubelet_client.go:99] GetNodeRunningPods err: Unauthorized
reconciler.go:70] doReconcile GetNodeRunningPods: invalid character 'U' looking for beginning of value
```

When this happens, we recommend that you enable authentication webhook and restart kubelet:

```yaml {5,8} title="/var/lib/kubelet/config.yaml"
apiVersion: kubelet.config.k8s.io/v1beta1
authentication:
  webhook:
    cacheTTL: 0s
    enabled: true
  ...
authorization:
  mode: Webhook
  ...
```

From v0.21.0, even if kubelet authentication webhook wasn't enabled, CSI Node will not run into errors. Instead it'll simply bypass kubelet, and obtain information directly from APIServer (like `ListPod`). Doing this adds a minor extra overhead to APIServer, thus authentication webhook is still recommended in production environments.

Notice that CSI Driver must be configured `podInfoOnMount: true` for the above behavior to take effect. This problem doesn't exist however with Helm installations, because `podInfoOnMount` is hard-coded into template files and automatically applied between upgrades. So with kubectl installations, ensure these settings are put into `k8s.yaml`:

```yaml {6} title="k8s.yaml"
...
apiVersion: storage.k8s.io/v1
kind: CSIDriver
...
spec:
  podInfoOnMount: true
  ...
```

As is demonstrated above, we recommend using Helm to install CSI Driver, as this avoids the toil of maintaining & reviewing `k8s.yaml`.
