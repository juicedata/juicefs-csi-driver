---
sidebar_label: 设置文件系统初始化选项
---

# 如何在 Kubernetes 中设置文件系统初始化选项

:::note 注意
此特性需使用 0.13.3 及以上版本的 JuiceFS CSI 驱动
:::

JuiceFS CSI 驱动支持设置 [`juicefs format`](https://juicefs.com/docs/zh/community/command_reference#juicefs-format)（社区版）或 [`juicefs auth`](https://juicefs.com/docs/zh/cloud/commands_reference#auth)（云服务版）的命令行选项来初始化文件系统。本文档展示了如何在 Kubernetes 中将文件系统初始化选项应用到 JuiceFS，社区版和云服务版的命令行选项不同，但在 CSI 驱动中的使用方式相同。

在创建 `Secret` 时（不管是[「静态配置」](static-provisioning.md)还是[「动态配置」](dynamic-provisioning.md)），添加 `format-options` 选项，将需要设置的配置项以 `,` 连接填入，如下（以社区版命令行选项为例）：

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

在 `Secret` 中 `format-options` 的优先级比其它选项更高，例如 `Secret` 中设置了 `access-key`，同时 `format-options` 中也设置了 `access-key`，那么在执行 `juicefs format` 命令时会优先使用 `format-options` 中设置的值。

社区版的具体配置选项请参考[文档](https://juicefs.com/docs/zh/community/command_reference#juicefs-format)，云服务版的具体配置选项请参考[文档](https://juicefs.com/docs/zh/cloud/commands_reference#auth)。
