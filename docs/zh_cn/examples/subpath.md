---
sidebar_label: 挂载子目录
---

# 如何在 Kubernetes 中挂载子目录

本文档展示了如何在 Kubernetes 中使用子目录挂载。

## 使用 `subPath`

社区版和云服务版的使用方式一致。

您可以在 PV 中使用 `subPath`：

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

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-subpath
```

确认数据被正确地写入 JuiceFS 文件系统中：

```sh
kubectl exec -ti juicefs-app-subpath -- tail -f /data/out.txt
```

## 使用 `subdir`

如果您使用的是云服务版，且所用 token 只有子目录的权限，可以使用以下方式，只需要在 `mountOptions` 中指定 `subdir=xxx`：

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

## 使用 `pathPattern`

:::note 注意
此特性需使用 0.13.3 及以上版本的 JuiceFS CSI 驱动
:::

`pathPattern` 允许您在 `StorageClass` 中定义其不同 PV 的子目录格式，可以指定用于通过 PVC 元数据（例如标签、注释、名称或命名空间）创建目录路径的模板。默认关闭，需要手动开启，方式如下：

```bash
kubectl -n kube-system patch sts juicefs-csi-controller --type='json' -p='[{"op": "remove", "path": "/spec/template/spec/containers/1"}, {"op": "replace", "path": "/spec/template/spec/containers/0/args", "value":["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--provisioner=true"]}]'
```

确保 JuiceFS CSI Controller 的 pod 已重启：

```bash
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
juicefs-csi-controller-0                2/2     Running   0                24m
```

您可以在 `StorageClass` 中这样使用 `pathPattern`：

```yaml {12}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
  namespace: default
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
