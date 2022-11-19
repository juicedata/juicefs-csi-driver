---
title: 升级 JuiceFS CSI 驱动
slug: /upgrade-csi-driver
---

查看 JuiceFS CSI 驱动的[发布说明](https://github.com/juicedata/juicefs-csi-driver/releases)页面了解所有已发布版本的信息。

:::note
如果你以[「进程挂载模式」](../introduction.md#by-process)使用 CSI 驱动，或者还在使用 v0.10 之前的版本，则需参照[「升级至 v0.10」](#upgrade-from-legacy)的步骤进行操作。
:::

## 升级 CSI 驱动

v0.10.0 开始，JuiceFS 客户端与 CSI 驱动进行了分离，升级 CSI 驱动将不会影响已存在的 PV。因此升级过程大幅简化，同时不影响已有服务。

### 通过 Helm 升级

请依次运行以下命令以升级 JuiceFS CSI 驱动：

```shell
helm repo update
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### 通过 kubectl 升级

重新安装 JuiceFS CSI 驱动的 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml)，或者如果你的团队有着自行维护的 `k8s.yaml`，也可以直接修改其中的 JuiceFS CSI 驱动组件的镜像标签（例如 `image: juicedata/juicefs-csi-driver:v0.17.2`）。

运行以下命令执行升级：

```shell
kubectl apply -f ./k8s.yaml
```

## 进程挂载模式下升级 {#upgrade-from-legacy}

所谓[进程挂载模式](../introduction.md#by-process)，就是 JuiceFS 客户端运行在 CSI Node Service Pod 内，此模式下升级，将不可避免地需要中断 JuiceFS 客户端挂载，你需要根据实际情况，参考以下方案进行升级。

v0.10 之前的 JuiceFS CSI 驱动，仅支持进程挂载模式，因此如果你还在使用 v0.9.x 或更早的版本，也需要参考本节内容进行升级。

### 方案一：滚动升级

如果使用 JuiceFS 的应用不可被中断，可以采用此方案。

#### 1. 创建新版本新增的资源

将以下的 YAML 文件保存成 `csi_new_resource.yaml`，然后执行 `kubectl apply -f csi_new_resource.yaml`。

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: juicefs-csi-external-node-service-role
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - create
      - update
      - delete
      - patch
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
  name: juicefs-csi-node-service-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: juicefs-csi-external-node-service-role
subjects:
  - kind: ServiceAccount
    name: juicefs-csi-node-sa
    namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: juicefs-csi-node-sa
  namespace: kube-system
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
```

#### 2. 将 node service DaemonSet 的升级策略改成 OnDelete

```shell
kubectl -n kube-system patch ds <ds_name> -p '{"spec": {"updateStrategy": {"type": "OnDelete"}}}'
```

#### 3. 升级 CSI 驱动的 node service DaemonSet

将以下的 YAML 文件保存成 `ds_patch.yaml`，然后执行 `kubectl -n kube-system patch ds <ds_name> --patch "$(cat ds_patch.yaml)"`。

```yaml
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:v0.10.6
          args:
            - --endpoint=$(CSI_ENDPOINT)
            - --logtostderr
            - --nodeid=$(NODE_ID)
            - --v=5
            - --enable-manager=true
          env:
            - name: CSI_ENDPOINT
              value: unix:/csi/csi.sock
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
          volumeMounts:
            - mountPath: /jfs
              mountPropagation: Bidirectional
              name: jfs-dir
            - mountPath: /root/.juicefs
              mountPropagation: Bidirectional
              name: jfs-root-dir
      serviceAccount: juicefs-csi-node-sa
      volumes:
        - hostPath:
            path: /var/lib/juicefs/volume
            type: DirectoryOrCreate
          name: jfs-dir
        - hostPath:
            path: /var/lib/juicefs/config
            type: DirectoryOrCreate
          name: jfs-root-dir
```

#### 4. 逐台升级 node service

在每台节点上执行以下操作：

1. 删除当前节点上的 CSI Driver pod：

```shell
kubectl -n kube-system delete po juicefs-csi-node-df7m7
```

2. 确认新的 node service pod 已经 ready（假设节点名为 `kube-node-2`）：

```shell
$ kubectl -n kube-system get po -o wide -l app.kubernetes.io/name=juicefs-csi-driver | grep kube-node-2
juicefs-csi-node-6bgc6     3/3     Running   0          60s   172.16.11.11   kube-node-2   <none>           <none>
```

3. 在当前节点上，删除使用 JuiceFS 的业务 pod 并重新创建。

4. 确认使用 JuiceFS 的业务 pod 已经 ready，并检查是否正常工作。

#### 5. 升级 CSI Driver controller 服务及其 role

将以下的 YAML 文件保存成 `sts_patch.yaml`，然后执行 `kubectl -n kube-system patch sts <sts_name> --patch "$(cat sts_patch.yaml)"`。

```yaml
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:v0.10.6
          env:
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
          volumeMounts:
            - mountPath: /jfs
              mountPropagation: Bidirectional
              name: jfs-dir
            - mountPath: /root/.juicefs
              mountPropagation: Bidirectional
              name: jfs-root-dir
      volumes:
        - hostPath:
            path: /var/lib/juicefs/volume
            type: DirectoryOrCreate
          name: jfs-dir
        - hostPath:
            path: /var/lib/juicefs/config
            type: DirectoryOrCreate
          name: jfs-root-dir
```

将以下的 YAML 文件保存成 `clusterrole_patch.yaml`，然后执行 `kubectl patch clusterrole <role_name> --patch "$(cat clusterrole_patch.yaml)"`。

```yaml
rules:
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
    verbs:
      - get
      - list
      - watch
      - create
      - delete
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - storage.k8s.io
    resources:
      - storageclasses
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - csinodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
```

### 方案二：整体升级

如果你能接受服务中断，这将是更为简单的升级手段。

在进行下方操作前，需要先卸载所有正在使用 JuiceFS CSI 驱动的应用。

#### 1. 创建新版本新增的资源

将以下的 YAML 文件保存成 `csi_new_resource.yaml`，然后执行 `kubectl apply -f csi_new_resource.yaml"`。

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: juicefs-csi-external-node-service-role
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
rules:
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - create
      - update
      - delete
      - patch
      - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
  name: juicefs-csi-node-service-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: juicefs-csi-external-node-service-role
subjects:
  - kind: ServiceAccount
    name: juicefs-csi-node-sa
    namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: juicefs-csi-node-sa
  namespace: kube-system
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "v0.10.6"
```

#### 2. 升级 CSI Driver node service DaemonSet

将以下的 YAML 文件保存成 `ds_patch.yaml`，然后执行 `kubectl -n kube-system patch ds <ds_name> --patch "$(cat ds_patch.yaml)"`。

```yaml
spec:
  updateStrategy:
    type: RollingUpdate
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:v0.10.6
          args:
            - --endpoint=$(CSI_ENDPOINT)
            - --logtostderr
            - --nodeid=$(NODE_ID)
            - --v=5
            - --enable-manager=true
          env:
            - name: CSI_ENDPOINT
              value: unix:/csi/csi.sock
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
          volumeMounts:
            - mountPath: /jfs
              mountPropagation: Bidirectional
              name: jfs-dir
            - mountPath: /root/.juicefs
              mountPropagation: Bidirectional
              name: jfs-root-dir
      serviceAccount: juicefs-csi-node-sa
      volumes:
        - hostPath:
            path: /var/lib/juicefs/volume
            type: DirectoryOrCreate
          name: jfs-dir
        - hostPath:
            path: /var/lib/juicefs/config
            type: DirectoryOrCreate
          name: jfs-root-dir
```

确认所有 `juicefs-csi-node-***` pods 都升级了。

#### 3. 升级 CSI Driver controller 服务及其 role

将以下的 YAML 文件保存成 `sts_patch.yaml`，然后执行 `kubectl -n kube-system patch sts <sts_name> --patch "$(cat sts_patch.yaml)"`。

```yaml
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:v0.10.6
          env:
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
          volumeMounts:
            - mountPath: /jfs
              mountPropagation: Bidirectional
              name: jfs-dir
            - mountPath: /root/.juicefs
              mountPropagation: Bidirectional
              name: jfs-root-dir
      volumes:
        - hostPath:
            path: /var/lib/juicefs/volume
            type: DirectoryOrCreate
          name: jfs-dir
        - hostPath:
            path: /var/lib/juicefs/config
            type: DirectoryOrCreate
          name: jfs-root-dir
```

将以下的 YAML 文件保存成 `clusterrole_patch.yaml`，然后执行 `kubectl patch clusterrole <role_name> --patch "$(cat clusterrole_patch.yaml)"`。

```yaml
rules:
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
    verbs:
      - get
      - list
      - watch
      - create
      - delete
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - storage.k8s.io
    resources:
      - storageclasses
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
  - apiGroups:
      - storage.k8s.io
    resources:
      - csinodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
      - delete
```

确认 `juicefs-csi-controller-***` pod 已经升级.

另外，如果 JuiceFS CSI Driver 是使用 Helm 安装的，也可以使用 Helm 来升级。

操作完成以后，重新启动所有使用 JuiceFS 的应用。

## 独立升级 JuiceFS 客户端 {#upgrade-juicefs-client}

如果你在使用进程挂载模式，或者仅仅是难以升级到 v0.10 之后的版本，但又需要使用新版 JuiceFS 进行挂载，那么也可以通过以下方法，在不升级 CSI 驱动的前提下，单独升级 JuiceFS 客户端。

1. 使用以下脚本将 `juicefs-csi-node` pod 中的 `juicefs` 客户端替换为新版：

   ```bash
   #!/bin/bash

   # 运行前请替换为正确路径
   KUBECTL=/path/to/kubectl
   JUICEFS_BIN=/path/to/new/juicefs

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system cp $JUICEFS_BIN '{}':/tmp/juicefs -c juicefs-plugin

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system exec -i '{}' -c juicefs-plugin -- \
       chmod a+x /tmp/juicefs && mv /tmp/juicefs /bin/juicefs
   ```

2. 将应用逐个重新启动，或 kill 掉已存在的 pod。
