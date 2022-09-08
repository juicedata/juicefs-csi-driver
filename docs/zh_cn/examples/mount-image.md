---
sidebar_label: 为 Mount Pod 指定镜像
---

# 如何让 Mount Pod 使用自定义的镜像

本文档展示了如何让 JuiceFS Mount Pod 使用自定义[镜像](https://kubernetes.io/zh-cn/docs/concepts/containers/images/)。默认的镜像为[`juicedata/mount:nightly`](https://hub.docker.com/r/juicedata/mount/tags)，为使 Mount Pod 能正常运行，请使用基于[`juicefs.Dockerfile`](https://github.com/juicedata/juicefs-csi-driver/blob/master/docker/juicefs.Dockerfile)构建的镜像。

:::note 注意
若采用进程挂载的方式启动 CSI 驱动，即 CSI Node 和 CSI Controller 的启动参数使用 `--by-process=true`，则本文档的相关配置会被忽略。
:::

## 安装 CSI 时覆盖默认镜像

CSI node 启动时，在 juicefs-plugin 容器中设置 `JUICEFS_MOUNT_IMAGE` 环境变量可覆盖默认的 Mount Pod 镜像，CSI image 在构建时把 `JUICEFS_MOUNT_IMAGE` 设为当时最新的稳定版 Mount Pod 镜像，一般为 `juicedata/mount:{latest ce version}-{latest ee version}`。

:::note 注意
一旦 juicefs-plugin 容器启动，默认的 Mount Pod 镜像就无法修改，如需修改只能在容器重新创建时再次设置 `JUICEFS_MOUNT_IMAGE` 环境变量。
:::

```yaml {12-13}
apiVersion: apps/v1
kind: DaemonSet
# metadata:
spec:
  template:
    # metadata:
    spec:
      containers:
      - name: juicefs-plugin
        image: juicedata/juicefs-csi-driver:nightly
        env:
        - name: JUICEFS_MOUNT_IMAGE
          value: juicedata/mount:patch-some-bug
```

## 在 PersistentVolume 中配置

您也可以在 `PersistentVolume` 中配置 Mount Pod 镜像：

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
      juicefs/mount-image: juicedata/mount:patch-some-bug
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

### 检查 mount pod 的 image 设置

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您可以验证 mount pod 的 image 设置得是否正确：

```sh
kubectl get -n kube-system po juicefs-{k8s-node}-juicefs-pv-{hash id} -o yaml | grep image
```

## 在 StorageClass 中配置

您可以在 `StorageClass` 中配置资源请求和约束：

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
  juicefs/mount-image: juicedata/mount:patch-some-bug
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

### 检查 mount pod 的 image 设置

应用配置后，验证 pod 是否正在运行：

```sh
kubectl get pods juicefs-app
```

您可以验证 mount pod 的 image 设置得是否正确：

```sh
kubectl get -n kube-system po juicefs-{k8s-node}-juicefs-pv-{hash id} -o yaml | grep image
```
