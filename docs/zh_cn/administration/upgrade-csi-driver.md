---
title: 升级 JuiceFS CSI 驱动
slug: /upgrade-csi-driver
sidebar_position: 2
---

阅读 JuiceFS CSI 驱动的[发布说明](https://github.com/juicedata/juicefs-csi-driver/releases)以了解是否需要升级。如果你希望单独升级 JuiceFS 客户端，参考[「升级 JuiceFS 客户端」](./upgrade-juicefs-client.md)。

## 升级 CSI 驱动（容器挂载模式） {#upgrade}

v0.10.0 开始，JuiceFS 客户端与 CSI 驱动进行了分离，升级 CSI 驱动将不会影响已存在的 PV。因此升级过程大幅简化，同时不影响已有服务。

但这也意味着，升级 CSI 驱动后，**应用并不会自动地享受到新版 JuiceFS 客户端**。你还需要重新创建应用容器，这样一来，CSI Node 会使用新版 Mount Pod 容器镜像创建挂载点，让应用使用新版 JuiceFS 客户端。

特别地，如果你[修改了 Mount Pod 容器镜像](../guide/custom-image.md#overwrite-mount-pod-image)，那么升级 CSI 驱动就完全不影响 JuiceFS 客户端版本了，你需要按照[文档](../guide/custom-image.md#overwrite-mount-pod-image)，继续自行管理 Mount Pod 容器镜像。

### 通过 Helm 升级 {#helm-upgrade}

用 Helm 安装 CSI 驱动时，所有的安装配置都汇集于 `values.yaml`，你需要自行管理该文件。升级的步骤也很简单，直接用新版的 Helm chart 重装 CSI 驱动即可：

```shell
helm repo update
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

如果你已经将整个 Helm chart 纳入版本控制系统管理，则需要用 `helm pull --untar juicefs/juicefs-csi-driver` 下载最新版本的 CSI 驱动 Helm chart，覆盖当前版本。注意，`values.yaml` 的维护是你的责任，升级过程中请注意不要覆盖该文件，否则安装配置将会被重置为默认状态。

### 通过 kubectl 升级 {#kubectl-upgrade}

如果你并未对 CSI 驱动配置做任何改动，那么直接下载最新的 [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml)，然后用下边的命令进行覆盖安装即可。

```shell
kubectl apply -f ./k8s.yaml
```

但如果你的团队有着自行维护的 `k8s.yaml`，并且已经对其中的配置做了变更，那就需要对新老版本的 `k8s.yaml` 进行内容比对，将新版引入的变动追加进来，然后再进行覆盖安装：

```shell
curl https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml > k8s-new.yaml

# 梳理配置前，对老版本安装文件进行备份
cp k8s.yaml k8s.yaml.bak

# 对比新老版本的内容差异，在保留配置变更的基础上，将新版引入的变动进行追加
# 比方说，新版的 CSI 驱动组件镜像往往会更新，例如 image: juicedata/juicefs-csi-driver:v0.17.5
vimdiff k8s.yaml k8s-new.yaml

# 配置梳理完毕，进行覆盖安装
kubectl apply -f ./k8s.yaml
```

正因为梳理配置的步骤相对复杂，因此面对生产集群，我们更推荐[使用 Helm 安装和升级 CSI 驱动](../getting_started.md#helm)。

#### v0.21.0 版本升级注意事项 {#v0-21-0}

在 JuiceFS CSI 驱动 v0.21.0 版本中，我们在 CSIDriver 资源中引入了 `podInfoOnMount: true`，但 CSIDriver 资源不能更新，升级前需要手动删除旧版本的 CSIDriver 资源，否则会导致升级失败：

```shell
kubectl delete csidriver csi.juicefs.com
```

再参考 [通过 kubectl 升级](#kubectl-upgrade) 进行升级。

若使用的是 helm 安装，可以直接升级 chart 升级，无需该步骤。

## 升级 CSI 驱动（进程挂载模式） {#mount-by-process-upgrade}

所谓[进程挂载模式](../introduction.md#by-process)，就是 JuiceFS 客户端运行在 CSI Node Service Pod 内，此模式下升级，将不可避免地需要中断 JuiceFS 客户端挂载，你需要根据实际情况，参考以下方案进行升级。

v0.10 之前的 JuiceFS CSI 驱动，仅支持进程挂载模式，因此如果你还在使用 v0.9.x 或更早的版本，也需要参考本节内容进行升级。

### 方案一：滚动升级

如果使用 JuiceFS 的应用不可被中断，可以采用此方案。

由于是滚动升级，因此目标版本如果引入了新资源，你需要提前梳理出来，并单独安装。本小节中包含的各种 YAML 内容，仅适用于自 v0.9 升级至 v0.10 的情况。视目标版本不同，你可能需要比较不同版本的 `k8s.yaml` 文件，提取出有差异的 Kubernetes 资源，并手动安装。

#### 1. 创建新增资源

将以下内容保存成 `csi_new_resource.yaml`，然后执行 `kubectl apply -f csi_new_resource.yaml`。

```yaml title="csi_new_resource.yaml"
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

#### 2. 将 CSI Node Service 的升级策略改成 `OnDelete`

```shell
kubectl -n kube-system patch ds <ds_name> -p '{"spec": {"updateStrategy": {"type": "OnDelete"}}}'
```

#### 3. 升级 CSI Node Service

将以下内容保存成 `ds_patch.yaml`，然后执行 `kubectl -n kube-system patch ds <ds_name> --patch "$(cat ds_patch.yaml)"`。

```yaml title="ds_patch.yaml"
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

#### 4. 执行滚动升级

在每台节点上执行以下操作：

1. 删除当前节点上的 CSI Node Service pod：

   ```shell
   kubectl -n kube-system delete po juicefs-csi-node-df7m7
   ```

2. 确认新的 CSI Node Service pod 已经 ready：

   ```shell
   $ kubectl -n kube-system get po -o wide -l app.kubernetes.io/name=juicefs-csi-driver | grep kube-node-2
   juicefs-csi-node-6bgc6     3/3     Running   0          60s   172.16.11.11   kube-node-2   <none>           <none>
   ```

3. 在当前节点上，删除使用 JuiceFS 的业务 pod 并重新创建。

4. 确认使用 JuiceFS 的业务 pod 已经 ready，并检查是否正常工作。

#### 5. 升级 CSI Controller 及其 role

将以下内容保存成 `sts_patch.yaml`，然后执行 `kubectl -n kube-system patch sts <sts_name> --patch "$(cat sts_patch.yaml)"`。

```yaml title="sts_patch.yaml"
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

将以下内容保存成 `clusterrole_patch.yaml`，然后执行 `kubectl patch clusterrole <role_name> --patch "$(cat clusterrole_patch.yaml)"`。

```yaml title="clusterrole_patch.yaml"
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

### 方案二：替换升级

如果你能接受服务中断，这将是更为简单的升级手段。

此法需要卸载所有正在使用 JuiceFS PV 的应用，按照上方的[正常升级流程](#upgrade)操作，然后重新创建受影响的应用即可。
