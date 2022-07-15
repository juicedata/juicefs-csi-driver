---
slug: collect-mount-pod-logs
sidebar_label: Collect Mount Pod Logs
---

# How to collect Mount Pod logs in Kubernetes

When the application pod fails to start or an exception occurs, it is usually necessary to view the logs of the JuiceFS client (Mount Pod) to troubleshoot.
This document describes how to collect logs that preserve JuiceFS Mount Pods in a Kubernetes environment.

## Build EFK stack

We can build an EFK (Elasticsearch + Fluentd + Kibana) stack in Kubernetes Cluster to collect Pod logs.

- Elasticsearch: index logs and provide a complete full-text search engine, which can facilitate users to retrieve the required data from the log. For installation, please refer to the [official documentation](https://www.elastic.co/guide/en/elasticsearch/reference/current/install-elasticsearch.html).
- Fluentd: fetch container log files, filter and transform log data, and then deliver the data to the Elasticsearch cluster. For installation, please refer to the [official documentation](https://docs.fluentd.org/installation).
- Kibana: visual analysis of logs, including log search, processing, and gorgeous dashboard display, etc. For installation, please refer to the [official documentation](https://www.elastic.co/guide/en/kibana/current/install.html).

## Collect logs of Mount Pod

When the JuiceFS CSI driver creates a Mount Pod, it adds label `app.kubernetes.io/name: juicefs-mount` to Mount Pod. In Fluentd's configuration file, you can add the following config:

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

At the same time, the following parser plugin can be added to the Fluentd configuration file:

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
