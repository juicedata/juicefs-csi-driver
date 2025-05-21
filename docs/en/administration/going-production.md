---
title: Production Recommendations
sidebar_position: 1
---

Best practices and recommended settings when going production.

## Mount Pod settings {#mount-pod-settings}

* Enable [Automatic Mount Point Recovery](../guide/configurations.md#automatic-mount-point-recovery);
* To support [smooth upgrade of Mount Pods](./upgrade-juicefs-client.md#smooth-upgrade), please configure the [CSI dashboard](./troubleshooting.md#csi-dashboard) or the [JuiceFS kubectl plugin](./troubleshooting.md#kubectl-plugin) in advance;
* For dynamic PV scenarios, it is recommended to [configure a more readable PV directory name](../guide/configurations.md#using-path-pattern);
* The `--writeback` option is strongly advised against, as it can easily cause data loss especially when used inside containers, if not properly managed. See ["Write Cache in Client (Community Edition)"](/docs/community/guide/cache#client-write-cache) and ["Write Cache in Client (Cloud Service)"](/docs/cloud/guide/cache#client-write-cache);
* When cluster resources are limited, refer to techniques in [Resource Optimization](../guide/resource-optimization.md#mount-pod-resources) for optimization;
* It's recommended to set non-preempting PriorityClass for Mount Pod, see [documentation](../guide/resource-optimization.md#set-non-preempting-priorityclass-for-mount-pod) for details.
* Best practices for reducing node capacity. see [documentation](#scale-down-node)。

## Sidecar recommendations {#sidecar}

Starting from v0.27.0, CSI Driver supports Kubernetes [native sidecar containers](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers). So if you are running Kubernetes v1.29 with CSI Driver v0.27.0 or newer versions, no special configurations are needed to ensure optimal exit order (sidecar containers terminate only after the application containers have exited).

But if your cluster does not yet meet the above version requirements, we recommend users configure the `preStop` lifecycle hook to control exit order:

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

Above snippet does only the simplest: sidecar (our mount container) exits after 30 seconds. But if your application listens on a particular network port, you can test this port to establish dependency and ensure sidecar exit order.

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
            # Change URL address accordingly
            url=http://127.0.0.1:8000
            while :
            do
              res=$(curl -s -w '%{exitcode}' $url)
              # Application is regarded as exited only on "Connection refused" output
              if [[ "$res" == 7 ]]
              then
                exit 0
              else
                echo "$url is still open, wait..."
                sleep 1
              fi
            done
```

## Configure Mount Pod monitoring (Community Edition) {#monitoring}

:::tip
Content in this section is only applicable to JuiceFS Community Edition, because Enterprise Edition doesn't provide metrics via local port, instead a centralized metrics API is provided, see [enterprise docs](https://juicefs.com/docs/zh/cloud/administration/monitoring/#prometheus-api).
:::

By default (not using `hostNetwork`), the Mount Pod provides a metrics API through port 9567 (you can also add [`metrics`](https://juicefs.com/docs/community/command_reference#mount) option in [`mountOptions`](../guide/configurations.md#mount-options) to customize the port number), the port name is `metrics`, so the monitoring configuration of Prometheus can be configured as follows.

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

## Collect Mount Pod logs using EFK {#collect-mount-pod-logs}

Troubleshooting CSI Driver usually involves reading Mount Pod logs, if [checking Mount Pod logs in real time](./troubleshooting.md#check-mount-pod) isn't enough, consider deploying an EFK (Elasticsearch + Fluentd + Kibana) stack (or other suitable systems) in Kubernetes Cluster to collect Pod logs for query. Taking EFK for example:

- Elasticsearch: index logs and provide a complete full-text search engine, which can facilitate users to retrieve the required data from the log. For installation, refer to the [official documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html).
- Fluentd: fetch container log files, filter and transform log data, and then deliver the data to the Elasticsearch cluster. For installation, refer to the [official documentation](https://docs.fluentd.org/installation).
- Kibana: visual analysis of logs, including log search, processing, and gorgeous dashboard display, etc. For installation, refer to the [official documentation](https://www.elastic.co/guide/en/kibana/current/install.html).

The Mount Pod is labeled `app.kubernetes.io/name: juicefs-mount`. Add the configuration below to the Fluentd configuration:

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

## Enable Validating Webhook

We recommend enabling the validating webhook in your production environment to prevent errors in configuration that could disrupt the normal operation of Mount Pods. For instance:

- A single Pod may be using multiple PersistentVolumes (PVs), but if the `volumeHandle` is duplicated, it will fail to mount properly.

- Incomplete or incorrect information in a `secret` can also prevent the Pod from mounting successfully.

```yaml
validatingWebhook:
  enabled: true
```

## CSI Controller high availability {#leader-election}

From 0.19.0 and above, CSI Driver supports CSI Controller HA (enabled by default), to effectively avoid single point of failure.

### Helm

HA is enabled by default in our default [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml):

```yaml {3-5}
controller:
  leaderElection:
    enabled: true # Enable Leader Election
    leaseDuration: "15s" # Interval between replicas competing for Leader, default to 15s
  replicas: 2 # At least 2 is required for HA
```

If faced with limited resource, or unstable SDN causing frequent election timeouts, try disabling election:

```yaml title="values-mycluster.yaml"
controller:
  leaderElection:
    enabled: false
  replicas: 1
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

Kubelet comes with [different authentication modes](https://kubernetes.io/zh-cn/docs/reference/access-authn-authz/kubelet-authn-authz), and default `AlwaysAllow` mode effectively disables authentication. But if kubelet uses other authentication modes, CSI Node will run into error when listing Pods (this is however, a issue fixed in newer versions, continue reading for more):

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

"Large scale" is not precisely defined in this context, if you're using a Kubernetes cluster over 100 worker nodes, or Pod number exceeds 1000, or a smaller cluster but with unusual high load for the APIServer, refer to this section for performance recommendations.

* Enable `ListPod` cache: CSI Driver needs to obtain the Pod list, when faced with a large number of Pods, APIServer and the underlying etcd can suffer performance issues. Use the `ENABLE_APISERVER_LIST_CACHE="true"` environment variable to enable this cache, which can be defined as follows inside Helm values:

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

* Also to lower the workload on the APIServer, [enabling Kubelet authentication](#kubelet-authn-authz) is recommended.
* If CSI Driver caused excessive APIServer queries, use `[KUBE_QPS|KUBE_BURST]` to perform rate limit:

  ```yaml title="values-mycluster.yaml"
  # Default values defined in https://pkg.go.dev/k8s.io/client-go/rest#Config
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

* Dashboard Disable manager function

The JuiceFS CSI Dashboard defaults to enabling the manager function and uses listAndWatch to cache resources in the cluster. If your cluster is very large, you may consider disabling it (supported from version 0.26.1). After disabling, resources will only be fetched from the cluster when the user accesses the dashboard. At the same time, fuzzy search and better pagination features will be lost.

  ```yaml title="values-mycluster.yaml"
  dashboard:
    enableManager: false
  ```

## Client write cache (not recommended) {#client-write-cache}

Even without Kubernetes, the client write cache (`--writeback`) is a feature that needs to be used with caution. Its function is to store the file data written by the client on the local disk and then asynchronously upload it to the object storage. This brings about a lot of user experience and data security issues, which are highlighted in the JuiceFS documentation:

* [Community Edition Documentation](https://juicefs.com/docs/community/guide/cache/#client-write-cache)
* [Enterprise Edition Documentation](https://juicefs.com/docs/cloud/guide/cache/#client-write-cache)

Normal use on the host is already a risky feature, so we do not recommend enable `--writeback` in the CSI Driver to avoid data loss due to the short life cycle of the container before the data is uploaded, resulting in data loss.

Under the premise of fully understanding the risks of `--writeback`, if your scenario must use this feature, then please read the following points carefully to ensure that the cluster is configured correctly and avoid as much as possible the additional risks caused by using write cache in the CSI Driver:

* Configure cache persistence to ensure that the cache directory will not be lost when the container is destroyed. For specific configuration methods, read [Cache settings](../guide/cache.md#cache-settings);
* Choose one of the following methods (you can also adopt both) to ensure that the JuiceFS client has enough time to complete the data upload when the application container exits:
  * Enable [Delayed Mount Pod deletion](../guide/resource-optimization.md#delayed-mount-pod-deletion). Even if the application Pod exits, the Mount Pod will wait for the specified time before being destroyed by the CSI Node. Set a reasonable delay to ensure that data is uploaded in a timely manner;
  * Since v0.24, the CSI Driver supports [customizing](../guide/configurations.md#customize-mount-pod) all aspects of the Mount Pod, so you can modify `terminationGracePeriodSeconds`. By using [`preStop`](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks), you can ensure that the Mount Pod waits for data uploads to finish before exiting, as demonstrated below:

    :::warning
    * After `preStop` is configured, if the write cache is not uploaded successfully, the Mount Pod will wait for the time set by the `terminationGracePeriodSeconds` parameter and cannot exit for a long time. This will affect the normal execution of certain operations (such as upgrading the Mount Pod). Please fully test and understand the corresponding risks;
    * Neither of the above two solutions can **fully guarantee** that all write cache data will be uploaded successfully.
    :::

    ```yaml title="values-mycluster.yaml"
    globalConfig:
      mountPodPatch:
        - terminationGracePeriodSeconds: 600  # Please adjust the waiting time when the container exits appropriately
          lifecycle:
            preStop:
              exec:
                command:
                - sh
                - -c
                - |
                  set +e

                  # Get the directory where write cache data is saved
                  staging_dir="$(cat ${MOUNT_POINT}/.config | grep 'CacheDir' | cut -d '"' -f 4)/rawstaging/"

                  # Wait for all files in the write cache directory to be uploaded before exiting
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

## Avoid Using `fsGroup` {#avoid-using-fsgroup}

JuiceFS does not support mapping the files in the file system to a specific Group ID when mounting. If you use `fsGroup` in your business Pod, the kubelet will recursively change the ownership and permissions of all files in the file system, which may cause your business Pod to start very slowly.

If you must use `fsGroup`, you can modify the `fsGroupChangePolicy` field and set it to `OnRootMismatch`. This will only change the ownership and permissions of the contents when the owner and permissions of the root directory do not match the expected permissions of the volume. This setting helps to reduce the time required to change the ownership and permissions of the volume.

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

## Scale Down {#scale-down-node}

The cluster manager may need to drain a node for maintenance or upgrading. It may also be necessary to rely on [Cluster Auto-Scaling Tools](https://kubernetes.io/docs/concepts/cluster-administration/node-autoscaling) for automatic scaling of the cluster.

When a node is being drained, Kubernetes will evict all Pods on the node, including Mount Pods. However, if a Mount Pod is evicted prematurely, that will cause error when the remaining application Pods try to access the JuiceFS PV. Moveover, Mount Pod will be re-created by CSI Node, since it's still being referenced by application Pods, leading to a restart loop, while all JuiceFS file system requests ends with an error.

To avoid this from happening, read below sections.

### Use PodDisruptionBudget {#pdb}

Set [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb) for the Mount Pod. PDB will ensure that the Mount Pod is protected when the node is drained, until all application Pods that reference this Mount Pod is evicted, thus application Pods can continue normal access towards the JuiceFS PV during the node drain. As an example:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
   name: jfs-pdb
   namespace: kube-system  # The namespace where JuiceFS CSI is installed
spec:
   minAvailable: "100%"  # Protect Mount Pod during a node drain
   selector:
      matchLabels:
         app.kubernetes.io/name: juicefs-mount
```

:::note Compatibility
Different service providers make their own modifications on Kubernetes, some of which breaks PDB, if this is the case, refer to the next section to use Validating Webhook to protect Mount Pod.
:::

### Use validating webhook {#validating-webhook}

In certain Kubernetes environments, PDB does not work as expected (e.g. [Karpenter](https://github.com/aws/karpenter-provider-aws/issues/7853)), in which if PDB is created, scaling down no longer works properly.

To prevent this situation, you can use our Validating Webhook instead. When CSI Driver detects that an evicted Mount Pod is still being used, it will simply reject any eviction. The autoscaling tools will enter a retry loop until the Mount Pod is successfully deleted by CSI Node. To enable this feature, refer to this Helm configuration:

:::note
This feature requires at least JuiceFS CSI Driver v0.27.1.
:::

```yaml
validatingWebhook:
  enabled: true
```

When using the [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler), if a node cannot be scaled down due to the existence of Mount Pod, it might be because that the Cluster Autoscaler cannot evict [Not Replicated Pods](https://github.com/kubernetes/autoscaler/issues/351), preventing normal scale-down operations. In this case, try the `cluster-autoscaler.kubernetes.io/safe-to-evict: "true"` annotation on the Mount Pods while utilizing the aforementioned webhook to achieve proper node scale-down.
