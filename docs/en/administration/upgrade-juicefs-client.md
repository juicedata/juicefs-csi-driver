---
title: Upgrade JuiceFS Client
slug: /upgrade-juicefs-client
sidebar_position: 3
---

Upgrade JuiceFS Client to the latest version to enjoy all kinds of improvements and fixes, read [release notes for JuiceFS Community Edition](https://github.com/juicedata/juicefs/releases) or [release notes for JuiceFS Cloud Service](https://juicefs.com/docs/cloud/release/) to learn more. Note that if you [upgrade JuiceFS CSI Driver](./upgrade-csi-driver.md), JuiceFS Client is upgraded along the way. However, if you would like to upgrade JuiceFS Client without changes to the CSI Driver itself, read this chapter.

## Upgrade container image for mount pod {#upgrade-mount-pod-image}

From v0.17.1 and above, CSI Driver supports customizing mount pod image, you can modify config and use the latest mount pod image to upgrade JuiceFS Client. This is all possible due to [the decoupling architecture of JuiceFS CSI Driver](../introduction.md#architecture).

Find the latest mount pod image in [our image registry](https://hub.docker.com/r/juicedata/mount/tags?page=1&ordering=last_updated&name=v), image tag format is `v<JUICEFS-CE-VERSION>-<JUICEFS-EE-VERSION>`.

If the desired JuiceFS Client isn't yet released, or the latest mount pod image hasn't been built, you can also [build your own mount pod image](../development/build-juicefs-image.md#build-mount-pod-image).

### Dynamic provisioning

When using [dynamic provisioning](../guide/pv.md#dynamic-provisioning), define mount pod image in `StorageClass` definition:

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
  juicefs/mount-image: juicedata/mount:v1.0.2-4.8.1
```

Once edit is saved, newly created PV will use specified image to create mount pods.

### Static provisioning

When using [static provisioning](../guide/pv.md#static-provisioning), define mount pod image in `PersistentVolume` definition:

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
      juicefs/mount-image: juicedata/mount:v1.0.2-4.8.1
```

## Upgrade JuiceFS Client temporarily

:::tip
You are strongly encouraged to upgrade JuiceFS CSI Driver to v0.10 and later versions, the method demonstrated below are not recommended for production use.
:::

If you're using [Mount by process mode](../introduction.md#by-process), or using CSI Driver prior to v0.10.0, and cannot easily upgrade to v0.10, you can choose to upgrade JuiceFS Client independently, inside the CSI Node Service pod.

This is only a temporary solution, if CSI Node Service pods are recreated, or new nodes are added to Kubernetes cluster, you'll need to run this script again.

1. Use this script to replace the `juicefs` binary in `juicefs-csi-node` pod with the new built one:

   ```bash
   #!/bin/bash

   KUBECTL=/path/to/kubectl
   JUICEFS_BIN=/path/to/new/juicefs

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system cp $JUICEFS_BIN '{}':/tmp/juicefs -c juicefs-plugin

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system exec -i '{}' -c juicefs-plugin -- \
       chmod a+x /tmp/juicefs && mv /tmp/juicefs /bin/juicefs
   ```

   :::note
   Replace `/path/to/kubectl` and `/path/to/new/juicefs` in the script with the actual values, then execute the script.
   :::

2. Restart the applications one by one, or kill the existing pods.
