---
title: Resource Optimization
sidebar_position: 5
---

Kubernetes allows much easier and efficient resource utilization, in JuiceFS CSI Driver, there's much to be done in this aspect. Methods on resource optimizations are introduced in this chapter.

## Adjust resources for Mount Pod {#mount-pod-resources}

Every application Pod that uses JuiceFS PV requires a running Mount Pod (reused for Pods using a same PV), thus configuring proper resource definition for Mount Pod can effectively optimize resource usage. Read [Resource Management for Pods and Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers) to learn about Pod resource requests and limits.

Under the default settings, JuiceFS Mount Pod resource `requests` is 1 CPU and 1GiB memory, resource `limits` is 5 CPU and 5GiB memory, this might not be the perfect setup for you since JuiceFS is used in so many different scenarios, you should make adjustments to fit the actual resource usage:

* If actual usage is lower, e.g. Mount Pod uses only 0.1 CPU, 100MiB memory, then you should match the resources `requests` to the actual usage, to avoid wasting resources, or worse, Mount Pod not being able to schedule to due overly large resource `requests`, this might also cause Pod preemptions which should be absolutely avoided in a production environment. For resource `limits`, you should also configure a reasonably larger value, so that the Mount Pod can deal with temporary load increases.
* If actual usage is higher, e.g. 2 CPU, 2GiB memory, even though the default `requests` allows for its scheduling, things are risky because Mount Pod is using more resources than it declares, this is called overcommitment and constant overcommitment can cause all sorts of stability issues like CPU throttling and OOM. So under this circumstance, you should also adjust requests and limits according to the actual usage.
* If high performance is required in actual scenarios, but `limits` is set too small, it will have a great negative impact on performance.

If you already have [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server) installed, use commands like these to conveniently check actual resource usage for CSI Driver components:

```shell
# Check Mount Pod resource usage
kubectl top pod -n kube-system -l app.kubernetes.io/name=juicefs-mount

# Check resource usage for CSI Controller and CSI Node, you may adjust their resource definition following the same principle
kubectl top pod -n kube-system -l app.kubernetes.io/name=juicefs-csi-driver
```

### Define resources in ConfigMap {#resources-configmap}

