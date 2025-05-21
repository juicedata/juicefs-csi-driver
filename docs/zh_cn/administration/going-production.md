---
title: 生产环境部署建议
sidebar_position: 1
---

本章介绍在生产环境中使用 CSI 驱动的一系列最佳实践，以及注意事项。

## Mount Pod 设置 {#mount-pod-settings}

* 启用[「挂载点自动恢复」](../guide/configurations.md#automatic-mount-point-recovery)；
* 为了支持[平滑升级 Mount Pod](./upgrade-juicefs-client.md#smooth-upgrade)，请提前配置好 [CSI 控制台](./troubleshooting.md#csi-dashboard)或 [JuiceFS kubectl 插件](./troubleshooting.md#kubectl-plugin)；
* 对于动态 PV 场景，建议[配置更加易读的 PV 目录名称](../guide/configurations.md#using-path-pattern)；
* 不建议使用 `--writeback`，容器场景下，如果配置不当，极易引发丢数据等事故，详见[「客户端写缓存（社区版）」](/docs/zh/community/guide/cache#client-write-cache)或[「客户端写缓存（云服务）」](/docs/zh/cloud/guide/cache#client-write-cache)；
* 如果资源吃紧，参照[「资源优化」](../guide/resource-optimization.md#mount-pod-resources)以调优；
* 考虑为 Mount Pod 设置非抢占式 PriorityClass，避免资源不足时，Mount Pod 将业务容器驱逐。详见[文档](../guide/resource-optimization.md#set-non-preempting-priorityclass-for-mount-pod)；
* 缩容节点的最佳实践。详见[文档](#scale-down-node)。

## Sidecar 模式推荐设置 {#sidecar}

### 退出顺序

目前 CSI 驱动支持 Kubernetes 原生的 [sidecar](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers) 模式，如果你的集群 Kubernetes 版本在 v1.29 及以上，CSI 在 v0.27.0 及以上，无需任何改动即可做到应用容器退出以后，sidecar 才退出。

如果你的集群不满足上述版本要求，我们建议用户通过设置 `preStop` 来满足延迟退出的需求：

```yaml
mountPodPatch:
  - terminationGracePeriodSeconds: 3600
    lifecycle:
      preStop:
        exec:
          command:
          - sh
          - -c
          - |
            sleep 30;
```

上方是最为简单的示范，sidecar（也就是 mount 容器）会等待 30 秒后才退出。如果你的应用监听了网络端口，也可以通过检测其监听端口来建立依赖关系，保证 sidecar 容器晚于业务容器退出。

```yaml
mountPodPatch:
  - terminationGracePeriodSeconds: 3600
    lifecycle:
      preStop:
        exec:
          command:
          - sh
          - -c
          - |
            set +e
            # 根据实际情况修改
            url=http://127.0.0.1:8000
            while :
            do
              res=$(curl -s -w '%{exitcode}' $url)
              # 仅当服务端口返回 Connection refused，才视为服务退出
              if [[ "$res" == 7 ]]
              then
                exit 0
              else
                echo "$url is still open, wait..."
                sleep 1
              fi
            done
```

## 监控 Mount Pod（社区版） {#monitoring}

:::tip
本节介绍的监控相关实践仅适用于 JuiceFS 社区版，企业版客户端并不通过本地端口来暴露监控数据，而是提供中心化的抓取 API，详见[企业版文档](https://juicefs.com/docs/zh/cloud/administration/monitoring/#prometheus-api)。
:::

默认设置下（未使用 `hostNetwork`），Mount Pod 通过 9567 端口提供监控 API（也可以通过在 [`mountOptions`](../guide/configurations.md#mount-options) 中添加 [`metrics`](https://juicefs.com/docs/zh/community/command_reference#mount) 选项来自定义端口号），端口名为 `metrics`，因此可以按如下方式配置 Prometheus 的监控配置。

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

## 开启 Validating Webhook {#enable-validating-webhook}

我们建议你在生产环境中开启 validating webhook，来避免一些错误配置被创建，导致 Mount Pod 无法正常工作。比如：

- 同一个 Pod 使用了多个 PV，但是其中的 volumeHandle 重复，导致无法正常挂载。
- secret 中信息不完整或者填写有误，导致无法正常挂载。

```yaml
validatingWebhook:
  enabled: true
```

## CSI Controller 的高可用设置 {#leader-election}

CSI Driver 在 0.19.0 及以上版本支持并默认启用 CSI Controller 高可用模式，能够有效避免单点故障。默认为双副本，竞选间隔（Lease duration）为 15s，这意味着当 CSI Controller 服务节点出现意外后，至多需要 15s 来恢复服务。考虑到 CSI Controller 的异常并不会直接影响已有挂载点继续正常运作，正常情况下无需调整竞选间隔时间。

### Helm

HA 已经在我们默认的 [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml) 中启用：

```yaml {3-5}
controller:
  leaderElection:
    enabled: true # 开启 Leader 选举
    leaseDuration: "15s" # Leader 的间隔，默认为 15s
  replicas: 2 # 副本数，高可用模式下至少需要 2 副本
```

如果资源不足，或者集群压力较大导致选举超时，那么可以尝试禁用高可用：

```yaml title="values-mycluster.yaml"
controller:
  leaderElection:
    enabled: false
  replicas: 1
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

  ```yaml title="values-mycluster.yaml"
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
* 如果 CSI 驱动造成的 APIServer 访问量太大，可以用 `[KUBE_QPS|KUBE_BURST]` 这两个环境变量来配置限速：

  ```yaml title="values-mycluster.yaml"
  # 默认值可以参考 https://pkg.go.dev/k8s.io/client-go/rest#Config
  controller:
    envs:
    - name: KUBE_QPS
      value: 3
    - name: KUBE_BURST
      value: 5

  node:
    envs:
    - name: KUBE_QPS
      value: 3
    - name: KUBE_BURST
      value: 5
  ```

* Dashboard 关闭 manager 功能

JuiceFS CSI Dashboard 默认会开启 manager 功能，同时使用 listAndWatch 的形式缓存集群中的资源，如果你的集群规模过大，可以考虑将其关闭（0.26.1 开始支持），关闭后只有当用户访问 dashboard 的时候，才会去集群中拉取资源。同时失去了模糊搜索，更好用的分页等功能。

  ```yaml title="values-mycluster.yaml"
  dashboard:
    enableManager: false
  ```

## 客户端写缓存（不推荐） {#client-write-cache}

就算脱离 Kubernetes，客户端写缓存（`--writeback`）也是需要谨慎使用的功能，他的作用是将客户端写入的文件数据存在本地盘，然后异步上传至对象存储。这带来不少使用体验和数据安全性的问题，在 JuiceFS 文档里都有着重介绍：

* [社区版文档](https://juicefs.com/docs/zh/community/guide/cache/#client-write-cache)
* [企业版文档](https://juicefs.com/docs/zh/cloud/guide/cache/#client-write-cache)

正常在宿主机上使用，便已经是具有风险的功能，因此我们不推荐在 CSI 驱动中开启 `--writeback`，避免因为容器生命周期短，造成数据还来不及上传，容器就销毁了，导致数据丢失。

在充分理解 `--writeback` 风险的前提下，如果你的场景必须使用该功能，那么请一定仔细阅读下列要点，保证集群正确配置，尽可能避免在 CSI 驱动中使用写缓存带来的额外风险：

* 配置好缓存持久化，确保缓存目录不会随着容器销毁而丢失。具体配置方法阅读[缓存设置](../guide/cache.md#cache-settings)；
* 选择下列方法之一（也可以都采纳），实现在应用容器退出的情况下，也保证 JuiceFS 客户端有足够的时间将数据上传完成：
  * 启用[延迟删除 Mount Pod](../guide/resource-optimization.md#delayed-mount-pod-deletion)，即便应用 Pod 退出，Mount Pod 也会等待指定时间后，才由 CSI Node 销毁。合理设置延时，保证数据及时上传完成；
  * 自 v0.24 起，CSI 驱动支持[定制](../guide/configurations.md#customize-mount-pod) Mount Pod 的方方面面，因此可以修改 `terminationGracePeriodSeconds`，再配合 [`preStop`](https://kubernetes.io/zh-cn/docs/concepts/containers/container-lifecycle-hooks/#container-hooks) 实现等待数据上传完成后，Mount Pod 才退出，示范如下：

    :::warning
    * 配置了 `preStop` 后，若写缓存一直未上传成功，Mount Pod 会一直等待 `terminationGracePeriodSeconds` 参数所设定的时间，长时间无法退出。这会影响某些操作的正常执行（如升级 Mount Pod），请充分测试并理解对应的风险；
    * 上述两种方案都不能**完全保证**所有写缓存数据都上传成功。
    :::

    ```yaml title="values-mycluster.yaml"
    globalConfig:
      mountPodPatch:
        - terminationGracePeriodSeconds: 600  # 请适当调整容器退出时的等待时间
          lifecycle:
            preStop:
              exec:
                command:
                - sh
                - -c
                - |
                  set +e

                  # 获取保存写缓存数据的目录
                  staging_dir="$(cat ${MOUNT_POINT}/.config | grep 'CacheDir' | cut -d '"' -f 4)/rawstaging/"

                  # 等待写缓存目录中的文件全部上传完毕再退出
                  if [ -d "$staging_dir" ]; then
                    while :
                    do
                      staging_files=$(find $staging_dir -type f | head -n 1)
                      if [ -z "$staging_files" ]; then
                        echo "all staging files uploaded"
                        break
                      else
                        echo "waiting for staging files: $staging_files ..."
                        sleep 3
                      fi
                    done
                  fi

                  umount -l ${MOUNT_POINT}
                  rmdir ${MOUNT_POINT}
                  exit 0
    ```

## 避免使用 `fsGroup` {#avoid-using-fsgroup}

JuiceFS 不支持挂载的时候将文件系统的文件映射为某个 Group ID，如果你在业务 Pod 中使用 fsGroup，kubelet 会去递归的去修改文件系统里面所有文件的所有权和权限，可能导致业务 Pod 启动非常缓慢。

如果确实需要使用，可以修改 `fsGroupChangePolicy` 字段，将其改为 `OnRootMismatch`, 只有根目录的属主与访问权限与卷所期望的权限不一致时，才改变其中内容的属主和访问权限。这一设置有助于缩短更改卷的属主与访问权限所需要的时间。

```yaml title="my-pod.yaml"
apiVersion: v1
kind: Pod
metadata:
  name: security-context-demo-2
spec:
  securityContext:
    runAsUser: 1000
    fsGroup: 2000
    fsGroupChangePolicy: "OnRootMismatch"
```

## 缩容节点 {#scale-down-node}

集群管理员有时会对节点进行排空（drain），以便维护节点、升级节点等。也有可能会依赖[集群自动扩缩容工具](https://kubernetes.io/zh-cn/docs/concepts/cluster-administration/cluster-autoscaling)对集群进行自动扩缩容。

在排空节点时，Kubernetes 会驱逐节点上所有的 Pod，包括 Mount Pod。如果 Mound Pod 先于应用 Pod 被驱逐，会导致应用 Pod 无法访问 JuiceFS PV，并且 CSI Node 检查到 Mount Pod 意外退出，但却还有应用 Pod 使用时，会再次拉起，这样会导致 Mount Pod 处于删除 - 拉取的循环中，造成节点缩容无法正常进行，同时业务 Pod 访问 JuiceFS PV 报错的异常。

为了避免缩容期间的异常，阅读以下小节了解如何处理。

### 设置干扰预算（PodDisruptionBudget）{#pdb}

可以为 Mount Pod 设置干扰预算（[PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb)）。干扰预算可以保证在排空节点时，Mount Pod 不会被驱逐，直到其对应的应用 Pod 被驱逐，CSI Node 会将其删除。这样既可以保证节点排空期间应用 Pod 对 JuiceFS PV 的访问，避免 Mount Pod 的删除 - 拉取循环，也不影响整个节点排空的流程。示例如下：

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: jfs-pdb
  namespace: kube-system  # 对应 JuiceFS CSI 所在的命名空间
spec:
  minAvailable: "100%"    # 避免所有 Mount Pod 在节点排空时被驱逐
  selector:
    matchLabels:
      app.kubernetes.io/name: juicefs-mount
```

:::note 兼容性
不同的服务提供商都对 Kubernetes 进行了适配和改造，使得 PDB 未必能如预期般工作，如果出现这种情况，请参考下一小节，用 Webhook 来保证排空节点时，Mount Pod 不被过早驱逐。
:::

### 使用 Validating Webhook 拒绝驱逐 {#validating-webhook}

某些 Kubernetes 环境中，PDB 并不如预期般工作（比如 [Karpenter](https://github.com/aws/karpenter-provider-aws/issues/7853)），如果使用了 PDB，可能会干扰自动扩缩容工具的正常缩容流程。

面对这种情况，则不应使用 PDB，而是为 CSI 驱动启用 Validating Webhook。这样 CSI 驱动在检查到被驱逐的 Mount Pod 还有应用 Pod 使用时，会拒绝驱逐请求。自动扩缩容工具工具会持续重试，直到 Mount Pod 引用计数归零、被正常释放。通过 Helm 安装的示例如下：

:::note
此特性需使用 0.27.1 及以上版本的 JuiceFS CSI 驱动
:::

```yaml
validatingWebhook:
  enabled: true
```

如果你在使用 [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) 工具时，如果在遇到含有 Mount Pod 的节点无法缩容的情况，可能是因为 Cluster Autoscaler 无法驱逐 [Not Replicated Pod](https://github.com/kubernetes/autoscaler/issues/351)，导致无法正常缩容。此时可以尝试为 Mount Pod 设置 `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` 注解，同时配合上述 webhook，来达到正常缩容的目的。
