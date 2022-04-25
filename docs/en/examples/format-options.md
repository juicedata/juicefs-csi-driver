---
sidebar_label: Config Format Options
---

# How to use Format Options in Kubernetes

CSI Driver supports the `juicefs format` command line options to set JuiceFS configuration. This document shows how to apply the format options to JuiceFS in Kubernetes.
Community edition and Cloud Service edition have different parameters, but are used in the same way in CSI.

When creating a Secret, add the `format-options` parameter. The configuration items that need to be set are filled in with the `,` connection, as follows:

```yaml {9}
apiVersion: v1
stringData:
  name: <NAME>
  storage: s3
  metaurl: redis://[:<PASSWORD>]@<HOST>:6379[/<DB>]
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  format-options: trash-days=1,block-size=4096
kind: Secret
metadata:
  name: juicefs-secret
  namespace: kube-system
type: Opaque
```

Community Edition configuration refer to [documentation](https://juicefs.com/docs/community/command_reference#juicefs-format);
Cloud Service Edition configuration refer to [documentation](https://juicefs.com/docs/cloud/commands_reference#auth).
