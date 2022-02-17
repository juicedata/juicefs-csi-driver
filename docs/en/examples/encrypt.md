---
sidebar_label: Data Encrypt
---

# How to Set Up Data Encryption in Kubernetes

> Supported Versions: >=v0.13.0

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

Key management refer to [this document](https://juicefs.com/docs/community/security/encrypt#key-management).
After generating the private key, create a Secret, as follows:

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] \
    --from-literal=storage=s3 \
    --from-literal=bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --from-literal=access-key=<ACCESS_KEY> \
    --from-literal=secret-key=<SECRET_KEY> \
    --from-literal=envs={"JFS_RSA_PASSPHRASE": <PASSPHRASE>} \
    --from-literal=encrypt_rsa_key=<PATH_TO_PRIVATE_KEY>
```

Among them, `PASSPHRASE` is the password used to create the private key, and `PATH_TO_PRIVATE_KEY` is the path to the generated private key file.

### Cloud service edition

#### Delegated Key Management

Key management refer to [this document](https://juicefs.com/docs/cloud/encryption#delegated-key-management).
Create Secret：

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] \
    --from-literal=storage=s3 \
    --from-literal=bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --from-literal=access-key=<ACCESS_KEY> \
    --from-literal=secret-key=<SECRET_KEY> \
    --from-literal=envs={"JFS_RSA_PASSPHRASE": <PASSPHRASE>} 
```

Among them, `PASSPHRASE` is the password used to enable storage encryption in the JuiceFS official console.

#### Self Managed Key

Key management refer to [this document](https://juicefs.com/docs/cloud/encryption#self-managed-key)
After generating the private key, create a Secret, as follows:

```sh
kubectl -n default create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=redis://[:<PASSWORD>]@<HOST>:6379[/<DB>] \
    --from-literal=storage=s3 \
    --from-literal=bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com \
    --from-literal=access-key=<ACCESS_KEY> \
    --from-literal=secret-key=<SECRET_KEY> \
    --from-literal=envs={"JFS_RSA_PASSPHRASE": <PASSPHRASE>} \
    --from-literal=encrypt_rsa_key=<PATH_TO_PRIVATE_KEY>
```

Among them, `PASSPHRASE` is the password used to create the private key, and `PATH_TO_PRIVATE_KEY` is the path to the generated private key file.

## Apply

There are two ways to use JuiceFS. Static provisioning can be used, refer to [this document](./static-provisioning.md). Dynamic provisioning can also be used, refer to [this document](./dynamic-provisioning.md)。
