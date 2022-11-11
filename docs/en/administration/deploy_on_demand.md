---
slug: deploy-on-demand
---

# CSI Node Deployed on Demand

The components of JuiceFS CSI Driver are divided into CSI Controller, CSI Node and Mount Pod. For details, please refer to [JuiceFS CSI Architecture Document](../introduction.md).
CSI Controller is deployed in StatefulSet, CSI Node is deployed as DaemonSet, and the Mount Pod is the Pod of JuiceFS client.

By default, CSI Node will be started on all nodes, but in some scenarios, users may want to start CSI Node only on nodes that need to mount the JuiceFS file system, which can reduce resource usage and improve cluster availability.
This document will describe how to start the CSI Node component on demand in a Kubernetes cluster.

## Configure JuiceFS CSI Node

Before starting JuiceFS CSI Node, you need to configure JuiceFS CSI Node. Add `nodeSelector` in DaemonSet, 
the value is the label that the business pod runs on the node that needs to use JuiceFS. Here, it is assumed that the Label of the business Node is `app: model-training`.

### Kubectl

If you use `kubectl` to install JuiceFS CSI Driver, you need to add `nodeSelector` in `juicefs-csi-node.yaml`, the configuration is as follows:

```yaml {11-12}
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: juicefs-csi-node
  namespace: kube-system
  ...
spec:
  ...
  template:
    spec:
      nodeSelector:
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

Among them, the value of `nodeSelector` needs to be configured according to the actual situation of the node where the business pods run in. 
After the configuration is complete, the JuiceFS CSI Driver can be installed.

### Helm

If you use Helm to install JuiceFS CSI Driver, you need to add `nodeSelector` in `values.yaml`, the configuration is as follows:

```yaml title="values.yaml"
node:
  nodeSelector: 
    app: model-training
```

Install JuiceFS CSI Driver:

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```
