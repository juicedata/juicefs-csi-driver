---
slug: deploy-on-demand
---

# CSI Node 组件按需启动

JuiceFS CSI Driver 的组件分为 CSI Controller、CSI Node 及 Mount Pod，详细可参考[JuiceFS CSI 架构文档](../introduction.md)。

默认情况下，CSI Node（Kubernetes DaemonSet）会在所有节点上启动，用户可能希望 CSI Node 仅在实际需要使用 JuiceFS，进一步减少资源占用。

## 配置 JuiceFS CSI Node

配置按需启动很简单，仅需在 DaemonSet 中加入 `nodeSelector`，指向实际需要使用 JuiceFS 的节点，假设需要的 Node 都已经打上了该 Label：`app: model-training`。

```shell
# 根据实际情况修改 nodes 和 label
kubectl label node [node-1] [node-2] app=model-training
```

### Kubectl

修改 `juicefs-csi-node.yaml` 然后运行 `kubectl apply -f juicefs-csi.node.yaml`，或者直接 `kubectl -n kube-system edit daemonset juicefs-csi-node`，加入 `nodeSelector` 配置：

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
        # 根据实际情况修改
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

### Helm

在 `values.yaml` 中添加如下配置：

```yaml title="values.yaml"
node:
  nodeSelector:
    app: model-training
```

安装：

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```
