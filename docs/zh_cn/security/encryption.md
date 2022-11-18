---
title: 启用数据加密
---

JuiceFS 支持数据加密，在 CSI 驱动中，你需要将密钥配置加入 Kubernetes Secret，令 JuiceFS CSI 驱动启用加密。

## 开启 CSI 的相关功能

该功能需要 CSI Node Service 在启动参数中加入 `--format-in-pod=true`（该参数在 0.13.0 引入），请对当前部署做确认，如有需要，可以用以下命令手动开启：

```shell
kubectl -n kube-system patch daemonset juicefs-csi-node --patch '{"spec": {"template": {"spec": {"containers": [{"name": "juicefs-plugin","args": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--format-in-pod=true"]}]}}}}'
# 确保 JuiceFS CSI Node Service 的 pod 均已重建
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver
```

## 在 Secret 中设置秘钥配置信息

### 社区版

参考[启用静态加密](https://juicefs.com/docs/zh/community/security/encrypt/#%E5%90%AF%E7%94%A8%E9%9D%99%E6%80%81%E5%8A%A0%E5%AF%86)生成秘钥后创建 Kubernetes Secret：

```yaml {13-16}
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
  # 私钥口令
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
  # 生成的秘钥文件原文
  encrypt_rsa_key: <PRIVATE_KEY>
```

### 云服务版

#### 托管密钥

参考[「托管密钥」](https://juicefs.com/docs/zh/cloud/encryption#%E6%89%98%E7%AE%A1%E5%AF%86%E9%92%A5)在云服务中启用加密，然后用相关信息创建 Kubernetes Secret：

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
  # 私钥口令
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
```

#### 自行管理密钥

参考[「自行管理密钥」](https://juicefs.com/docs/zh/cloud/encryption#%E8%87%AA%E8%A1%8C%E7%AE%A1%E7%90%86%E5%AF%86%E9%92%A5)生成密钥。生成密钥后，然后创建 Kubernetes Secret：

```yaml {11-14}
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
  # 私钥口令
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
  # 生成的私钥文件原文
  encrypt_rsa_key: <PRIVATE_KEY>
```
