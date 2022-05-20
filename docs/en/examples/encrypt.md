---
sidebar_label: Data Encryption
---

# How to Set Up Data Encryption in Kubernetes

:::note
This feature requires JuiceFS CSI Driver version 0.13.0 and above.
:::

JuiceFS supports data encryption, this document shows how to use data encryption of JuiceFS in Kubernetes.

## Enable CSI related feature

This feature relies on the feature of set volume in mount pod. v0.13.0 is disabled by default and needs to be manually enabled. Execute the following command:

```shell
$ kubectl -n kube-system patch ds juicefs-csi-node --patch '{"spec": {"template": {"spec": {"containers": [{"name": "juicefs-plugin","args": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--format-in-pod=true"]}]}}}}'
daemonset.apps/juicefs-csi-node patched
```

Make sure that the JuiceFS CSI node's pods are all rebuilt.

## Set private key configuration in Secret

### Community edition

Key management refer to [this document](https://juicefs.com/docs/community/security/encrypt/#enable-data-encryption-at-rest).
After generating the private key, create a Secret, as follows:

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

Among them, `PASSPHRASE` is the password used to create the private key, and `PATH_TO_PRIVATE_KEY` is the path to the generated private key file.

### Cloud service edition

#### Delegated Key Management

Key management refer to [this document](https://juicefs.com/docs/cloud/encryption#delegated-key-management).
Create Secret：

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

Among them, `PASSPHRASE` is the password used to enable storage encryption in the JuiceFS official console.

#### Self Managed Key

Key management refer to [this document](https://juicefs.com/docs/cloud/encryption#self-managed-key)
After generating the private key, create a Secret, as follows:

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

Among them, `PASSPHRASE` is the password used to create the private key, and `PATH_TO_PRIVATE_KEY` is the path to the generated private key file.

## Apply

There are two ways to use JuiceFS. Static provisioning can be used, refer to [this document](./static-provisioning.md). Dynamic provisioning can also be used, refer to [this document](./dynamic-provisioning.md)。
