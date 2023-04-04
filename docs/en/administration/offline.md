---
title: Offline Cluster
sidebar_position: 5
---

An offline cluster is a Kubernetes cluster in which nodes cannot access the public internet, and hence cannot readily retrieve Docker images for CSI Controller components. If worker nodes in your environment cannot access [Docker Hub](https://hub.docker.com) or [Quay](https://quay.io), they are deemed offline cluster.

## Copy images {#copy-images}

This section is currently only available in Chinese.

## Change mount pod SA to allow pull image {#mount-pod-sa}

Offline clusters often use private image registries, which require authentication to access. The default ServiceAccount (SA) for mount pod is `juicefs-csi-node-sa`, which probably isn't equipped to pull images from your private registry. Follow below guide to change SA for mount pod in order to pull its image normally.

A SA called `juicefs-mount-sa` is assumed to have access to your private registry, make adjustments accordingly in your environment (refer to [Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#add-imagepullsecrets-to-a-service-account) to see how to create the SA).

### Static provisioning

Modify the `volumeAttributes` in PV, add `juicefs/mount-service-account`:

```yaml {10}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  ...
spec:
  csi:
    ...
    volumeAttributes:
      juicefs/mount-service-account: juicefs-mount-sa
  ...
```

### Dynamic provisioning

Modify the `parameters` in StorageClass, add `juicefs/mount-service-account`:

```yaml {8}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  ...
  juicefs/mount-service-account: juicefs-mount-sa
```
