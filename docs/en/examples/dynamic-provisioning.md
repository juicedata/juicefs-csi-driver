---
sidebar_label: Dynamic Provisioning
---

# Dynamic Provisioning Of JuiceFS Using in Kubernetes

This document shows how to make a dynamic provisioned JuiceFS volume mounted inside container.

## Prerequisite

To create the CSI Driver `Secret` in Kubernetes, the required fields for the community edition and the cloud service edition are different, as follows:

### Community edition

Take Amazon S3 as an example:

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] \
    --from-literal=storage=s3 \
    --from-literal=bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --from-literal=access-key=<ACCESS_KEY> \
    --from-literal=secret-key=<SECRET_KEY>
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

```shell
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=${JUICEFS_NAME} \
    --from-literal=token=${JUICEFS_TOKEN} \
    --from-literal=accesskey=${JUICEFS_ACCESSKEY} \
    --from-literal=secretkey=${JUICEFS_SECRETKEY}
```

- `name`: JuiceFS file system name
- `token`: JuiceFS managed token. Read [this document](https://juicefs.com/docs/cloud/metadata#token-management) for more details.
- `accesskey`: Object storage access key
- `secretkey`: Object storage secret key

You should ensure `accesskey` and `secretkey` pair has `GetObject`, `PutObject`, `DeleteObject` permission for the object storage bucket.

## Apply

Create StorageClass, PersistentVolumeClaim (PVC) and sample pod

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
