---
slug: /collect-mount-pod-logs
sidebar_label: 收集 Mount Pod 日志
sidebar_position: 4
---

# 如何在 Kubernetes 中收集 Mount Pod 日志

当应用 pod 无法正常启动或出现异常时，通常需要查看 JuiceFS 客户端（Mount Pod）的日志来排查问题。本文将介绍如何在 Kubernetes 环境中收集保留 JuiceFS Mount Pod 的日志。

## 搭建 EFK 体系

我们可以在 Kubernetes 环境中搭建 EFK（Elasticsearch + Fluentd + Kibana）体系，用来收集 Pod 的日志。其中：

- Elasticsearch：负责对日志进行索引，并提供了一个完整的全文搜索引擎，可以方便用户从日志中检索需要的数据。安装方法请参考[官方文档](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html)。
- Fluentd：负责获取容器日志文件、过滤和转换日志数据，然后将数据传递到 Elasticsearch 集群。安装方法请参考[官方文档](https://docs.fluentd.org/installation)。
- Kibana：负责对日志进行可视化分析，包括日志搜索、处理以及绚丽的仪表板展示等。安装方法请参考[官方文档](https://www.elastic.co/guide/en/kibana/current/install.html)。

## 收集 Mount Pod 日志

JuiceFS CSI 驱动在创建 Mount Pod 时，会为其打上固定的 `app.kubernetes.io/name: juicefs-mount` 标签。在 Fluentd 的配置文件中可以配置收集对应标签的日志：

```html
<filter kubernetes.**>
  @id filter_log
  @type grep
  <regexp>
    key $.kubernetes.labels.app_kubernetes_io/name
    pattern ^juicefs-mount$
  </regexp>
</filter>
```

同时，可以在 Fluentd 的配置文件中加上如下解析插件：

```html
<filter kubernetes.**>
  @id filter_parser
  @type parser
  key_name log
  reserve_data true
  remove_key_name_field true
  <parse>
    @type multi_format
    <pattern>
      format json
    </pattern>
    <pattern>
      format none
    </pattern>
  </parse>
</filter>
```
