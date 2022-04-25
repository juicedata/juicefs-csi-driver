---
sidebar_label: 设置 format 参数
---

# 如何在 Kubernetes 中使用 JuiceFS Format 参数

CSI Driver 支持 `juicefs format` 命令行参数来设置 JuiceFS 的配置项。本文档展示了如何 在 Kubernetes 中将 format 参数应用到 JuiceFS。社区版和云服务版的参数不同，但在 CSI
中的使用方式相同。

在创建 Secret 时，添加 `format-options` 参数，需要设置的配置项以 `,` 连接填入，如下：

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

社区版的配置参数参考 [文档](https://juicefs.com/docs/zh/community/command_reference#juicefs-format)
；云服务版的配置参数参考 [文档](https://juicefs.com/docs/zh/cloud/commands_reference#auth)。

