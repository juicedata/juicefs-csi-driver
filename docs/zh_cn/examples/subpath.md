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
  - 所用 token 只有子目录的访问权限

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
