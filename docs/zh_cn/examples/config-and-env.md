---
sidebar_label: 在 Mount Pod 中设置配置文件和环境变量
---

# 如何在 JuiceFS mount pod 中设置配置文件和环境变量

本文档展示了如何在 JuiceFS mount pod 中设置配置文件和环境变量，以设置谷歌云服务帐号的密钥文件和相关环境变量为例。

## 在 Secret 中设置配置文件和环境变量

请先参考谷歌云文档了解如何进行 [身份验证](https://cloud.google.com/docs/authentication) 和 [授权](https://cloud.google.com/iam/docs/overview) 工作。

将手动生成的[服务帐号密钥文件](https://cloud.google.com/docs/authentication/production#create_service_account)经过 base64 编码之后的结果放入 Kubernetes secret 的 `data` 字段中，key 就是你要放入 mount pod 的配置文件名（如 `application_default_credentials.json`）：

```yaml
kubectl apply -f - <<EOF
apiVersion: v1
data:
  application_default_credentials.json: eyAiY2xpZW50X2lkIjogIjc2NDA4NjA1MTg1MC02cXI0cDZncGk2aG41MDZwdDhlanVxODNkaT*****=
kind: Secret
metadata:
  name: gc-secret
  namespace: kube-system
type: Opaque
EOF
```

在 Kubernetes 中为 CSI 驱动程序创建 Secret。其中 `configs` 的 key 是上面创建出来的 secret 名称，value 是配置文件保存在 mount pod 中的根路径。`envs` 是希望为 mount pod 设置的环境变量。

社区版和云服务版其他参数有所区别，分别如下：

### 社区版

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

### 云服务版

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=${JUICEFS_NAME} \
    --from-literal=token=${JUICEFS_TOKEN} \
    --from-literal=accesskey=${JUICEFS_ACCESSKEY} \
    --from-literal=secretkey=${JUICEFS_SECRETKEY} \
    --from-literal=envs={"GOOGLE_APPLICATION_CREDENTIALS": "/root/.config/gcloud/application_default_credentials.json"} \
    --from-literal=configs={"gc-secret": "/root/.config/gcloud"}
```

## 部署

您可以使用 [静态配置](static-provisioning.md) 或 [动态配置](dynamic-provisioning.md)。这里以动态配置为例：

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

## 检查 pod 是否使用了 JuiceFS 文件系统

当所有的资源创建好之后，验证 pod 是否 Running 状态：

```sh
kubectl get pods juicefs-app
```

验证环境变量已经正确设置：

```sh
$ kubectl -n kube-system get po juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -oyaml | grep env -A 4
    env:
    - name: JFS_FOREGROUND
      value: "1"
    - name: GOOGLE_APPLICATION_CREDENTIALS
      value: /root/.config/gcloud/application_default_credentials.json
```

您还可以验证配置文件是否在您设置的路径中：

```sh
$ kubectl -n kube-system exec -it juicefs-kube-node-3-pvc-6289b8d8-599b-4106-b5e9-081e7a570469 -- cat /root/.config/gcloud/application_default_credentials.json
{ "client_id": "764086051850-6qr4p6g****", "client_secret": "*****", "refresh_token": "******", "type": "authorized_user" }
```
