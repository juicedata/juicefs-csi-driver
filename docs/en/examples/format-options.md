---
sidebar_label: Config File System Initialization Options
---

# How to config file system initialization options in Kubernetes

:::note
This feature requires JuiceFS CSI Driver version 0.13.3 and above.
:::

JuiceFS CSI Driver support setting [`juicefs format`](https://juicefs.com/docs/community/command_reference#juicefs-format) (Community Edition) or [`juicefs auth`](https://juicefs.com/docs/cloud/commands_reference#auth) (Cloud Service Edition) to initialize the file system. This document shows how to apply file system initialization options to JuiceFS in Kubernetes. The command line options are different for the community edition and cloud service edition, but are used in the same way in the CSI Driver.

When creating a `Secret` (either ["Static Provisioning"](static-provisioning.md) or ["Dynamic Provisioning"](dynamic-provisioning.md)), add the `format-options` option, and fill in the configuration items that need to be set with the `,` connection, as follows (take the community edition command line options as an example):

```yaml {14}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
  namespace: kube-system
type: Opaque
stringData:
  name: <NAME>
  storage: s3
  metaurl: redis://[:<PASSWORD>]@<HOST>:6379[/<DB>]
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  format-options: trash-days=1,block-size=4096
```

In `Secret`, `format-options` has higher priority than other options. For example, `Secret` sets `access-key`, and `format-options` also sets `access-key`, then in when executing the `juicefs format` command, the value set in `format-options` will be used first.

For the specific configuration options of the community edition, please refer to the [document](https://juicefs.com/docs/community/command_reference#juicefs-format), and for the specific configuration options of the cloud service edition, please refer to the [document](https://juicefs.com/docs/cloud/commands_reference#auth).
