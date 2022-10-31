---
slug: deploy-on-demand
---

# JuiceFS CSI Node Deploy On Demand

JuiceFS CSI Driver v0.18.0 supports deploying CSI Node components on demand. This feature is disabled by default.

When this feature is enabled, the JuiceFS CSI Node component Pod will not be installed by default in your cluster. It will only be deployed on the node when there is an application Pod using JuiceFS CSI Volume.

## Usage

### Enable before installation

Add `nodeSelector` to the DaemonSet named `juicefs-csi-node` in the installation Yaml. At the same time, in order to improve the efficiency of mounting, you can replace the image of the CSI Node with slim image, as shown below:

```yaml {9-10,13}
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: juicefs-csi-node
  namespace: kube-system
spec:
  template:
    spec:
      nodeSelector:
        app: juicefs-csi-node
    containers:
    - name: juicefs-csi-node
      image: juicedata/juicefs-csi-driver:<csi-version>-slim   // csi-version is the current CSI driver version
...
```

If you use Helm to install, you can add the following configuration to `values.yaml` to enable this feature:

```yaml title="values.yaml"
node:
  nodeSelector:
    app: juicefs-csi-node
image:
  tag: "<csi-version>-slim"    // csi-version is the current CSI driver version
```

### Enable after installation

If you have already installed JuiceFS CSI Driver, you can use the `kubectl patch` command to add `nodeSelector` to enable this feature:

```bash
kubectl patch daemonset juicefs-csi-node -n kube-system --type=json -p='[{"op": "add", "path": "/spec/template/spec/nodeSelector", "value": {"app": "juicefs-csi-node"}}]'
```

If JuiceFS CSI Driver has already been installed, the JuiceFS CSI Node component Pod will be deleted after enabling this feature.
The JuiceFS CSI Node component Pod will be installed on the node when there is an application Pod using JuiceFS Volume.

## Attention

After enabling this feature, when JuiceFS CSI Driver is uninstalled, the nodes (the nodes that has used JuiceFS Volume) in the cluster will remain JuiceFS label. You need to delete it manually.
