# Resource Optimization

Kubernetes allows much easier and efficient resource utilization, in JuiceFS CSI Driver, there's much to be done in this aspect. Methods on resource optimizations are introduced in this chapter.

## Configure resource requests and limits for Mount Pod {#mount-pod-resources}

Every application pod that uses JuiceFS PV requires a running mount pod (reused for pods using a same PV), thus configuring proper resource definition for mount pod can effectively optimize resource usage.

Read the official documentation [Resource Management for Pods and Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers) to learn about pod resource requests and limits. For a default JuiceFS mount pod, resource requests is 1 CPU and 1GiB memory, resource limits is 2 CPU and 5GiB memory.

### Static provisioning

You can set resource requests and limits in `PersistentVolume`:

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
      juicefs/mount-cpu-request: 1000m
      juicefs/mount-memory-request: 1Gi
```

Apply PVC and example pod as follows:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  storageClassName: ""
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app-resources
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: data
      resources:
        requests:
          cpu: 10m
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-pvc
```

After config change, verify that pod is running:

```sh
kubectl get pods juicefs-app-resources
```

Verify mount pod resources:

```sh
kubectl -n kube-system get po juicefs-kube-node-2-juicefs-pv -o yaml | grep -A 6 resources
```

### Dynamic provisioning

You can set resource requests and limits in `StorageClass`:

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
  juicefs/mount-cpu-request: 1000m
  juicefs/mount-memory-request: 1Gi
```

Apply PVC and example pod:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app-resources
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: juicefs-pv
  volumes:
    - name: juicefs-pv
      persistentVolumeClaim:
        claimName: juicefs-pvc
```

After config change, verify that pod is running:

```sh
kubectl get pods juicefs-app-resources
```

Verify mount pod resources:

```sh
kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -o yaml | grep -A 6 resources
```

## Clean cache when mount pod exits

Local cache can be a precious resource, especially when dealing with large scale data. JuiceFS CSI Driver does not delete cache by default when mount pod exits. If this behavior doesn't suit you, make adjustment so that local cache is cleaned when mount pod exits.

:::note
This feature requires JuiceFS CSI Driver 0.14.1 and above.
:::

### Static provisioning

Modify `volumeAttributes` in PV definition, add `juicefs/clean-cache: "true"`:

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
      juicefs/clean-cache: "true"
```

Deploy PVC and example pod as follows:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  storageClassName: ""
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: data
      resources:
        requests:
          cpu: 10m
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-pvc
```

### Dynamic provisioning

Configure `parameters` in StorageClass definition, add `juicefs/clean-cache: "true"`:

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
  juicefs/clean-cache: "true"
```

Deploy PVC and example pod:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: juicefs-pv
  volumes:
    - name: juicefs-pv
      persistentVolumeClaim:
        claimName: juicefs-pvc
```

## Delayed mount pod deletion

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

Apply PVC and sample pod:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  storageClassName: ""
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: data
      resources:
        requests:
          cpu: 10m
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: juicefs-pvc
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

Apply PVC and sample pod:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
    - args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      name: app
      volumeMounts:
        - mountPath: /data
          name: juicefs-pv
  volumes:
    - name: juicefs-pv
      persistentVolumeClaim:
        claimName: juicefs-pvc
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

JuiceFS CSI Driver consists of CSI Controller, CSI Node Service and Mount Pod. Refer to [JuiceFS CSI Driver Architecture](../introduction.md) for details.

By default, CSI Node Service (DaemonSet) will run on all nodes, users may want to run it only on nodes that really need to use JuiceFS, to further reduce resource usage.

### Add node label

Add label for nodes that actually need to use JuiceFS, for example, mark nodes that need to run model training:

```shell
# Adjust nodes and label accordingly
kubectl label node [node-1] [node-2] app=model-training
```

### Modify JuiceFS CSI Driver installation configuration

#### Install via Helm

Add `nodeSelector` in `values.yaml`:

```yaml title="values.yaml"
node:
  nodeSelector:
    app: model-training
```

Install JuiceFS CSI Driver:

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

#### Install via kubectl

Either edit `juicefs-csi-node.yaml` and run `kubectl apply -f juicefs-csi-node.yaml`, or edit directly using `kubectl -n kube-system edit daemonset juicefs-csi-node`, add the `nodeSelector` part:

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
        # adjust accordingly
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```
