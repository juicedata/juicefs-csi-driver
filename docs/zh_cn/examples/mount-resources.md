---
sidebar_label: 配置 Mount Pod 的资源限制
---

# 如何配置 Mount Pod 的资源限制

本文档展示了如何给 JuiceFS mount pod 设置资源 limit 和 request。

## 静态配置

您可以在 PV 中配置资源限制：

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

### 检查 mount pod 的 resources 设置

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-resources
```

您可以验证 mount pod 的 resource 设置得是否正确：

```sh
kubectl -n kube-system get po juicefs-kube-node-2-test-bucket -o yaml | grep -A 6 resources
```

## 动态配置

您可以在 StorageClass 中配置资源限制：

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

### 检查 mount pod 的 resources 设置

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app-resources
```

您可以验证 mount pod 的 resource 设置得是否正确：

```sh
kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -o yaml | grep -A 6 resources
```
