---
title: Going Production
sidebar_position: 1
---

Best practices and recommended settings when going production.

## PV settings

Below settings are recommended for a production environment.

* Enable [Automatic Mount Point Recovery](../guide/pv.md#automatic-mount-point-recovery)
* `--writeback` is strongly advised against, as it can easily cause data loss especially when used inside containers, if not properly managed. See [Write Cache in Client (Cloud Service)](https://juicefs.com/docs/cloud/guide/cache/#client-write-cache) and [Write Cache in Client (Community Edition)](https://juicefs.com/docs/community/cache_management/#writeback)

## Collect mount pod logs using EFK

Troubleshooting CSI Driver usually involves reading mount pod logs, if [checking mount pod logs in real time](./troubleshooting.md#check-mount-pod) isn't enough, consider deploying an EFK (Elasticsearch + Fluentd + Kibana) stack (or other suitable systems) in Kubernetes Cluster to collect pod logs for query. Taking EFK for example:

- Elasticsearch: index logs and provide a complete full-text search engine, which can facilitate users to retrieve the required data from the log. For installation, please refer to the [official documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html).
- Fluentd: fetch container log files, filter and transform log data, and then deliver the data to the Elasticsearch cluster. For installation, please refer to the [official documentation](https://docs.fluentd.org/installation).
- Kibana: visual analysis of logs, including log search, processing, and gorgeous dashboard display, etc. For installation, please refer to the [official documentation](https://www.elastic.co/guide/en/kibana/current/install.html).

Mount pod is labeled `app.kubernetes.io/name: juicefs-mount`. Add below config to the Fluentd configuration:

```xml
<filter kubernetes.**>
  @id filter_log
  @type grep
  <regexp>
    key $.kubernetes.labels.app_kubernetes_io/name
    pattern ^juicefs-mount$
  </regexp>
</filter>
```

And add the following parser plugin to the Fluentd configuration file:

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
