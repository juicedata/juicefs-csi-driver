---
slug: /deploy-on-demand
---

# CSI Node Deployed on Demand

JuiceFS CSI Driver consists of CSI Controller, CSI Node and Mount Pod. Refer to [JuiceFS CSI Architecture Document](/csi/introduction) for details.

By default, CSI Node (Kubernetes DaemonSet) will run on all nodes, users may want to start CSI Node only on nodes that really need to use JuiceFS, to further reduce resource usage.

## Configure JuiceFS CSI Node

Running CSI Node on demand can be achieved by simply adding a `nodeSelector` clause in the DaemonSet manifest. Set the value to match the desired node label, assuming that the nodes have already been labeled with `app: model-training`:

```shell
# adjust nodes and label accordingly
kubectl label node [node-1] [node-2] app=model-training
```

### Kubectl

Either edit `juicefs-csi-node.yaml` and run `kubectl apply -f juicefs-csi-node.yaml`, or edit directly using `kubectl -n kube-system edit daemonset juicefs-csi-node`, add the `nodeSelector` part:

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
        # adjust accordingly
        app: model-training
      containers:
      - name: juicefs-plugin
        ...
...
```

### Helm

Add `nodeSelector` in `values.yaml`:

```yaml title="values.yaml"
node:
  nodeSelector:
    app: model-training
```

Install JuiceFS CSI Driver:

```bash
helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```
