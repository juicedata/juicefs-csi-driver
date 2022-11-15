# 资源优化

Kubernetes 的一大好处就是促进资源充分利用，在 JuiceFS CSI Driver 中，也有不少方面可以做资源占用优化，甚至带来一定的性能提升。在这里集中罗列介绍。

## 为 Mount Pod 配置资源请求和约束

每一个使用着 JuiceFS PV 的容器，都对应着一个 Mount Pod（会智能匹配和复用），因此为 Mount Pod 配置合理的资源声明，将是最有效的优化资源占用的手段。

关于为 Pod 和容器管理资源，配置资源请求（`request`）和约束（`limit`），请详读[Kubernetes 官方文档](https://kubernetes.io/zh-cn/docs/concepts/configuration/manage-resources-containers)，此处便不再赘述。JuiceFS Mount Pod 的资源请求默认为 1 CPU 和 1GiB 内存，资源约束默认为 2 CPU 和 5GiB 内存。

:::note 注意
若采用进程挂载的方式启动 CSI 驱动，即 CSI Node 和 CSI Controller 的启动参数使用 `--by-process=true`，需要将 CSI Node `DaemonSet` 的资源请求调大到至少 1 CPU 和 1GiB 内存，资源约束调大到至少 2 CPU 和 5GiB 内存。
:::

### 静态配置

您可以在 `PersistentVolume` 中配置资源请求和约束：

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

部署 PVC 和示例 pod：

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

配置完成后，验证 Pod 是否正在运行：

```sh
kubectl get pods juicefs-app-resources
```

您可以验证 mount pod 的 resource 设置得是否正确：

```sh
kubectl -n kube-system get po juicefs-kube-node-2-juicefs-pv -o yaml | grep -A 6 resources
```

### 动态配置

您可以在 `StorageClass` 中配置资源请求和约束：

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

部署 PVC 和示例 pod：

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

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-resources
```

验证 mount pod resource：

```sh
kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -o yaml | grep -A 6 resources
```

## 配置 Mount Pod 退出时清理缓存

在不少大规模场景下，已建立的缓存是宝贵的，因此 JuiceFS CSI Driver 默认并不会在 Mount Pod 退出时清理缓存。如果这对你的场景不适用，可以对 PV 进行配置，令 Mount Pod 退出时直接清理自己的缓存。

:::note 注意
此特性需使用 0.14.1 及以上版本的 JuiceFS CSI 驱动
:::

### 静态配置

您可以在 PV 中配置是否需要清理缓存，在 `volumeAttributes` 中设置 `juicefs/clean-cache`，值为 `"true"`，如下：

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

部署 PVC 和示例 pod：

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
  name: juicefs-app-mount-options
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

### 动态配置

您可以在 StorageClass 中配置是否需要清理缓存，在 `parameters` 中设置 `juicefs/clean-cache`，值为 `"true"`，如下：

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

部署 PVC 和示例 pod：

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
  name: juicefs-app-mount-options
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

## 如何延迟删除 Mount Pod

:::note 注意
此特性需使用 0.13.0 及以上版本的 JuiceFS CSI 驱动
:::

Mount Pod 是支持复用的，由 JuiceFS CSI Node Service 以引用计数的方式进行管理：当没有任何应用 Pod 在使用该 Mount Pod 创建出来的挂载时，JuiceFS CSI 会删除 Mount Pod。

但在 Kubernetes 不少场景中，容器转瞬即逝，调度极其频繁，这时可以为 Mount Pod 配置延迟删除，这样一来，如果短时间内还有新应用 Pod 使用相同的 Volume，Mount Pod 能够被继续复用，免除了反复销毁创建的开销。

### 静态配置

您可以在 PV 中配置延迟删除的时长，在 `volumeAttributes` 中设置 `juicefs/mount-delete-delay`，值为需要设置的时长，如下：

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

其中，单位可以为："ns"（纳秒），"us"（微秒），"ms"（毫秒），"s"（秒），"m"（分钟），"h"（小时）。

当最后一个应用 pod 删除后，mount pod 被打上 `juicefs-delete-at` 的 annotation，记录应该被删除的时刻，当到了设置的删除时间后，mount pod 才会被删除；
当有新的应用 Pod 使用相同 JuiceFS Volume 后，annotation `juicefs-delete-at` 会被删除。

部署 PVC 和示例 pod：

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
  name: juicefs-app-mount-options
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

### 动态配置

您也可以在 StorageClass 中配置延迟删除的时长，在 `parameters` 中设置 `juicefs/mount-delete-delay`，值为需要设置的时长，如下：

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

部署 PVC 和示例 pod：

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
  name: juicefs-app-mount-options
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

## PV 的回收策略

### 动态配置

如果动态配置方式，默认的回收策略为 Delete，这意味着应用卸载时，PV 也会一并删除释放。你也可以修改默认行为为 Retain，也就是由管理员手动回收资源，配置方式如下：

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

### 静态配置

静态配置中，只支持 Retain 回收策略，即需要集群管理员手动回收资源。配置方式如下：

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

## 限制 CSI Node 组件的部署范围

JuiceFS CSI Driver 的组件分为 CSI Controller、CSI Node 及 Mount Pod，详细可参考[JuiceFS CSI 架构文档](../introduction.md)。

默认情况下，CSI Node（Kubernetes DaemonSet）会在所有节点上启动，如果希望进一步减少资源占用，则可按照本节介绍的方式，让 CSI Node 仅在实际需要使用 JuiceFS 的节点上启动。

### 配置 JuiceFS CSI Node

配置按需启动很简单，仅需在 DaemonSet 中加入 `nodeSelector`，指向实际需要使用 JuiceFS 的节点，假设需要的 Node 都已经打上了该 Label：`app: model-training`。

```shell
# 根据实际情况为 nodes 打上 label
kubectl label node [node-1] [node-2] app=model-training
```

#### Kubectl

修改 `juicefs-csi-node.yaml` 然后运行 `kubectl apply -f juicefs-csi.node.yaml`，或者直接 `kubectl -n kube-system edit daemonset juicefs-csi-node`，加入 `nodeSelector` 配置：

```yaml {11-13}
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
        # 根据实际情况修改
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

#### Helm

在 `values.yaml` 中添加如下配置：

```yaml title="values.yaml"
node:
  nodeSelector:
    app: model-training
```

安装：

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```
