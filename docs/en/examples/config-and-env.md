---
sidebar_label: Set Configuration Files and Environment Variables in Mount Pod
---

# How to set configuration files and environment variables in Mount Pod

This document shows how to set the configuration file and environment variables in Mount Pod, taking set the key file and related environment variables of the Google Cloud service account as an example.

## Set configuration files and environment variables in secret

Please refer to Google Cloud documentation to learn how to perform [authentication](https://cloud.google.com/docs/authentication) and [authorization](https://cloud.google.com/iam/docs/overview).

Put the manually generated [service account key file](https://cloud.google.com/docs/authentication/production#create_service_account) after Base64 encoding into the `data` field of the Kubernetes Secret, the key is the name of the configuration file to put in the mount pod (such as `application_default_credentials.json`):

```yaml {9}
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: gc-secret
  namespace: kube-system
type: Opaque
data:
  application_default_credentials.json: eyAiY2xpZW50X2lkIjogIjc2NDA4NjA1MTg1MC02cXI0cDZncGk2aG41MDZwdDhlanVxODNkaT*****=
EOF
```

Create a Secret for the CSI driver in Kubernetes, add `configs` and `envs` parameters. The key of `configs` is the Secret name created above, and the value is the root path of the configuration file saved in the mount pod. The `envs` is the environment variable you want to set for mount pod.

The required fields for the community edition and the cloud service edition are different, as follows:

### Community edition

```yaml {13-14}
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
  envs: "{GOOGLE_APPLICATION_CREDENTIALS: /root/.config/gcloud/application_default_credentials.json}"
  configs: "{gc-secret: /root/.config/gcloud}"
```

### Cloud service edition

```yaml {11-12}
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
  envs: "{GOOGLE_APPLICATION_CREDENTIALS: /root/.config/gcloud/application_default_credentials.json}"
  configs: "{gc-secret: /root/.config/gcloud}"
```

## Apply

You can use [static provisioning](../guide/pv.md#static-provisioning) or [dynamic provisioning](../guide/pv.md#dynamic-provisioning). Here take dynamic provisioning as example:

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
```

## Check JuiceFS file system is used

After the objects are created, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

Verify that the environment variables have been set correctly:

```sh
$ kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -oyaml |grep env -A 4
    env:
    - name: JFS_FOREGROUND
      value: "1"
    - name: GOOGLE_APPLICATION_CREDENTIALS
      value: /root/.config/gcloud/application_default_credentials.json
```

You can also verify that the configuration file is in the path you set:

```sh
$ kubectl -n kube-system exec -it juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -- cat /root/.config/gcloud/application_default_credentials.json
{ "client_id": "764086051850-6qr4p6g****", "client_secret": "*****", "refresh_token": "******", "type": "authorized_user" }
```
