---
sidebar_label: 挂载子目录
---

# 如何在 Kubernetes 中挂载子目录

本文档展示了如何在 Kubernetes 中使用子目录挂载。

## 使用 `subdir`

`subdir` 是指直接用 JuiceFS 提供的子路径挂载特性（`juicefs mount --subdir`）来挂载子目录。注意以下使用场景必须使用 `subdir` 来挂载子目录，而不能使用 `subPath`：

- **JuiceFS 社区版及云服务版**
  - 需要在应用 pod 中进行[缓存预热](https://juicefs.com/docs/zh/community/cache_management#%E7%BC%93%E5%AD%98%E9%A2%84%E7%83%AD)
- **JuiceFS 云服务版**
  - 所用 [token](https://juicefs.com/docs/zh/cloud/metadata) 只有子目录的访问权限

您只需要在 PV 的 `mountOptions` 中指定 `subdir=xxx` 即可：

:::tip 提示
如果指定的子目录不存在会自动创建
:::

```yaml {21-22}
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
  mountOptions:
    - subdir=/test
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
  name: juicefs-app-subpath
  namespace: default
spec:
  containers:
    - name: app
      args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
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

## 使用 `subPath`

`subPath` 的原理是由 JuiceFS CSI 驱动将指定的子路径 [bind mount](https://docs.docker.com/storage/bind-mounts) 到应用 pod 中。

您可以在 PV 中这样使用 `subPath`：

:::tip 提示
如果指定的子目录不存在会自动创建
:::

```yaml {21-22}
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
      subPath: fluentd
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
  name: juicefs-app-subpath
  namespace: default
spec:
  containers:
    - name: app
      args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
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

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-subpath
```

确认数据被正确地写入 JuiceFS 文件系统中：

```sh
kubectl exec -ti juicefs-app-subpath -- tail -f /data/out.txt
```

## 使用 `pathPattern` {#using-path-pattern}

:::note 注意
此特性需使用 0.13.3 及以上版本的 JuiceFS CSI 驱动
:::

通过 `pathPattern` 您可以在 `StorageClass` 中定义其不同 PV 的子目录格式，可以指定用于通过 PVC 元数据（例如标签、注释、名称或命名空间）创建目录路径的模板。此特性默认关闭，需要手动开启。

### Helm

若您使用 Helm 来安装 JuiceFS CSI 驱动，可以在 `values.yaml` 中添加如下配置以开启该特性：

```yaml title="values.yaml"
controller:
  provisioner: true
```

再重新部署 JuiceFS CSI 驱动：

```bash
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

确保 JuiceFS CSI Controller 的 pod 已重启：

```bash
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
juicefs-csi-controller-0                2/2     Running   0                24m
```

### Kubectl

若您使用 `kubectl` 来安装 JuiceFS CSI 驱动，需要通过 `kubectl patch` 命令修改默认的启动选项以开启该特性：

```bash
kubectl -n kube-system patch sts juicefs-csi-controller \
  --type='json' \
  -p='[{"op": "remove", "path": "/spec/template/spec/containers/1"}, {"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--provisioner=true"]}]'
```

确保 JuiceFS CSI Controller 的 pod 已重启：

```bash
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
juicefs-csi-controller-0                2/2     Running   0                24m
```

### 使用方式

您可以在 `StorageClass` 中这样使用 `pathPattern`：

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
  pathPattern: "${.PVC.namespace}-${.PVC.name}"
```

使用方式为 `${.PVC.<metadata>}`。示例：

1. 若文件夹命名为 `<pvc-namespace>-<pvc-name>`，则 `pathPattern` 为 `${.PVC.namespace}-${.PVC.name}`；
2. 若文件夹命名为 PVC 中标签名为 `a` 的值，则 `pathPattern` 为`${.PVC.labels.a}`；
3. 若文件夹命名为 PVC 中注释名为 `a` 的值，则 `pathPattern` 为`${.PVC.annotations.a}`。
