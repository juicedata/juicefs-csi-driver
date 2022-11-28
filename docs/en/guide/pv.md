---
title: Create and Use PV
sidebar_position: 1
---

## Create mount configuration {#create-mount-config}

With JuiceFS CSI Driver, mount configurations are stored inside a Kubernetes Secret, create it before use.

:::note
If you're already [managing StorageClass via Helm](../getting_started.md#helm-sc), then the needed Kubernetes Secret is already created along the way, in this case we recommend you to continue managing StorageClass and Kubernetes Secret by Helm, rather than creating a separate Secret using kubectl.
:::

### Community edition

Before using PV, you should [create a file system](https://juicefs.com/docs/community/quick_start_guide/#creating-a-file-system), for example:

```shell
juicefs format \
    --storage=s3 \
    --bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --access-key=<ACCESS_KEY> --secret-key=<SECRET_KEY> \
    <META_URL> \
    <NAME>
```

And then create Kubernetes secret:

```yaml {7-16}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <JUICEFS_NAME>
  metaurl: <META_URL>
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  # Adjust mount pod timezone, defaults to UTC.
  # envs: "{TZ: Asia/Shanghai}"
  # You can also choose to format a volume within the mount pod fill in format options below.
  # format-options: trash-days=1,block-size=4096
```

Fields description:

- `name`: The JuiceFS file system name.
- `metaurl`: Connection URL for metadata engine. Read [Set Up Metadata Engine](https://juicefs.com/docs/community/databases_for_metadata) for details.
- `storage`: Object storage type, such as `s3`, `gs`, `oss`. Read [Set Up Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage) for the full supported list.
- `bucket`: Bucket URL. Read [Set Up Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage) to learn how to setup different object storage.
- `access-key`/`secret-key`: Object storage credentials.
- `envs`：Mount pod environment variables.
- `format-options`: Options used when creating a JuiceFS volume, see [`juicefs format`](https://juicefs.com/docs/community/command_reference#format). This options is only available in v0.13.3 and above.

Information like `access-key` can be specified both as a Secret `stringData` field, and inside `format-options`. If provided in both places, `format-options` will take precedence.

### Cloud service

Before continue, you should have already [created a file system](https://juicefs.com/docs/cloud/getting_started#create-file-system).

Create Kubernetes Secret:

```yaml {7-16}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <JUICEFS_NAME>
  metaurl: <META_URL>
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  # Adjust mount pod timezone, defaults to UTC.
  # envs: "{TZ: Asia/Shanghai}"
  # You can also choose to run juicefs auth within the mount pod fill in auth parameters below.
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

Fields description:

- `name`: The JuiceFS file system name.
- `token`: Token used to authenticate against JuiceFS Volume, see [Access token](https://juicefs.com/docs/cloud/acl#access-token).
- `access-key`/`secret-key`: Object storage credentials.
- `envs`：Mount pod environment variables.
- `format-options`: Options used by the [`juicefs auth`](https://juicefs.com/docs/cloud/commands_reference#auth) command, this command deals with authentication and generate local mount configuration. This options is only available in v0.13.3 and above.

Information like `access-key` can be specified both as a Secret `stringData` field, and inside `format-options`. If provided in both places, `format-options` will take precedence.

For Cloud Service, the `juicefs auth` command is somewhat similar to the `juicefs format` in JuiceFS Community Edition, thus CSI Driver uses `format-options` for both scenarios.

## Dynamic provisioning {#dynamic-provisioning}

Read [Usage](../introduction.md#usage) to learn about dynamic provisioning. Dynamic provisioning automatically creates PV for you, and the parameters needed by PV resides in StorageClass, thus you'll have to [create a StorageClass](../getting_started.md#create-storage-class) in advance.

### Deploy

Create PersistentVolumeClaim (PVC) and example pod:

```yaml
kubectl apply -f - <<EOF
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
EOF
```

Verify that pod is running, and check if data is written into JuiceFS:

```shell
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

## Use generic ephemeral volume {#general-ephemeral-storage}

[Generic ephemeral volumes](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes) are similar to `emptyDir`, which provides a per-pod directory for scratch data. When application pods are in need of large volume ephemeral storage, consider using JuiceFS as generic ephemeral volume.

Generic ephemeral volume works similar to dynamic provisioning, thus you'll need to [create a StorageClass](../getting_started.md#create-storage-class) as well. But generic ephemeral volume uses `volumeClaimTemplate` which automatically creates PVC for you.

Declare generic ephemeral volume directly in pod definition:

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

:::note
As for reclaim policy, generic ephemeral volume works the same as dynamic provisioning, so if you changed [the default PV reclaim policy](./resource-optimization.md#reclaim-policy) to `Retain`, the ephemeral volume introduced in this section will no longer be ephemeral, you'll have to manage PV lifecycle yourself.
:::

## Static provisioning {#static-provisioning}

Read [Usage](../introduction.md#usage) to learn about static provisioning.

Static provisioning means you are in charge of creating and managing PV/PVC, similar to [Configure a Pod to Use a PersistentVolume for Storage](https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage/).

Although dynamic provisioning saves you from manually creating PVs, static provisioning is still helpful when you already have lots of data stored in JuiceFS, and wants to directly expose to Kubernetes pods.

### Deploy

Create PersistentVolume (PV), PersistentVolumeClaim (PVC) and example pod:

:::note
The PV `volumeHandle` needs to be unique within the cluster, simply using the PV name is recommended.
:::

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
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
---
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

After all resources are created, verify that all is working well:

```shell
# Verify PV is created
kubectl get pv

# Verify the pod is running
kubectl get pods

# Verify that data is written into JuiceFS
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

You can customize mount options by appending `mountOptions` to above PV definition:

```yaml {8-13}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  mountOptions:
    - enable-xattr
    - max-uploads=50
    - cache-size=2048
    - cache-dir=/var/foo
    - allow_other
  ...
```

Mount options are different between Community Edition and Cloud Service:

- [Community edition](https://juicefs.com/docs/community/command_reference#juicefs-mount)
- [Cloud service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount)

## Common PV settings

### Automatic Mount Point Recovery

JuiceFS CSI Driver supports automatic mount point recovery since v0.10.7, when mount pod run into problems, a simple restart (or re-creation) can bring back JuiceFS mountpoint, and application pods can continue to work.

Applications need to [set `mountPropagation` to `HostToContainer` or `Bidirectional`](https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation) in pod `volumeMounts`. In this way, host mount is propagated to the pod:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: juicefs-app-static-deploy
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: app
          # Required when using Bidirectional
          # securityContext:
          #   privileged: true
          volumeMounts:
            - mountPath: /data
              name: data
              mountPropagation: HostToContainer
          ...
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: juicefs-pvc-static
```

### PV storage capacity {#storage-capacity}

For now, JuiceFS CSI Driver doesn't support setting storage capacity. the storage specified under PersistentVolume and PersistentVolumeClaim is simply ignored, just use a reasonable size as placeholder (e.g. `100Gi`).

```yaml
resources:
  requests:
    storage: 100Gi
```

### Access modes {#access-modes}

JuiceFS PV supports `ReadWriteMany` and `ReadOnlyMany` as access modes, change the `accessModes` field accordingly in above PV/PVC (or `volumeClaimTemplate`) definitions.
