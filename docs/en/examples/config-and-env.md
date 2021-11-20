# How to set config files and env in JuiceFS mount pod

This document shows how to mount config files or set envs in JuiceFS mount pod.

## set config and env in secret

This example uses google cloud platform as object. Please follow Google Cloud document to know
how [authentication](https://cloud.google.com/docs/authentication)
and [authorization](https://cloud.google.com/iam/docs/overview) work. And you create gc credential config in a right way.

Put the result of base64 gc credential config in a Kubernetes secret, and the key is the config file you will put in
mount pod:

```yaml
apiVersion: v1
data:
  application_default_credentials.json: eyAiY2xpZW50X2lkIjogIjc2NDA4NjA1MTg1MC02cXI0cDZncGk2aG41MDZwdDhlanVxODNkaT*****=
kind: Secret
metadata:
  name: gc-secret
  namespace: kube-system
type: Opaque
```

Create secrets for CSI driver in Kubernetes. The key of `configs` is the secret name, value is the path of secret being mounted in pod.

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] \
    --from-literal=storage=s3 \
    --from-literal=bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --from-literal=access-key=<ACCESS_KEY> \
    --from-literal=secret-key=<SECRET_KEY> \
    --from-literal=envs={"GOOGLE_APPLICATION_CREDENTIALS": "/root/.config/gcloud/application_default_credentials.json"} \
    --from-literal=configs={"gc-secret": "/root/.config/gcloud"}
```

## Apply 

You can use [static provision](static-provisioning.md) or [dynamic provision](dynamic-provisioning.md) . We take dynamic provision as example:

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
kubectl get pods juicefs-app
```

Verify that env you set:

```sh
$ kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -oyaml |grep env -A 4
    env:
    - name: JFS_FOREGROUND
      value: "1"
    - name: GOOGLE_APPLICATION_CREDENTIALS
      value: /root/.config/gcloud/application_default_credentials.json
```

Also you can verify that gc credential config is in path you set:

```sh
$ kubectl -n kube-system exec -it juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -- cat /root/.config/gcloud/application_default_credentials.json
{ "client_id": "764086051850-6qr4p6g****", "client_secret": "*****", "refresh_token": "******", "type": "authorized_user" }
```
