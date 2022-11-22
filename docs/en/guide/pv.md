---
title: Create and Use PV
sidebar_position: 1
---

## Create mount configuration {#create-mount-config}

With JuiceFS CSI Driver, mount configurations are stored inside a Kubernetes Secret, create it before use.

### Community edition

Before using PV, you should [create a JuiceFS volume](https://juicefs.com/docs/community/quick_start_guide/#creating-a-file-system), for example:

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
  # Adjust mount pod timezone, defaults to UTC
  # envs: "{TZ: Asia/Shanghai}"
  # You can also choose to format a volume within the mount pod
  # fill in format options below
  # format-options: trash-days=1,block-size=4096
```

- `name`: The JuiceFS file system name.
- `metaurl`: Connection URL for metadata engine. Read [Metadata Engine](https://juicefs.com/docs/community/databases_for_metadata) for details.
- `storage`: Object storage type, such as `s3`, `gs`, `oss`. Read [Set Up Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage) for the full supported list.
- `bucket`: Bucket URL. Read [Set Up Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage) to learn how to setup different object storage.
- `access-key`/`secret-key`: Object storage credentials.
- `envs`：Mount pod environment variables.
- `format-options`: Options used when creating a JuiceFS volume, see [`juicefs format`](https://juicefs.com/docs/zh/community/command_reference#format).

Information like `access-key` can be specified both as a Secret `stringData` field, and inside `format-options`. If provided in both places, `format-options` will take precedence.

### Cloud service edition

Before continue, you should have already [created a filesystem](https://juicefs.com/docs/zh/cloud/getting_started#create-file-system).

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
  # Adjust mount pod timezone, defaults to UTC
  # envs: "{TZ: Asia/Shanghai}"
  # You can also choose to run juicefs auth within the mount pod
  # fill in auth parameters below
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

- `name`: The JuiceFS file system name.
- `token`: Token used to authenticate against JuiceFS Volume, see [Access token](https://juicefs.com/docs/cloud/acl#access-token).
- `access-key`/`secret-key`: Object storage credentials.
- `envs`：Mount pod environment variables.
- `format-options`: Options used by the [`juicefs auth`](https://juicefs.com/docs/zh/cloud/commands_reference#auth) command, this command deals with authentication and generate local mount configuration.

Information like `access-key` can be specified both as a Secret `stringData` field, and inside `format-options`. If provided in both places, `format-options` will take precedence.

For Cloud Service, the `juicefs auth` command is somewhat similar to the `juicefs format` in JuiceFS Community Edition, thus CSI Driver uses `format-options` for both scenarios.

## Dynamic provisioning {#dynamic-provisioning}

Create StorageClass, PersistentVolumeClaim (PVC) and sample pod:

:::info
Since JuiceFS is an elastic file system it doesn't really enforce any file system capacity. The actual storage capacity value in `PersistentVolume` and `PersistentVolumeClaim` is not used when creating the file system. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.
:::

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

## Check JuiceFS file system is used

After the objects are created, verify that pod is running:

```sh
kubectl get pods
```

Also you can verify that data is written onto JuiceFS file system:

```sh
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```
