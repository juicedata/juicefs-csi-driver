---
title: Cache
sidebar_position: 3
---

JuiceFS comes with a powerful cache design, read more in [JuiceFS Community Edition](https://juicefs.com/docs/community/guide/cache), [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/guide/cache). This chapter introduces cache related settings and best practices in CSI Driver.

## Cache settings {#cache-settings}

For Kubernetes nodes, a dedicated disk is often used as data and cache storage, be sure to properly configure the cache directory, or JuiceFS cache will by default be written to `/var/jfsCache`, which can easily eat up system storage space.

After cache directory is set, it'll be accessible in the mount pod via `hostPath`, you might also need to configure other cache related options (like `--cache-size`) according to ["Adjust mount options"](./configurations.md#mount-options).

:::note

* In CSI Driver, `cache-dir` parameter does not support wildcard character, if you need to use multiple disks as storage devices, specify multiple directories joined by the `:` character. See [JuiceFS Community Edition](https://juicefs.com/docs/community/command_reference/#mount) and [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount).
* For scenario that does intensive small writes, we usually recommend users to temporarily enable client write cache, but due to its inherent risks, this is advised against when using CSI Driver, because pod lifecycle is significantly more unstable, and can cause data loss if pod exists unexpectedly.

:::

Cache related settings is configured in [mount options](./configurations.md#mount-options), you can also refer to the straightforward examples below. After PV is created and mounted, you can also [check the mount pod command](../administration/troubleshooting.md#check-mount-pod) to make sure the options contain the newly set cache directory.

* Static provisioning:

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

* Dynamic provisioning:

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

## Cache warm-up {#warmup}

JuiceFS Client runs inside the mount pod, so cache warm-up has to happen inside the mount pod, use below commands to enter the mount pod and carry out the warm-up:

```shell
# Application pod information will be used in below commands, save them as environment variables.
APP_NS=default  # application pod namespace
APP_POD_NAME=example-app-xxx-xxx

# Enter the mount pod using a single command
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

## Dedicated cache cluster {#dedicated-cache-cluster}

:::note
Dedicated cache cluster is only supported in JuiceFS Cloud Service & Enterprise Edition, Community Edition is not supported.
:::

Kubernetes containers are usually ephemeral, a [distributed cache cluster](/docs/cloud/guide/distributed-cache) built on top of ever-changing containers is unstable, which really hinders cache utilization. For this type of situation, you can deploy a [dedicated cache cluster](/docs/cloud/guide/distributed-cache#dedicated-cache-cluster) to achieve a stable cache service.

Use below example to deploy a StatefulSet of JuiceFS clients, together they form a stable JuiceFS cache group.

A JuiceFS cache cluster is deployed with the cache group name `jfscache`, in order to use this cache cluster in application JuiceFS clients, you'll need to join them into the same cache group, and additionally add the `--no-sharing` option, so that these application clients doesn't really involve in building the cache data, this is what prevents a instable cache group.

Under dynamic provisioning, modify mount options according to below examples, see full description in [mount options](../guide/configurations.md#mount-options).

```yaml {13-14}
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
  ...
  - cache-group=jfscache
  - no-sharing
```
