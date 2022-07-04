---
sidebar_label: Dynamic Provisioning
---

# Dynamic Provisioning Of JuiceFS Using in Kubernetes

This document shows how to make a dynamic provisioned JuiceFS volume mounted inside container.

## Prerequisite

To create the CSI Driver `Secret` in Kubernetes, the required fields for the community edition and the cloud service edition are different, as follows:

### Community edition

Take Amazon S3 as an example:

```yaml {7-14}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <NAME>
  metaurl: redis://[:<PASSWORD>]@<HOST>:6379[/<DB>]
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  # If you need to set the time zone of the JuiceFS Mount Pod, please uncomment the next line, the default is UTC time.
  # envs: "{TZ: Asia/Shanghai}"
```

- `name`: The JuiceFS file system name.
- `metaurl`: Connection URL for metadata engine (e.g. Redis). Read [this document](https://juicefs.com/docs/community/databases_for_metadata) for more information.
- `storage`: Object storage type, such as `s3`, `gs`, `oss`. Read [this document](https://juicefs.com/docs/community/how_to_setup_object_storage) for the full supported list.
- `bucket`: Bucket URL. Read [this document](https://juicefs.com/docs/community/how_to_setup_object_storage) to learn how to setup different object storage.
- `access-key`: Access key.
- `secret-key`: Secret key.

Replace fields enclosed by `<>` with your own environment variables. The fields enclosed `[]` is optional which related your deployment environment.

You should ensure:
1. The `access-key`, `secret-key` pair has `GetObject`, `PutObject`, `DeleteObject` permission for the object storage bucket
2. The Redis DB is clean and the password (if provided) is right

### Cloud service edition

```yaml {7-12}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${JUICEFS_ACCESSKEY}
  secret-key: ${JUICEFS_SECRETKEY}
  # If you need to set the time zone of the JuiceFS Mount Pod, please uncomment the next line, the default is UTC time.
  # envs: "{TZ: Asia/Shanghai}"
```

- `name`: JuiceFS file system name
- `token`: JuiceFS managed token. Read [this document](https://juicefs.com/docs/cloud/metadata#token-management) for more details.
- `access-key`: Object storage access key
- `secret-key`: Object storage secret key

You should ensure `access-key` and `secret-key` pair has `GetObject`, `PutObject`, `DeleteObject` permission for the object storage bucket.

## Deploy

Create StorageClass, PersistentVolumeClaim (PVC) and sample pod:

:::info
Since JuiceFS is an elastic file system it doesn't really enforce any file system capacity. The actual storage capacity value in `PersistentVolume` and `PersistentVolumeClaim` is not used when creating the file system. However, since the storage capacity is a required field by Kubernetes, you must specify the value and you can use any valid value e.g. `10Pi` for the capacity.
:::

```yaml
kubectl apply -f - <<EOF
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
---
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
