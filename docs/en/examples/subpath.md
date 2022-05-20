---
sidebar_label: Mount Subdirectory
---

# How to mount subdirectory in Kubernetes

This document shows how to mount with subdirectory in Kubernetes.

## Using `subPath`

The community edition and the cloud service edition are used in the same way.

You can use `subPath` in PV:

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

Apply PVC and sample pod as follows:

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

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app-subpath
```

Also you can verify that data is written onto JuiceFS file system:

```sh
kubectl exec -ti juicefs-app-subpath -- tail -f /data/out.txt
```

## Using `subdir`

If you are using the cloud service edition, and the token you use only has permission for subdirectory, you can use the following methods, only need to specify `subdir=xxx` in `mountOptions`:

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

Apply PVC and sample pod as follows:

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


## Using `pathPattern`

:::note
This feature requires JuiceFS CSI Driver version 0.13.3 and above.
:::

`pathPattern` allows you to customize the format of subdirectories of different PVs in the `StorageClass`, you can specify a template for creating directory paths from PVC metadata such as labels, annotations, names, or namespaces. It is turned off by default and needs to be turned on manually, as follows:

```bash
kubectl -n kube-system patch sts juicefs-csi-controller --type='json' -p='[{"op": "remove", "path": "/spec/template/spec/containers/1"}, {"op": "replace", "path": "/spec/template/spec/containers/0/args", "value":["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--provisioner=true"]}]'
```

Make sure pods of JuiceFS CSI Controller are restarted:

```bash
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
juicefs-csi-controller-0                2/2     Running   0                24m
```

You can use `pathPattern` in `StorageClass` like this:

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

The usage is `${.PVC.<metadata>}`. For examples:

1. If the folder name is `<pvc-namespace>-<pvc-name>`, the `pathPattern` is `${.PVC.namespace}-${.PVC.name}`.
2. If the folder name is the value of the label `a` of PVC, the `pathPattern` is `${.PVC.labels.a}`.
3. If the folder name is the value of the annotation `a` of PVC, the `pathPattern` is `${.PVC.annotations.a}`.
