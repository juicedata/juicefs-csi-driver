---
title: Resource Optimization
sidebar_position: 3
---

Kubernetes allows much easier and efficient resource utilization, in JuiceFS CSI Driver, there's much to be done in this aspect. Methods on resource optimizations are introduced in this chapter.

## Adjust resources for mount pod {#mount-pod-resources}

Every application pod that uses JuiceFS PV requires a running mount pod (reused for pods using a same PV), thus configuring proper resource definition for mount pod can effectively optimize resource usage. Read [Resource Management for Pods and Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers) to learn about pod resource requests and limits.

Under the default settings, JuiceFS mount pod resource `requests` is 1 CPU and 1GiB memory, resource `limits` is 2 CPU and 5GiB memory, this might not be the perfect setup for you since JuiceFS is used in so many different scenarios, you should make adjustments to fit the actual resource usage:

* If actual usage is lower, e.g. mount pod uses only 0.1 CPU, 100MiB memory, then you should match the resources `requests` to the actual usage, to avoid wasting resources, or worse, mount pod not being able to schedule to due overly large resource `requests`, this might also cause pod preemptions which should be absolutely avoided in a production environment. For resource `limits`, you should also configure a reasonably larger value, so that the mount pod can deal with temporary load increases.
* If actual usage is higher, e.g. 2 CPU, 2GiB memory, even though the default `requests` allows for its scheduling, things are risky because mount pod is using more resources than it declares, this is called overcommitment and constant overcommitment can cause all sorts of stability issues like CPU throttling and OOM. So under this circumstance, you should also adjust requests and limits according to the actual usage.

If you already have [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server) installed, use commands like these to conveniently check actual resource usage for CSI Driver components:

```shell
# Check mount pod resource usage
kubectl top pod -n kube-system -l app.kubernetes.io/name=juicefs-mount

# Check resource usage for CSI Controller and CSI Node, you may adjust their resource definition following the same principle
kubectl top pod -n kube-system -l app.kubernetes.io/name=juicefs-csi-driver
```

### Declare resources in PVC annotations {#mount-pod-resources-pvc}

Since 0.23.4, users can declare mount pod resources within PVC annotations, since this field can be edited through out its entire life cycle, it has become the most flexible and hence most recommended way to manage mount pod resources. But do note this:

* After annotations are edited, existing mount pods won't be re-created according to the current config, you'll need to delete existing mount pods to trigger the re-creation.
* [Automatic mount point recovery](./pv.md#automatic-mount-point-recovery) must be set up in advance so that the new mount points can be propagated back to the application pods.
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

### Other methods (deprecated) {#deprecated-resources-definition}

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

If StorageClass is managed by Helm, you can define mount pod resources directly in `values.yaml`:

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

## Set non-preempting PriorityClass for Mount Pod {#set-non-preempting-priorityclass-for-mount-pod}

:::tip

- It's recommended to set non-preempting PriorityClass for Mount Pod by default.
- If the mount mode of CSI Driver is ["Sidecar mode"](../introduction.md#sidecar), the following problems will not be encountered.
:::

When CSI Node creates a Mount Pod, it will set PriorityClass to `system-node-critical` by default, so that the Mount Pod will not be evicted when the node resources are insufficient.

However, when the Mount Pod is created, if the node resources are insufficient, `system-node-critical` will cause scheduler to enable preemption for the Mount Pod, which may affect the existing pods on the node. If you do not want the existing pods to be affected, you can set the PriorityClass of the Mount Pod to be Non-preempting, as follows:

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

## Share mount pod for the same StorageClass {#share-mount-pod-for-the-same-storageclass}

By default, mount pod is only shared when multiple application pods are using a same PV. However, you can take a step further and share mount pod (in the same node, of course) for all PVs that are created using the same StorageClass, under this policy, different application pods will bind the host mount point on different paths, so that one mount pod is serving multiple application pods.

To enable mount pod sharing for the same StorageClass, add the `STORAGE_CLASS_SHARE_MOUNT` environment variable to the CSI Node Service:

```shell
kubectl -n kube-system set env -c juicefs-plugin daemonset/juicefs-csi-node STORAGE_CLASS_SHARE_MOUNT=true
```

Evidently, more aggressive sharing policy means lower isolation level, mount pod crashes will bring worse consequences, so if you do decide to use mount pod sharing, make sure to enable [automatic mount point recovery](./pv.md#automatic-mount-point-recovery) as well, and [increase mount pod resources](#mount-pod-resources).

## Clean cache when mount pod exits {#clean-cache-when-mount-pod-exits}

Refer to [relevant section in Cache](./cache.md#mount-pod-clean-cache).

## Delayed mount pod deletion {#delayed-mount-pod-deletion}

:::note
This feature requires JuiceFS CSI Driver 0.13.0 and above.
:::

Mount pod will be re-used when multiple applications reference a same PV, JuiceFS CSI Node Service will manage mount pod life cycle using reference counting: when no application is using a JuiceFS PV anymore, JuiceFS CSI Node Service will delete the corresponding mount pod.

But with Kubernetes, containers are ephemeral and sometimes scheduling happens so frequently, you may prefer to keep the mount pod for a short while after PV deletion, so that newly created pods can continue using the same mount pod without the need to re-create, further saving cluster resources.

Delayed deletion is controlled by a piece of config that looks like `juicefs/mount-delete-delay: 1m`, this supports a variety of units: `ns` (nanoseconds), `us` (microseconds), `ms` (milliseconds), `s` (seconds), `m` (minutes), `h` (hours).

With delete delay set, when reference count becomes zero, the mount pod is marked with the `juicefs-delete-at` annotation (a timestamp), CSI Node Service will schedule deletion only after it reaches `juicefs-delete-at`. But in the mean time, if a newly created application needs to use this exact PV, the annotation `juicefs-delete-at` will be emptied, allowing new application pods to continue using this mount pod.

Config is set differently for static and dynamic provisioning.

### Static provisioning

In PV definition, modify the `volumeAttributes` field, add `juicefs/mount-delete-delay`:

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

### Dynamic provisioning

In StorageClass definition, modify the `parameters` field, add `juicefs/mount-delete-delay`:

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

Apart from `nodeSelector`, Kubernetes also offer other mechanisms to control pod scheduling, see [Assigning Pods to Nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node).

:::warning
If you were to use nodeSelector to limit CSI-node to specified nodes, then you need to add the same nodeSelector to your applications, so that app pods are guaranteed to be scheduled on the nodes that can actually provide JuiceFS service.
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

## Uninstall JuiceFS CSI Controller {#uninstall-juicefs-csi-controller}

The CSI Controller component exists for a single purpose: PV provisioning when using [Dynamic Provisioning](./pv.md#dynamic-provisioning). So if you have no use for dynamic provisioning, you can safely uninstall CSI Controller, leaving only CSI Node Service:

```shell
kubectl -n kube-system delete sts juicefs-csi-controller
```

If you're using Helm:

```yaml title="values.yaml"
controller:
  enabled: false
```

Considering that CSI Controller doesn't really take up a lot of resources, this practice isn't really recommended, and only kept here as a reference.
