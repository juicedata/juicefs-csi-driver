# How to upgrade JuiceFS CSI Driver from v0.9.0 to v0.10.0

Juicefs CSI Driver separated JuiceFS client from CSI Driver since v0.10.0. But the upgrade from v0.9.0 to v0.10.0 will cause all PVs become unavailable, we can upgrade one by one node to make the upgrade smooth. If your application using JuiceFS volume service can be interrupted, you can choose the method of [upgrading the whole cluster](upgrade-the-whole-cluster).

## Upgrade one by one node

### 1. Create resources added in new verison

Replace namespace of YAML below and save it as `csi_new_resource.yaml`, and then apply with `kubectl apply -f csi_new_resource.yaml`.

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: juicefs-mount-critical
value: 1000000000
description: "Juicefs mount pod priority, should not be preempted."
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: juicefs-csi-external-node-service-role
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "latest"
rules:
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
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - get
      - create
      - delete
      - update
      - patch
      - list
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "latest"
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
  namespace: <namespace>
  labels:
    app.kubernetes.io/name: juicefs-csi-driver
    app.kubernetes.io/instance: juicefs-csi-driver
    app.kubernetes.io/version: "latest"
```

### 2. Update node service DaemonSet `updateStrategy` to `OnDelete`

```shell
kubectl -n <namespace> patch ds <ds_name> -p '{"spec": {"updateStrategy": {"type": "OnDelete"}}}'
```

### 3. Update CSI Driver node service DaemonSet

Save YAML below as `ds_patch.yaml`, and then apply with `kubectl -n <namespace> patch ds <ds_name> --patch "$(cat ds_patch.yaml)"`.

```yaml
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:latest
          args:
            - --endpoint=$(CSI_ENDPOINT)
            - --logtostderr
            - --nodeid=$(NODE_ID)
            - --v=5
            - --enable-manager=true
          env:
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
            - name: JUICEFS_MOUNT_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
            - name: JUICEFS_MOUNT_IMAGE
              value: juicedata/juicefs-csi-driver:latest
            - name: JUICEFS_MOUNT_PATH
              value: /var/lib/juicefs/volume
            - name: JUICEFS_CONFIG_PATH
              value: /var/lib/juicefs/config
            - name: JUICEFS_MOUNT_PRIORITY_NAME
              value: 'juicefs-mount-critical'
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

### 4. Upgrade node service pod one by one node

Do operations below per node:

1. Delete CSI Driver pod in one node:

```shell
kubectl -n kube-system delete po juicefs-csi-node-df7m7
```

2. Verify new node service pod is ready (suppose the host name of node is `kube-node-2`):

```shell
$ kubectl -n kube-system get po -o wide -l app.kubernetes.io/name=juicefs-csi-driver | grep kube-node-2
juicefs-csi-node-6bgc6     3/3     Running   0          60s   172.16.11.11   kube-node-2   <none>           <none>
```

3. In this node, delete the pods mounting JuiceFS volume and recreate them.

4. Verify if the pods mounting JuiceFS volume is ready, and check them if works well.

### 5. Upgrade CSI Driver controller service

Save YAML below as `sts_patch.yaml`, and then apply with `kubectl -n <namespace> patch sts <sts_name> --patch "$(cat sts_patch.yaml)"`.

```yaml
spec:
  template:
    spec:
      containers:
        - name: juicefs-plugin
          image: juicedata/juicefs-csi-driver:latest
```

## Upgrade the whole cluster

### 1. Stop all the applications using JuiceFS volume

### 2. Upgrade CSI driver

* If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make sure `juicefs-csi-controller` and `juicefs-csi-node` pods are restarted.
* If you have pinned to a specific version, modify your `k8s.yaml` to a newer version, then run `kubectl apply -f k8s.yaml`.
* Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.

### 3. Start all the applications using JuiceFS volume
