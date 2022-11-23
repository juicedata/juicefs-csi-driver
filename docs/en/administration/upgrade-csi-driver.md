---
title: Upgrade JuiceFS CSI Driver
slug: /upgrade-csi-driver
---

Check the [release notes](https://github.com/juicedata/juicefs-csi-driver/releases) to decide if you need to upgrade JuiceFS CSI Driver.

:::note
If you are using CSI Driver in [Mount by process mode](../introduction.md#by-process), or still using versions before v0.10, refer to [Upgrade under mount by process mode](#mount-by-process-upgrade) for upgrade steps.
:::

## Upgrade CSI Driver {#upgrade}

Since v0.10.0, JuiceFS Client is separated from CSI Driver, upgrading CSI Driver will no longer affect existing PVs, allowing a much easier upgrade process, which doesn't interrupt running applications.

### Upgrade via Helm

Run below commands:

```bash
helm repo update
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### Upgrade via kubectl

Reinstall JuiceFS CSI Driver using the latest [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml), if you maintain your own fork of `k8s.yaml`, simply modify the image label to the latest version (like `image: juicedata/juicefs-csi-driver:v0.17.2`), and then run:

```shell
kubectl apply -f ./k8s.yaml
```

## Upgrade under mount by process mode {#mount-by-process-upgrade}

[Mount by process](../introduction.md#by-process) means that JuiceFS Client runs inside CSI Node Service Pod, under this mode, upgrading CSI Driver will inevitably interrupt existing mounts, use one of below methods to carry out the upgrade.

Before v0.10.0, JuiceFS CSI Driver only supports mount by process, so if you're still using v0.9.x or earlier versions, follow this section to upgrade as well.

### Method 1: Rolling upgrade

Use this option if applications using JuiceFS cannot be interrupted.

If new resources are introduced in the target version, you'll need to manually create them. All YAML examples listed in this section are only applicable when upgrading from v0.9 to v0.10. Depending on the target version, you might need to compare different versions of `k8s.yaml` files, extract the different Kuberenetes resources, and manually install them yourself.

#### 1. Create resources added in new version

Save below content as `csi_new_resource.yaml`, and then run `kubectl apply -f csi_new_resource.yaml`.

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

#### 2. Change the upgrade strategy of CSI Node Service to `OnDelete`

```shell
kubectl -n kube-system patch ds <ds_name> -p '{"spec": {"updateStrategy": {"type": "OnDelete"}}}'
```

#### 3. Upgrade the CSI Node Service

Save below content as `ds_patch.yaml`, and then run `kubectl -n kube-system patch ds <ds_name> --patch "$(cat ds_patch.yaml)"`.

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

#### 4. Execute rolling upgrade

Do the following on each node:

1. Delete the CSI Node Service pod on the current node:

   ```shell
   kubectl -n kube-system delete po juicefs-csi-node-df7m7
   ```

2. Verify the re-created CSI Node Service pod is ready:

   ```shell
   $ kubectl -n kube-system get po -o wide -l app.kubernetes.io/name=juicefs-csi-driver | grep kube-node-2
   juicefs-csi-node-6bgc6     3/3     Running   0          60s   172.16.11.11   kube-node-2   <none>           <none>
   ```

3. Delete pods using JuiceFS PV and recreate them.

4. Verify that the application pods are re-created and working correctly.

#### 5. Upgrade CSI Controller and its role

Save below content as `sts_patch.yaml`, and run `kubectl -n kube-system patch sts <sts_name> --patch "$(cat sts_patch.yaml)"`.

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

Save below content as `clusterrole_patch.yaml`, and then run `kubectl patch clusterrole <role_name> --patch "$(cat clusterrole_patch.yaml)"`.

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

### Method 2: Recreate and upgrade

If applications using JuiceFS are allowed to be suspended, this no doubt is the simpler way to upgrade.

To recreate and upgrade, first stop all applications using JuiceFS PV, and carry out [normal upgrade steps](#upgrade), and recreate all affected applications.

## Upgrade JuiceFS Client Independently

If you're using [Mount by process mode](../introduction.md#by-process), or using CSI Driver prior to v0.10.0, and cannot easily upgrade to v0.10, you can choose to upgrade JuiceFS Client independently, inside the CSI Node Service pod.

This is only a temporary solution, if DaemonSet pods are recreated, or new nodes are added to Kubernetes cluster, you'll need to run this script again.

1. Use this script to replace the `juicefs` binary in `juicefs-csi-node` pod with the new built one:

   ```bash
   #!/bin/bash

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

   :::note
   Replace `/path/to/kubectl` and `/path/to/new/juicefs` in the script with the actual values, then execute the script.
   :::

2. Restart the applications one by one, or kill the existing pods.
