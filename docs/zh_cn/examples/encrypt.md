---
sidebar_label: 数据加密
---

# 如何在 Kubernetes 中设置数据加密

:::note 注意
此特性需使用 0.13.0 及以上版本的 JuiceFS CSI 驱动
:::

JuiceFS 支持数据加密功能，本文档展示如何在 Kubernetes 中使用 JuiceFS 的数据加密功能。

## 开启 CSI 的相关功能

该功能依赖 mount pod 挂载配置文件，v0.13.0 版本默认关闭，需要手动开启，执行如下命令：

```shell
$ kubectl -n kube-system patch ds juicefs-csi-node --patch '{"spec": {"template": {"spec": {"containers": [{"name": "juicefs-plugin","args": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--format-in-pod=true"]}]}}}}'
daemonset.apps/juicefs-csi-node patched
```

确保 JuiceFS CSI node 的 pod 均已重建。

## 在 Secret 中设置秘钥配置信息

### 社区版

秘钥管理参考[这篇文档](https://juicefs.com/docs/zh/community/security/encrypt#%E5%AF%86%E9%92%A5%E7%AE%A1%E7%90%86)
生成秘钥后创建 Secret，如下：

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
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
  encrypt_rsa_key: <PATH_TO_PRIVATE_KEY>
```

其中，`PASSPHRASE` 为创建秘钥时所用的密码，`PATH_TO_PRIVATE_KEY` 为生成的秘钥文件的路径。

### 云服务版

#### 托管密钥

托管秘钥的使用参考 [这篇文档](https://juicefs.com/docs/zh/cloud/encryption#%E6%89%98%E7%AE%A1%E5%AF%86%E9%92%A5)
创建 Secret：

```yaml {11}
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
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
```

其中，`PASSPHRASE` 为在 JuiceFS 官方控制台开启存储加密功能时使用的密码。

#### 自行管理密钥

秘钥的生成参考 [这篇文档](https://juicefs.com/docs/zh/cloud/encryption#%E8%87%AA%E8%A1%8C%E7%AE%A1%E7%90%86%E5%AF%86%E9%92%A5)
生成秘钥后创建 Secret，如下：

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
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
  encrypt_rsa_key: <PATH_TO_PRIVATE_KEY>
```

其中，`PASSPHRASE` 为创建秘钥时所用的密码，`PATH_TO_PRIVATE_KEY` 为生成的秘钥文件的路径。

## 部署

创建好 Secret 后，有两种方式使用 JuiceFS。可以采用静态配置，参考 [这篇文档](./static-provisioning.md)；也可以采用动态配置，参考 [这篇文档](./dynamic-provisioning.md)。