Starting from v0.24, you can easily customize Mount Pods and sidecar containers in [ConfigMap](./configurations.md#customize-mount-pod), changing resource definition is as simple as:

```yaml title="values-mycluster.yaml" {3-6}
globalConfig:
  mountPodPatch:
    - resources:
        requests:
          cpu: 100m
          memory: 512Mi
```

After changes are applied, rollout the application Pods or delete the Mount Pods to take effect.

### Declare resources in PVC annotations {#mount-pod-resources-pvc}

:::tip
Starting from v0.24, CSI Driver can customize Mount Pods and sidecar containers in the [ConfigMap](./configurations.md#customize-mount-pod), legacy method introduced in this section is not recommended.
:::

Since 0.23.4, users can declare Mount Pod resources within PVC annotations, since this field can be edited through out its entire life cycle, it has become the most flexible and hence most recommended way to manage Mount Pod resources. But do note this:

* After annotations are edited, existing Mount Pods won't be re-created according to the current config, you'll need to delete existing Mount Pods to trigger the re-creation.
* [Automatic mount point recovery](./configurations.md#automatic-mount-point-recovery) must be set up in advance so that the new mount points can be propagated back to the application Pods.
* Even with automatic mount point recovery, this process WILL cause short service abruption.

```yaml {6-9}
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
  annotations:
    juicefs/mount-cpu-request: 100m
    juicefs/mount-cpu-limit: "1"  # Enclose numbers in quotes
    juicefs/mount-memory-request: 500Mi
    juicefs/mount-memory-limit: 1Gi
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
```

### Omit resources definition {#omit-resources}

If omitting certain resources fields is required, you can set them to "0":

```yaml
juicefs/mount-cpu-limit: "0"
juicefs/mount-memory-limit: "0"
# if Mount Pod uses little resource, use a very low value instead of 0
juicefs/mount-cpu-requests: "1m"
juicefs/mount-memory-requests: "4Mi"
```

Apply the above config and new Mount Pods will interprete "0" as omit, forming the following definition:

```yaml
resources:
  requests:
    cpu: 1m
    memory: 4Mi
```

There's good reason we advise against setting requests to "0", Kubernetes itself interpretes an ommited requests as equals to limits, that is to say if you write the following resources:

```yaml
# this is BAD example, do not copy and use
juicefs/mount-cpu-limit: "32"
juicefs/mount-memory-limit: "64Gi"
# zero causes csi-node to omit the requests field
juicefs/mount-cpu-requests: "0"
juicefs/mount-memory-requests: "0"
```

According to the requests = limits interpretation, the resulting definition is usually NOT what users expected, and Mount Pods cannot start.

```yaml
resources:
  limits:
    cpu: 32
    memory: 64Gi
  requests:
    cpu: 32
    memory: 64Gi
```

### Other methods (deprecated) {#deprecated-resources-definition}

:::warning
It is recommended to use the PVC annotations method introduced above. This method supports dynamic changes, so it is our more recommended method. Once the method described below is set up successfully, it cannot be modified. The only way is to delete and rebuild the PV, which is no longer recommended.
:::

For static provisioning, set resource requests and limits in `PersistentVolume`:

```yaml {22-25}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/mount-cpu-limit: 5000m
      juicefs/mount-memory-limit: 5Gi
      juicefs/mount-cpu-request: 100m
      juicefs/mount-memory-request: 500Mi
```

For dynamic provisioning, set resource requests and limits in `StorageClass`:

```yaml {11-14}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  juicefs/mount-cpu-limit: 5000m
  juicefs/mount-memory-limit: 5Gi
  juicefs/mount-cpu-request: 100m
  juicefs/mount-memory-request: 500Mi
```

In versions 0.23.4 and later, since parameter templating is supported, PVC annotations can be referenced in the `parameters` field of StorageClass:

```yaml {8-11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  ...
  juicefs/mount-cpu-limit: ${.pvc.annotations.csi.juicefs.com/mount-cpu-limit}
  juicefs/mount-memory-limit: ${.pvc.annotations.csi.juicefs.com/mount-memory-limit}
  juicefs/mount-cpu-request: ${.pvc.annotations.csi.juicefs.com/mount-cpu-request}
  juicefs/mount-memory-request: ${.pvc.annotations.csi.juicefs.com/mount-memory-request}
```

It should be noted that since [define Mount Pod resources in PVC annotations](#mount-pod-resources-pvc) is already supported, this configuration method is no longer needed.

If StorageClass is managed by Helm, you can define Mount Pod resources directly in `values.yaml`:

```yaml title="values.yaml" {5-12}
storageClasses:
- name: juicefs-sc
  enabled: true
  ...
  mountPod:
    resources:
      requests:
        cpu: "100m"
        memory: "500Mi"
      limits:
        cpu: "5"
        memory: "5Gi"
```

## Set reasonable cache size for Mount Pod {#set-reasonable-cache-size-for-mount-pod}

[Node-pressure eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction) is usually set in a Kubernetes cluster. `nodefs.available` is the available disk space of the node's root file system. The default cache size of JuiceFS is 100GiB. `free-space-ratio`, the minimum free space ratio of the default cache directory, is 0.1. The default cache size is likely to trigger node eviction. It is recommended to set a reasonable cache size according to the actual disk space of the node.

Cache size `cache-size` and the minimum free space ratio `free-space-ratio` of the cache directory can be set in mount options, see [Mount Options](./configurations.md#mount-options) for details.

## Set non-preempting PriorityClass for Mount Pod {#set-non-preempting-priorityclass-for-mount-pod}

:::tip

- It's recommended to set non-preempting PriorityClass for Mount Pod by default.
- If the mount mode of CSI Driver is ["Sidecar mode"](../introduction.md#sidecar), the following problems will not be encountered.

:::

When CSI Node creates a Mount Pod, it will set PriorityClass to `system-node-critical` by default, so that the Mount Pod will not be evicted when the node resources are insufficient.

However, when the Mount Pod is created, if the node resources are insufficient, `system-node-critical` will cause scheduler to enable preemption for the Mount Pod, which may affect the existing Pods on the node. If you do not want the existing Pods to be affected, you can set the PriorityClass of the Mount Pod to be Non-preempting, as follows:

1. Create a Non-preempting PriorityClass in cluster. For more information about PriorityClass, refer to [Official Documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption):

   ```yaml
   apiVersion: scheduling.k8s.io/v1
   kind: PriorityClass
   metadata:
     name: juicefs-mount-priority-nonpreempting
   value: 1000000000           # The higher the value, the higher the priority, and the range is -2147483648 to 1000000000 inclusive. Should be as large as possible to ensure Mount Pods are not evicted
   preemptionPolicy: Never     # Non-preempting
   globalDefault: false
   description: "This priority class used by JuiceFS Mount Pod."
   ```

2. Add the `JUICEFS_MOUNT_PRIORITY_NAME` environment variable to the CSI Node Service, with the value of the PriorityClass name created above, and add environment variable `JUICEFS_MOUNT_PREEMPTION_POLICY` with value `Never` to set the preemption policy for the Mount Pod to Never.

   ```shell
   kubectl -n kube-system set env -c juicefs-plugin daemonset/juicefs-csi-node JUICEFS_MOUNT_PRIORITY_NAME=juicefs-mount-priority-nonpreempting JUICEFS_MOUNT_PREEMPTION_POLICY=Never
   kubectl -n kube-system set env -c juicefs-plugin statefulset/juicefs-csi-controller JUICEFS_MOUNT_PRIORITY_NAME=juicefs-mount-priority-nonpreempting JUICEFS_MOUNT_PREEMPTION_POLICY=Never
   ```

## Share Mount Pod for the same StorageClass {#share-mount-pod-for-the-same-storageclass}

By default, Mount Pod is only shared when multiple application Pods are using a same PV. However, you can take a step further and share Mount Pod (in the same node, of course) for all PVs that are created using the same StorageClass, under this policy, different application Pods will bind the host mount point on different paths, so that one Mount Pod is serving multiple application Pods.

To enable Mount Pod sharing for the same StorageClass, add the `STORAGE_CLASS_SHARE_MOUNT` environment variable to the CSI Node Service:

```shell
kubectl -n kube-system set env -c juicefs-plugin daemonset/juicefs-csi-node STORAGE_CLASS_SHARE_MOUNT=true
```

Evidently, more aggressive sharing policy means lower isolation level, Mount Pod crashes will bring worse consequences, so if you do decide to use Mount Pod sharing, make sure to enable [automatic mount point recovery](./configurations.md#automatic-mount-point-recovery) as well, and [increase Mount Pod resources](#mount-pod-resources).

## Clean cache when Mount Pod exits {#clean-cache-when-mount-pod-exits}

Refer to [relevant section in Cache](./cache.md#mount-pod-clean-cache).

## Delayed Mount Pod deletion {#delayed-mount-pod-deletion}

Mount Pod will be re-used when multiple applications reference a same PV, JuiceFS CSI Node Service will manage Mount Pod life cycle using reference counting: when no application is using a JuiceFS PV anymore, JuiceFS CSI Node Service will delete the corresponding Mount Pod.

But with Kubernetes, containers are ephemeral and sometimes scheduling happens so frequently, you may prefer to keep the Mount Pod for a short while after PV deletion, so that newly created Pods can continue using the same Mount Pod without the need to re-create, further saving cluster resources.

Delayed deletion is controlled by a piece of config that looks like `juicefs/mount-delete-delay: 1m`, this supports a variety of units: `ns` (nanoseconds), `us` (microseconds), `ms` (milliseconds), `s` (seconds), `m` (minutes), `h` (hours).

With delete delay set, when reference count becomes zero, the Mount Pod is marked with the `juicefs-delete-at` annotation (a timestamp), CSI Node Service will schedule deletion only after it reaches `juicefs-delete-at`. But in the mean time, if a newly created application needs to use this exact PV, the annotation `juicefs-delete-at` will be emptied, allowing new application Pods to continue using this Mount Pod.

### Configuration

[ConfigMap](./configurations.md#configmap) is recommended for this type of customization.

```yaml title="values-mycluster.yaml"
globalConfig:
  mountPodPatch:
    # Set delete delay for selected PVC
    - pvcSelector:
        matchLabels:
          mylabel1: "value1"
      annotations:
        juicefs-delete-delay: 5m

    # Use delete delay for all PVCs
    - annotations:
        juicefs-delete-delay: 5m
```

If you're stuck with legacy CSI Drivers that doesn't support ConfigMap, use the legacy methods, where config is set differently for static and dynamic provisioning.

For static provisioning, in PV definition, add `juicefs/mount-delete-delay` to the `volumeAttributes` field:

```yaml {22}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/mount-delete-delay: 1m
```

For dynamic provisioning, in StorageClass definition, add `juicefs/mount-delete-delay` to the `parameters` field:

```yaml {11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  juicefs/mount-delete-delay: 1m
```

## PV Reclaim Policy {#reclaim-policy}

[Reclaim policy](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming) dictates what happens to data in storage after PVC or PV is deleted. Retain and Delete are the most commonly used policies, Retain means PV (alongside with its associated storage asset) is kept after PVC is deleted, while the Delete policy will remove PV and its data in JuiceFS when PVC is deleted.

### Static provisioning

Static provisioning only support the Retain reclaim policy:

```yaml {13}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

### Dynamic provisioning

For dynamic provisioning, reclaim policy is Delete by default, can be changed to Retain in StorageClass definition:

```yaml {6}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
reclaimPolicy: Retain
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
```

## Running CSI Node Service on select nodes {#csi-node-node-selector}

JuiceFS CSI Driver consists of CSI Controller, CSI Node Service and Mount Pod. Refer to [JuiceFS CSI Driver Architecture](../introduction.md#architecture) for details.

By default, CSI Node Service (DaemonSet) will run on all nodes, users may want to run it only on nodes that really need to use JuiceFS, to further reduce resource usage.

### Add node label {#add-node-label}

Add label for nodes that actually need to use JuiceFS, for example, mark nodes that need to run model training:

```shell
# Adjust nodes and label accordingly
kubectl label node [node-1] [node-2] app=model-training
```

### Modify JuiceFS CSI Driver installation configuration {#modify-juicefs-csi-driver-installation-configuration}

Apart from `nodeSelector`, Kubernetes also offer other mechanisms to control Pod scheduling, see [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node).

:::warning
If you were to use nodeSelector to limit CSI-node to specified nodes, then you need to add the same nodeSelector to your applications, so that app Pods are guaranteed to be scheduled on the nodes that can actually provide JuiceFS service.
:::

#### Install via Helm

Add `nodeSelector` in `values.yaml`:

```yaml title="values.yaml"
node:
  nodeSelector:
    # Adjust accordingly
    app: model-training
```

Install JuiceFS CSI Driver:

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

#### Install via kubectl

Add `nodeSelector` in [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml):

```yaml {11-13} title="k8s.yaml"
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: juicefs-csi-node
  namespace: kube-system
  ...
spec:
  ...
  template:
    spec:
      nodeSelector:
        # Adjust accordingly
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

Install JuiceFS CSI Driver:

```shell
kubectl apply -f k8s.yaml
```
