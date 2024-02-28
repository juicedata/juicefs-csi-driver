---
title: Production Recommendations
sidebar_position: 1
---

Best practices and recommended settings when going production.

## PV settings {#pv-settings}

Below settings are recommended for a production environment.

* [Configure more readable names for PV directory](../guide/pv.md#using-path-pattern)
* Enable [Automatic Mount Point Recovery](../guide/pv.md#automatic-mount-point-recovery)
* The `--writeback` option is strongly advised against, as it can easily cause data loss especially when used inside containers, if not properly managed. See ["Write Cache in Client (Community Edition)"](/docs/community/cache_management/#writeback) and ["Write Cache in Client (Cloud Service)"](/docs/cloud/guide/cache/#client-write-cache).
* When cluster is low on resources, refer to optimization techniques in [Resource Optimization](../guide/resource-optimization.md).

## Mount Pod settings {#mount-pod-settings}

* It's recommended to set non-preempting PriorityClass for Mount Pod, see [documentation](../guide/resource-optimization.md#set-non-preempting-priorityclass-for-mount-pod) for details.

## Configure mount pod monitoring (Community Edition) {#monitoring}

:::tip
Content in this section is only applicable to JuiceFS Community Edition, because Enterprise Edition doesn't provide metrics via local port, instead a centralized metrics API is provided, see [enterprise docs](https://juicefs.com/docs/zh/cloud/administration/monitoring/#prometheus-api).
:::

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

## Enable kubelet authentication {#kubelet-authn-authz}

Kubelet comes with [different authentication modes](https://kubernetes.io/zh-cn/docs/reference/access-authn-authz/kubelet-authn-authz), and default `AlwaysAllow` mode effectively disables authentication. But if kubelet uses other authentication modes, CSI Node will run into error when listing pods (this is however, a issue fixed in newer versions, continue reading for more):

```
kubelet_client.go:99] GetNodeRunningPods err: Unauthorized
reconciler.go:70] doReconcile GetNodeRunningPods: invalid character 'U' looking for beginning of value
```

This can be resolved using one of below methods:

### Upgrade CSI Driver

Upgrade CSI Driver to v0.21.0 or newer versions, so that when faced with authentication issues, CSI Node will simply bypass kubelet and connect APIServer to watch for changes. However, this watch process initiates with a `ListPod` request (with `labelSelector` to minimize performance impact), this adds a minor extra overhead to APIServer, if your APIServer is already heavily loaded, consider enabling authentication webhook (see in the next section).

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

### Delegate kubelet authentication to APIServer

Below content is summarized from [Kubernetes documentation](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/#kubelet-authorization).

Kubelet configuration can be specified directly in command arguments, or alternatively put in configuration files (default to `/var/lib/kubelet/config.yaml`), find out which one using commands like below:

```shell {6}
$ systemctl cat kubelet
# /lib/systemd/system/kubelet.service
...
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
...
```

Notice the highlighted lines above indicates that this kubelet puts configurations in `/var/lib/kubelet/config.yaml`, so you'll need to modify this file to enable webhook authentication (using the highlighted lines below):

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

If however, a configuration file isn't used, then kubelet is configured purely via startup command arguments, append `--authorization-mode=Webhook` and `--authentication-token-webhook` to achieve the same thing.

## Large scale clusters {#large-scale}

"Large scale" is not precisely defined in this context, if you're using a Kubernetes cluster over 100 worker nodes, or pod number exceeds 1000, or a smaller cluster but with unusual high load for the APIServer, refer to this section for performance recommendations.

* Enable `ListPod` cache: CSI Driver needs to obtain the pod list, when faced with a large number of pods, APIServer and the underlying etcd can suffer performance issues. Use the `ENABLE_APISERVER_LIST_CACHE="true"` environment variable to enable this cache, which can be defined as follows inside Helm values:

  ```yaml title="values.yaml"
  controller:
    envs:
    - name: ENABLE_APISERVER_LIST_CACHE
      value: "true"

  node:
    envs:
    - name: ENABLE_APISERVER_LIST_CACHE
      value: "true"
  ```

* Also to lower the workload on the APIServer, [enabling Kubelet authentication](#kubelet-authn-authz) is recommended.
