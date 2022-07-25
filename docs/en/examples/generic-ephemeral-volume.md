---
sidebar_label: Use Generic Ephemeral Volumes
---

# How to use generic ephemeral volumes for JuiceFS in Kubernetes

Kubernetes' [Generic Ephemeral Volumes](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes) are similar to `emptyDir`, provide a per-pod directory for scratch data. This document shows how to use generic ephemeral volume for JuiceFS.

## Prerequisite

### Create Secret

Create a `Secret` in Kubernetes. Refer to document: [Create Secret](./dynamic-provisioning.md#prerequisite).

### Create StorageClass

Create a `StorageClass` based on the `Secret` created in the previous step:

```yaml
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
```

## Use generic ephemeral volumes in Pod

Generic ephemeral volume can be declared directly in the Pod:

```yaml {19-30}
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
    ephemeral:
      volumeClaimTemplate:
        metadata:
          labels:
            type: juicefs-ephemeral-volume
        spec:
          accessModes: [ "ReadWriteMany" ]
          storageClassName: "juicefs-sc"
          resources:
            requests:
              storage: 1Gi
```

With a generic ephemeral volume, Kubernetes automatically creates a PVC for the pod,
and when the pod is destroyed, the PVC is also destroyed.
