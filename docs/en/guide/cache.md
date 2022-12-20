---
title: Cache
sidebar_position: 2
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Set cache directory {#cache-dir}

For Kubernetes nodes, a dedicated disk is often used as data and cache storage, be sure to properly configure the cache directory, or JuiceFS cache will by default be written to `/var/jfsCache`, which can easily eat up system storage space.

After cache directory is set, it'll be accessible in the mount pod via `hostPath`, you'll also need to configure other cache related options (like `--cache-size`) according to [Adjust mount options](./pv.md#mount-options).

:::note
Different from the `--cache-dir` parameter in the JuiceFS Client, the `cache-dir` parameter does not support wildcard character, if you need to use multiple disks as storage devices, specify multiple directories joined by the `:` character. See [JuiceFS Community Edition](https://juicefs.com/docs/community/command_reference/#mount) and [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount).
:::

## Static provisioning

Set cache directory using `spec.mountOptions` in PV:

```yaml {15}
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
  mountOptions:
    - cache-dir=/dev/vdb1
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

After PV is created and mounted, [Check the mount pod command](../administration/troubleshooting.md#check-mount-pod) to make sure the options contain the newly set cache directory.

### Dynamic provisioning

Specify `mountOptions` in StorageClass:

```yaml {12}
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
mountOptions:
  - cache-dir=/dev/vdb1
```

After PV is created and mounted, [Check the mount pod command](../administration/troubleshooting.md#check-mount-pod) to make sure the options contain the newly set cache directory.

## Use PVC as cache path

From 0.15.1 and above, JuiceFS CSI Driver supports using a PVC as cache directory. This is often used in hosted Kubernetes clusters provided by cloud services, which allows you to use a dedicated cloud disk as cache storage for CSI Driver.

First, create a PVC according to your cloud service provider's manual, for example:

* [Amazon EBS CSI driver](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html)
* [DigitalOcean Volumes Block Storage](https://docs.digitalocean.com/products/kubernetes/how-to/add-volumes/)

Assuming a PVC named `ebs-pvc` is already created under the same namespace as the mount pod (default to `kube-system`), use below example to use this PVC as cache directory for JuiceFS CSI Driver.

### Static provisioning

Use this PVC in a JuiceFS PV:

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
      juicefs/mount-cache-pvc: "ebs-pvc"
```

### Dynamic provisioning

To use `ebs-pvc` in StorageClass:

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
  juicefs/mount-cache-pvc: "ebs-pvc"
```

## Cache warm-up

JuiceFS Client runs inside the mount pod, so cache warm-up has to happen inside the mount pod, use below commands to enter the mount pod and carry out the warm-up:

```shell
# Application pod information will be used in below commands, save them as environment variables.
APP_NS=default  # application pod namespace
APP_POD_NAME=example-app-xxx-xxx

# Enter the mount pod using a single command
kubectl -n kube-system exec -it $(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')) -- bash

# Locate the JuiceFS mount point inside pod
df -h | grep JuiceFS
```

JuiceFS Client binary path is different across Community Edition and Cloud Service:

<Tabs>
  <TabItem value="community-edition" label="Community Edition">

```shell
/usr/local/bin/juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data
```

  </TabItem>
  <TabItem value="cloud-service" label="Cloud Service">

```shell
/usr/bin/juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data
```

  </TabItem>
</Tabs>

On the other hand, if you've already install JuiceFS Client in the application pod, it's OK to run the warm-up command directly inside the application pod.

## Clean cache when mount pod exits {#mount-pod-clean-cache}

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
