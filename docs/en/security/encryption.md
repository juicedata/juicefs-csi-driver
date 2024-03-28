---
title: Enable Data Encryption
---

JuiceFS supports data encryption, in CSI Driver, you need to add private key information to Kubernetes Secret, in order to enable encryption for JuiceFS CSI Driver.

## Enable CSI related feature

This feature demands CSI Node Service be started with `--format-in-pod=true` (available since 0.13.0), check current installation and use below command to add this parameter if in need.

```shell
kubectl -n kube-system patch ds juicefs-csi-node --patch '{"spec": {"template": {"spec": {"containers": [{"name": "juicefs-plugin","args": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--format-in-pod=true"]}]}}}}'
# Wait until JuiceFS CSI Node Service pods are re-created
kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver
```

Using Helm to install the CSI driver:
```
# values.yaml
node:
  additionalArguments:
  - "--format-in-pod=true"
```

## Set private key configuration in Secret

### Community edition

Refer to [Enable Data Encryption At Rest](https://juicefs.com/docs/community/security/encrypt/#enable-data-encryption-at-rest) to generate a private key, and then create a Kubernetes Secret:

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
  # Passphrase for private key
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
  # Generated private key string
  encrypt_rsa_key: <PRIVATE_KEY>
```

### Cloud Service edition

#### Delegated Key Management

Refer to ["Delegated Key Management"](https://juicefs.com/docs/cloud/encryption#delegated-key-management) to enable encryption in JuiceFS Cloud Service, and then create a Kubernetes Secret using relevant credentials:

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
  # passphrase for private key
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
```

#### Self Managed Key

Refer to ["Self Managed Key"](https://juicefs.com/docs/cloud/encryption#self-managed-key) to generate private key. After generating the private key, create a Kubernetes Secret as follows:

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
  # passphrase for private key
  envs: "{JFS_RSA_PASSPHRASE: <PASSPHRASE>}"
  # generated private key string
  encrypt_rsa_key: <PRIVATE_KEY>
```
