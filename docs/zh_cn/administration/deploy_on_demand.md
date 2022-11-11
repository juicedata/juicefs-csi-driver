---
slug: deploy-on-demand
---

# CSI Node 组件按需启动

JuiceFS CSI Driver 的组件分为 CSI Controller、CSI Node 及 Mount Pod，详细可参考[JuiceFS CSI 架构文档](../introduction.md)。
其中 CSI Controller 为 StatefulSet，CSI Node 为 DaemonSet，Mount Pod 为运行 JuiceFS 客户端的 Pod。

默认情况下，CSI Node 会在所有节点上启动，但是在某些场景下，用户可能希望 CSI Node 仅在需要挂载 JuiceFS 文件系统的节点上启动，这样可以减少资源占用，提高集群的可用性。
这篇文档将介绍如何在 Kubernetes 集群中按需启动 CSI Node 组件。

## 配置 JuiceFS CSI Node

在启动 JuiceFS CSI Node 之前，需要先配置 JuiceFS CSI Node，在 DaemonSet 中加入 `nodeSelector`，其值为需要使用 JuiceFS 的业务 pod 运行的节点上具有的特点标签。
这里假设业务 Node 的 Label 为 `app: model-training`。

### Kubectl

若您使用 `kubectl` 来安装 JuiceFS CSI 驱动，需要在 `juicefs-csi-node.yaml` 中加入 `nodeSelector`，配置项如下：

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

其中，`nodeSelector` 的值需要根据业务 pod 运行的节点的实际情况进行配置。配置完即可安装 JuiceFS CSI Driver。

### Helm

若您使用 Helm 来安装 JuiceFS CSI 驱动，可以在 `values.yaml` 中添加如下配置：

```yaml title="values.yaml"
node:
  nodeSelector: 
    app: model-training
```

安装：

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```
