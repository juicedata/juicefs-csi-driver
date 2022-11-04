---
slug: deploy-on-demand
---

# JuiceFS CSI Node 组件按需启动

JuiceFS CSI Driver v0.18.0 开始支持 CSI Node 组件的按需启动，该功能默认关闭。

开启该功能时，您的集群中将默认不安装 JuiceFS CSI Node 组件的 Pod，只有当有应用 Pod 使用 JuiceFS CSI Volume 时，对应节点的
JuiceFS CSI Node 组件才会被启动。

## 使用方法

### 安装时开启

在安装 Yaml 中的名为 `juicefs-csi-node` 的 DaemonSet 中添加 `nodeSelector`。同时，为了提高使用时挂载的效率，可以将 CSI Node 的镜像换成 slim 镜像，如下所示：

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
    - name: juicefs-plugin
      image: juicedata/juicefs-csi-driver:<csi-version>-slim   // csi-version 为当前 CSI 驱动版本
...
```

若您使用 Helm 安装，可以在 `values.yaml` 中添加如下配置以开启该特性：

```yaml title="values.yaml"
node:
  nodeSelector:
    app: juicefs-csi-node
image:
  tag: "<csi-version>-slim"    // csi-version 为当前 CSI 驱动版本
```

### 安装后开启

若您已安装 JuiceFS CSI Driver，可以通过 `kubectl patch` 命令添加 `nodeSelector` 以开启该特性：

```bash
kubectl patch daemonset juicefs-csi-node -n kube-system --type=json -p='[{"op": "add", "path": "/spec/template/spec/nodeSelector", "value": {"app": "juicefs-csi-node"}}]'
```

若已经安装了 JuiceFS CSI Driver，再开启该特性，已经安装的 JuiceFS CSI Node 组件 Pod 将会被删除，下一次该节点上有应用 Pod 使用
JuiceFS Volume 时，JuiceFS CSI Node 组件才会被安装。

## 注意

启用该特性后，卸载 JuiceFS CSI Driver 时，集群中的节点（已经使用过 JuiceFS Volume 的节点）会遗留 JuiceFS 的 label，您需要手动删除。
