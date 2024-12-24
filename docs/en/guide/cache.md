---
title: Cache
sidebar_position: 3
description: Learn about JuiceFS cache settings and best practices for JuiceFS CSI Driver.
---

JuiceFS comes with a powerful cache design, read more in [JuiceFS Community Edition](https://juicefs.com/docs/community/guide/cache), [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/guide/cache). This chapter introduces cache related settings and best practices in CSI Driver.

With CSI Driver, you can use either a host directory or a PVC for cache storage. Their main differences are in isolation level and data locality rather than performance. Here is a breakdown:

* Host directories (`hostPath`) are easy to use. Cache data is stored directly on local cache disks, so observation and management are fairly straightforward. However, if Mount Pods (with application Pods) get scheduled to different nodes, all cache content will be lost, leaving residual data that might need to be cleaned up in this process (read sections below on cache cleanup). If you have no special requirements on isolation or data locality, use this method.
* If all worker nodes are used to run JuiceFS Mount Pods, and they host similar cache content (similar situation if you use distributed caching), Pod migration is not really a problem, and you can still use host directories for cache storage.
* When using a PVC for cache storage, different JuiceFS PVs can isolate cache data. If the Mount Pod is migrated to another node, the PVC reference remains the same. This ensures that the cache is unaffected.

## Using host directories (`hostPath`) {#cache-settings}

For Kubernetes nodes, a dedicated disk is often used as data and cache storage, be sure to properly configure the cache directory, or JuiceFS cache will by default be written to `/var/jfsCache`, which can easily eat up system storage space.

After the cache directory is set, it will be accessible in the Mount Pod via `hostPath`. You might also need to configure other cache-related options (like `--cache-size`) according to [Adjust mount options](./configurations.md#mount-options).

:::note

* In CSI Driver, `cache-dir` parameter does not support wildcard character, if you need to use multiple disks as storage devices, specify multiple directories joined by the `:` character. See [JuiceFS Community Edition](https://juicefs.com/docs/community/command_reference/#mount) and [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount).
* For scenarios that involve intensive small writes, we usually recommend users to temporarily enable client write cache, but due to its inherent risks, this is advised against when using CSI Driver, because Pod lifecycle is significantly more unstable, and can cause data loss if Pod exists unexpectedly.

:::

### Using ConfigMap

Read [Custom directory](./configurations.md#custom-cachedirs).

### Define in PV (deprecated)

Since CSI Driver v0.25.1, cache directories are supported in ConfigMap. Please refer to the previous section to manage all PV settings in a centralized place. The following practice of defining cache directories in PVs is deprecated.

Static provisioning:

```yaml {15-16}
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
    - cache-size=204800
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
```

Dynamic provisioning:

```yaml {12-13}
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
  - cache-size=204800
```

## Use PVC as cache path

From 0.15.1 and above, JuiceFS CSI Driver supports using a PVC as cache directory. This is often used in hosted Kubernetes clusters provided by cloud services, which allows you to use a dedicated cloud disk as cache storage for CSI Driver.

First, create a PVC according to your cloud service provider's manual, for example:

* [Amazon EBS CSI Driver](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html)
* [Use the Azure Disks CSI Driver in Azure Kubernetes Service (AKS)](https://learn.microsoft.com/en-us/azure/aks/azure-disk-csi)
* [Using the Google Compute Engine persistent disk CSI Driver](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver)
* [DigitalOcean Volumes Block Storage](https://docs.digitalocean.com/products/kubernetes/how-to/add-volumes)

Assuming a PVC named `jfs-cache-pvc` is already created in the same namespace as the Mount Pod (which defaults to `kube-system`), use the following example to set this PVC as the cache directory for JuiceFS CSI Driver.

### Using ConfigMap

Read [Custom directory](./configurations.md#custom-cachedirs).

### Define in PV (deprecated)

Static provisioning:

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
      juicefs/mount-cache-pvc: "jfs-cache-pvc"
```

Dynamic provisioning:

Reference `jfs-cache-pvc` in StorageClass:

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
  juicefs/mount-cache-pvc: "jfs-cache-pvc"
```

## Cache warm-up {#warmup}

The JuiceFS client runs inside the Mount Pod, so cache warm-up must be performed inside the Mount Pod. Use the commands below to enter the Mount Pod and carry out the warm-up operation:

```shell
# application Pod information will be used in below commands, save them as environment variables.
APP_NS=default  # application Pod namespace
APP_POD_NAME=example-app-xxx-xxx

# Enter the Mount Pod using a single command
kubectl -n kube-system exec -it $(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')) -- bash

# Locate the JuiceFS mount point inside pod
df -h | grep JuiceFS

# Run warmup command
juicefs warmup /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa/data
```

For [dedicated cache cluster](https://juicefs.com/docs/cloud/guide/distributed-cache) scenarios, if you need to automate the warmup process, consider using Kubernetes Job:

```yaml title="warmup-job.yaml"
apiVersion: batch/v1
kind: Job
metadata:
  name: warmup
  labels:
    app.kubernetes.io/name: warmup
spec:
  backoffLimit: 0
  activeDeadlineSeconds: 3600
  ttlSecondsAfterFinished: 86400
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: warmup
        app.kubernetes.io/name: warmup
    spec:
      serviceAccountName: default
      containers:
        - name: warmup
          command:
            - bash
            - -c
            - |
              # Below shell code is only needed in on-premise environments, which unpacks JSON and set its key-value pairs as environment variables
              for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
                echo "export $keyval"
                eval export $keyval
              done

              # Authenticate and mount JuiceFS, all environment variables comes from the volume credentials within the Kubernetes Secret
              # ref: https://juicefs.com/docs/cloud/getting_started#create-file-system
              /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

              # Mount with --no-sharing to avoid download cache data to local container storage
              # Replace CACHEGROUP with actual cache group name
              /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --cache-size=0 --cache-group=CACHEGROUP

              # Check if warmup succeeds, by default, if any of the data blocks fails to download, the command fails, and client log needs to be check for troubleshooting
              /usr/bin/juicefs warmup /mnt/jfs
              code=$?
              if [ "$code" != "0" ]; then
                cat /var/log/juicefs.log
              fi
              exit $code
          image: juicedata/mount:ee-5.0.2-69f82b3
          securityContext:
            privileged: true
          env:
            - name: VOL_NAME
              valueFrom:
                secretKeyRef:
                  key: name
                  name: juicefs-secret
            - name: ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  key: access-key
                  name: juicefs-secret
            - name: SECRET_KEY
              valueFrom:
                secretKeyRef:
                  key: secret-key
                  name: juicefs-secret
            - name: TOKEN
              valueFrom:
                secretKeyRef:
                  key: token
                  name: juicefs-secret
            - name: ENVS
              valueFrom:
                secretKeyRef:
                  key: envs
                  name: juicefs-secret
      restartPolicy: Never
```

## Cache cleanup {#mount-pod-clean-cache}

Local cache can be a precious resource, especially when dealing with large scale data. For this reason, JuiceFS CSI Driver does not delete cache by default when the Mount Pod exits. If this behavior does not fit your needs, you can configure it to clear the local cache when the Mount Pod exits.

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

## Dedicated cache cluster {#dedicated-cache-cluster}

:::note
Dedicated cache cluster is only supported in JuiceFS Cloud Service & Enterprise Edition, Community Edition is not supported.
:::

Kubernetes containers are usually ephemeral, a [distributed cache cluster](/docs/cloud/guide/distributed-cache) built on top of ever-changing containers is unstable, which really hinders cache utilization. For this type of situation, you can deploy a [dedicated cache cluster](/docs/cloud/guide/distributed-cache#dedicated-cache-cluster) to achieve a stable cache service.

There are currently two ways to deploy a distributed cache cluster in Kubernetes:

1. For most scenarios, it can be deployed through ["Cache Group Operator"](./cache-group-operator.md);
2. For scenarios that require flexible customization of deployment configuration, you can deploy it through ["Write your own YAML configuration file"](./generic-applications.md#distributed-cache-cluster).
