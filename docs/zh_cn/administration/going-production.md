---
title: 生产环境部署建议
sidebar_position: 1
---

本章介绍在生产环境中使用 CSI 驱动的一系列最佳实践，以及注意事项。

## PV 设置

在生产环境中，推荐这样设置 PV：

* 启用[「挂载点自动恢复」](../guide/pv.md#automatic-mount-point-recovery)
* 不建议使用 `--writeback`，容器场景下，如果配置不当，极易引发丢数据等事故，详见[「客户端写缓存（社区版）」](https://juicefs.com/docs/zh/community/cache_management#writeback)或[「客户端写缓存（云服务）」](https://juicefs.com/docs/zh/cloud/guide/cache/#client-write-cache)。

## 在 EFK 中收集 Mount Pod 日志

CSI 驱动的问题排查，往往涉及到查看 Mount Pod 日志。如果[实时查看 Mount Pod 日志](./troubleshooting.md#check-mount-pod)无法满足你的需要，考虑搭建 EFK（Elasticsearch + Fluentd + Kibana），或者其他合适的容器日志收集系统，用来留存和检索 Pod 日志。以 EFK 为例：

- Elasticsearch：负责对日志进行索引，并提供了一个完整的全文搜索引擎，可以方便用户从日志中检索需要的数据。安装方法请参考[官方文档](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html)。
- Fluentd：负责获取容器日志文件、过滤和转换日志数据，然后将数据传递到 Elasticsearch 集群。安装方法请参考[官方文档](https://docs.fluentd.org/installation)。
- Kibana：负责对日志进行可视化分析，包括日志搜索、处理以及绚丽的仪表板展示等。安装方法请参考[官方文档](https://www.elastic.co/guide/en/kibana/current/install.html)。

Mount Pod 均包含固定的 `app.kubernetes.io/name: juicefs-mount` 标签。在 Fluentd 的配置文件中可以配置收集对应标签的日志：

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

然后在 Fluentd 的配置文件中加上如下解析插件：

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
