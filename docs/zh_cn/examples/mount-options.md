# 如何在 Kubernetes 中使用 Mount Options

本文档展示了如何将 mount options 应用到 JuiceFS。

CSI Driver 支持 `juicefs mount` 命令行选项和 _fuse_ 挂载选项（`-o` 表示 `juicefs mount` 命令）。

```
juicefs mount --max-uploads=50 --cache-dir=/var/foo --cache-size=2048 --enable-xattr -o allow_other <META-URL> <MOUNTPOINT>
```

## 静态配置

您可以在 PV 中使用 mountOptions：

```yaml
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
    volumeHandle: test-bucket
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      mountOptions:
        - enable-xattr
        - max-uploads=50
        - cache-size=2048
        - cache-dir=/var/foo
        - allow_other
```

更多配置选项参考 [JuiceFS mount command](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount) 。

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

### 检查 mount options

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-mount-options
```

您还可以验证 mount option 是否在挂载的 JuiceFS 文件系统中进行了自定义，参考 [这篇文档](../troubleshooting.md#找到-mount-pod) 找到对应的 mount pod：

```sh
kubectl exec -ti juicefs-csi-node-2zz7h -c juicefs-plugin sh
ps xf
```

## 动态配置

您也可以在 StorageClass 中使用 mountOptions：

```yaml
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
mountOptions:
  - enable-xattr
  - max-uploads=50
  - cache-size=2048
  - cache-dir=/var/foo
  - allow_other
```

更多配置选项参考 [JuiceFS mount command](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount) 。

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

### 检查 mount options

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-mount-options
```

您还可以验证 mount option 是否在挂载的 JuiceFS 文件系统中进行了自定义，参考 [这篇文档](../troubleshooting.md#找到-mount-pod) 找到对应的 mount pod：

```sh
kubectl exec -ti juicefs-csi-node-2zz7h -c juicefs-plugin sh
ps xf
```
