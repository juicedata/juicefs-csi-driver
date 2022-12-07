---
sidebar_label: 在 Mount Pod 中设置配置文件和环境变量
---

# 如何在 Mount Pod 中设置配置文件和环境变量

本文档展示了如何在 Mount Pod 中设置配置文件和环境变量，以设置谷歌云服务帐号的密钥文件和相关环境变量为例。

## 在 Secret 中设置配置文件和环境变量

请先参考谷歌云文档了解如何进行 [身份验证](https://cloud.google.com/docs/authentication) 和 [授权](https://cloud.google.com/iam/docs/overview) 工作。

将手动生成的[服务帐号密钥文件](https://cloud.google.com/docs/authentication/production#create_service_account)经过 Base64 编码之后的结果放入 Kubernetes Secret 的 `data` 字段中，key 就是你要放入 mount pod 的配置文件名（如 `application_default_credentials.json`）：

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

在 Kubernetes 中为 CSI 驱动程序创建 Secret，新增 `configs` 与 `envs` 参数。其中 `configs` 的 key 是上面创建出来的 Secret 名称，value 是配置文件保存在 mount pod 中的根路径。`envs` 是希望为 mount pod 设置的环境变量。

社区版和云服务版其他参数有所区别，分别如下：

### 社区版

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

### 云服务版

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

## 部署

您可以使用 [静态配置](../guide/pv.md#static-provisioning) 或 [动态配置](../guide/pv.md#dynamic-provisioning)。这里以动态配置为例：

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
    - name: app
      args:
        - -c
        - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
      command:
        - /bin/sh
      image: centos
      volumeMounts:
        - mountPath: /data
          name: juicefs-pv
  volumes:
    - name: juicefs-pv
      persistentVolumeClaim:
        claimName: juicefs-pvc
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
