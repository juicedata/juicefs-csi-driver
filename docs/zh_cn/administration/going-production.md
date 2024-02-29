---
title: 生产环境部署建议
sidebar_position: 1
---

本章介绍在生产环境中使用 CSI 驱动的一系列最佳实践，以及注意事项。

## PV 设置 {#pv-settings}

在生产环境中，推荐这样设置 PV：

* [配置更加易读的 PV 目录名称](../guide/pv.md#using-path-pattern)
* 启用[「挂载点自动恢复」](../guide/pv.md#automatic-mount-point-recovery)
* 不建议使用 `--writeback`，容器场景下，如果配置不当，极易引发丢数据等事故，详见[「客户端写缓存（社区版）」](/docs/zh/community/cache_management#writeback)或[「客户端写缓存（云服务）」](/docs/zh/cloud/guide/cache/#client-write-cache)
* 如果资源吃紧，参照[「资源优化」](../guide/resource-optimization.md)以调优

## Mount Pod 设置 {#mount-pod-settings}

* 建议为 Mount Pod 设置非抢占式 PriorityClass，详见[文档](../guide/resource-optimization.md#set-non-preempting-priorityclass-for-mount-pod)。

## 监控 Mount Pod（社区版） {#monitoring}

:::tip
本节介绍的监控相关实践仅适用于 JuiceFS 社区版，企业版客户端并不通过本地端口来暴露监控数据，而是提供中心化的抓取 API，详见[企业版文档](https://juicefs.com/docs/zh/cloud/administration/monitoring/#prometheus-api)。
:::

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

## 启用 Kubelet 认证鉴权 {#kubelet-authn-authz}

[Kubelet 的认证鉴权](https://kubernetes.io/zh-cn/docs/reference/access-authn-authz/kubelet-authn-authz)分为很多种，默认的 `AlwaysAllow` 模式允许所有请求。但若 kubelet 关闭了匿名访问，会导致 CSI Node 获取 Pod 列表时报错（该报错本身已经修复，见后续描述）：

```
kubelet_client.go:99] GetNodeRunningPods err: Unauthorized
reconciler.go:70] doReconcile GetNodeRunningPods: invalid character 'U' looking for beginning of value
```

面对这种情况，选择以下一种解决方法：

### 升级 CSI 驱动

升级 CSI 驱动至 v0.21.0 或更新版本，升级完毕后，当 CSI Node 遭遇一样的鉴权错误时，就不再直连 Kubelet，而是 watch APIServer 去获取信息，由于 watch list 机制在启动时会对 APIServer 进行一次 `ListPod` 请求（携带了 `labelSelector`，最大程度减少开销），在集群负载较大的情况下，会对 APIServer 造成额外的压力。因此如果你的 Kubernetes APIServer 负载已经很高，我们推荐配置 CSI Node 对 Kubelet 的认证（详见下一小节）。

需要注意，CSI 驱动需要配置 `podInfoOnMount: true`，上边提到的避免报错的特性才会真正生效。如果你采用 [Helm 安装方式](../getting_started.md#helm)，则 `podInfoOnMount` 默认开启无需配置，该特性会随着升级自动启用。而如果你使用 kubectl 直接安装，你需要为 `k8s.yaml` 添加如下配置：

```yaml {6} title="k8s.yaml"
...
apiVersion: storage.k8s.io/v1
kind: CSIDriver
...
spec:
  podInfoOnMount: true
  ...
```

这也是为什么在生产环境，我们推荐用 Helm 安装 CSI 驱动，避免手动维护的 `k8s.yaml`，在升级时带来额外的心智负担。

### 将 Kubelet 鉴权委派给 APIServer

下文中的配置方法均总结自[官方文档](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-authn-authz/#kubelet-authorization)。

Kubelet 的配置，既可以直接放在命令行参数中，也可以书写在配置文件里（默认 `/var/lib/kubelet/config.yaml`），你可以使用类似下方的命令来确认 Kubelet 如何管理配置：

```shell {6}
$ systemctl cat kubelet
# /lib/systemd/system/kubelet.service
...
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
...
```

注意上方示范中高亮行，这表明 Kubelet 将配置存放在 `/var/lib/kubelet/config.yaml`，这种情况下需要编辑配置该配置文件，启用 Webhook 认证（注意高亮行）：

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

但若 Kubelet 并未使用配置文件，而是将所有配置都直接追加在启动参数中，那么你需要追加 `--authorization-mode=Webhook` 和 `--authentication-token-webhook`，来实现相同的效果。

## 大规模集群 {#large-scale}

本节语境中不对「大规模」作明确定义，如果你的集群节点数超过 100，或者 Pod 总数超 1000，或者前两个条件均未达到，但是 Kubernetes APIServer 的负载过高，都可以考虑本节中的推荐事项，排除潜在的性能问题。

* 开启 `ListPod` 缓存：CSI 驱动需要获取 Pod 列表，如果 Pod 数量庞大，对 APIServer 和背后的 etcd 有性能冲击。此时可以通过 `ENABLE_APISERVER_LIST_CACHE="true"` 这个环境变量来启用缓存特性。你可以在 `values.yaml` 中通过环境变量声明：

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

* 同样是为了减轻 APIServer 访问压力，建议[启用 Kubelet 认证鉴权](#kubelet-authn-authz)。
