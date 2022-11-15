# 资源优化

Kubernetes 的一大好处就是促进资源充分利用，在 JuiceFS CSI 驱动中，也有不少方面可以做资源占用优化，甚至带来一定的性能提升。在这里集中罗列介绍。

## 为 Mount Pod 配置资源请求和约束

每一个使用着 JuiceFS PV 的容器，都对应着一个 mount pod（会智能匹配和复用），因此为 mount pod 配置合理的资源声明，将是最有效的优化资源占用的手段。

关于为 Pod 和容器管理资源，配置资源请求（`request`）和约束（`limit`），请详读 [Kubernetes 官方文档](https://kubernetes.io/zh-cn/docs/concepts/configuration/manage-resources-containers)，此处便不再赘述。JuiceFS Mount Pod 的资源请求默认为 1 CPU 和 1GiB 内存，资源约束默认为 2 CPU 和 5GiB 内存。

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

在不少大规模场景下，已建立的缓存是宝贵的，因此 JuiceFS CSI 驱动默认并不会在 Mount Pod 退出时清理缓存。如果这对你的场景不适用，可以对 PV 进行配置，令 Mount Pod 退出时直接清理自己的缓存。

:::note 注意
此特性需使用 0.14.1 及以上版本的 JuiceFS CSI 驱动
:::

### 动态配置

在 StorageClass 中配置 `parameters`，添加 `juicefs/clean-cache: "true"`：

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

### 静态配置

在 PV 的资源定义中修改 `volumeAttributes`，添加`juicefs/clean-cache: "true"`：

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

## 延迟删除 Mount Pod

:::note 注意
此特性需使用 0.13.0 及以上版本的 JuiceFS CSI 驱动
:::

Mount Pod 是支持复用的，由 JuiceFS CSI Node Service 以引用计数的方式进行管理：当没有任何应用 Pod 在使用该 Mount Pod 创建出来的 PV 时，JuiceFS CSI Node Service 会删除 Mount Pod。

但在 Kubernetes 不少场景中，容器转瞬即逝，调度极其频繁，这时可以为 mount pod 配置延迟删除，这样一来，如果短时间内还有新应用 Pod 使用相同的 Volume，mount pod 能够被继续复用，免除了反复销毁创建的开销。

控制延迟删除 Mount Pod 的配置项形如 `juicefs/mount-delete-delay: 1m`，单位支持 "ns"（纳秒），"us"（微秒），"ms"（毫秒），"s"（秒），"m"（分钟），"h"（小时）。

配置好延迟删除后，当引用计数归零，mount pod 会被打上 `juicefs-delete-at` 的注解（annotation），标记好删除时间，到达设置的删除时间后，mount pod 才会被删除。但如果在此期间有新的应用 Pod 欲使用该 PV，注解 `juicefs-delete-at` 就被清空，mount pod 的删除计划随之取消，得以继续复用。

动态和静态配置方式中，需要在不同的地方填写该配置。

### 动态配置

需要在 StorageClass 定义中配置延迟删除的时长，修改 `parameters` 字段，添加 `juicefs/mount-delete-delay`，设置为需要的时长：

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

### 静态配置

需要在 PV 定义中配置延迟删除的时长，修改 `volumeAttributes` 字段，添加 `juicefs/mount-delete-delay`，设置为需要的时长：

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

## PV 回收策略

[回收策略](https://kubernetes.io/zh-cn/docs/concepts/storage/persistent-volumes/#reclaiming)决定了 PVC 或 PV 被删除后，存储里的数据何去何从。常用的回收策略是保留（Retain）和删除（Delete），保留回收策略需要用户自己回收资源（包括 PV、JuiceFS 上的数据），而删除回收策略则意味着 PV 及 JuiceFS 上的数据会随着 PVC 删除而直接清理掉。

### 动态配置

动态配置默认的回收策略为 Delete，可以在 StorageClass 定义中修改为 Retain：

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

静态配置中，只支持 Retain 回收策略：

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

## 仅在某些节点上运行 CSI Node Service

JuiceFS CSI 驱动的组件分为 CSI Controller、CSI Node 及 Mount Pod，详细可参考 [JuiceFS CSI 驱动架构](/csi/introduction)。

默认情况下，CSI Node（Kubernetes DaemonSet）会在所有 Kubernetes 节点上启动，如果希望进一步减少资源占用，则可按照本节介绍的方式，让 CSI Node 仅在实际需要使用 JuiceFS 的节点上启动。

### 配置节点标签

先为需要使用 JuiceFS 的节点加上相应的标签，比方说为执行模型训练的节点打上标签：

```shell
# 根据实际情况为 Kubernetes 节点加上标签
kubectl label node [node-1] [node-2] app=model-training
```

### 修改 JuiceFS CSI 驱动安装配置

#### 通过 Helm 安装

在 `values.yaml` 中添加如下配置：

```yaml title="values.yaml"
node:
  nodeSelector:
    # 根据实际情况修改节点标签
    app: model-training
```

安装 JuiceFS CSI 驱动：

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```
#### 通过 kubectl 安装

在 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml) 中新增 `nodeSelector` 配置：

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
        # 根据实际情况修改节点标签
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

安装 JuiceFS CSI 驱动：

```shell
kubectl apply -f k8s.yaml
```
