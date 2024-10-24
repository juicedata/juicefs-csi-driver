---
title: Upgrade JuiceFS CSI Driver
slug: /upgrade-csi-driver
sidebar_position: 2
---

If you did not run into problems, there's no need to rush upgrades. But we do recommend that you keep using the latest major & minor versions of CSI Driver (patch versions however, can be skipped if it does not contain the needed improvements). To confirm which version that you're currently using, simply check the image version using the below one-liner:

```shell
kubectl get pods -l app=juicefs-csi-node -ojsonpath='{range .items[*]}{..spec..image}{"\n"}{end}' --all-namespaces | head -n 1 | grep -oP 'juicefs-csi-driver:\S+'
```

Check the [release notes](https://github.com/juicedata/juicefs-csi-driver/releases) to decide if you need to upgrade JuiceFS CSI Driver. Be aware that:

* Upgrading CSI Driver accompanies JuiceFS Client upgrade (i.e. mount image), service won't be affected because existing Mount Pods will remain the same, newer versions take effect after application pod is re-created
* If you already explicitedly defined mount image tag via [other approaches](../guide/custom-image.md), then upgrading CSI Driver no longer affect mount image version
* If you need to solely upgrade JuiceFS Client, refer to [upgrade JuiceFS Client](./upgrade-juicefs-client.md)

## Upgrade CSI Driver (mount by pod mode) {#upgrade}

Since v0.10.0, JuiceFS Client is separated from CSI Driver, upgrading CSI Driver will no longer affect existing PVs, allowing a much easier upgrade process, which doesn't interrupt running applications.

But on the other hand, this also means **upgrading CSI Driver will not automatically apply update to JuiceFS Client for application pods**. You'll have to re-create application pods, so that CSI Node makes Mount Pods using the newer version of Mount Pod image.

Another thing to keep in mind, if you have [overwritten Mount Pod image](../guide/custom-image.md#overwrite-mount-pod-image), then upgrading CSI Driver will not affect JuiceFS Client version at all, you should just continue manage Mount Pod image according to the [docs](../guide/custom-image.md#overwrite-mount-pod-image).

### Upgrade via Helm {#helm-upgrade}

When using Helm to manage CSI Driver installations, all cluster-specific configs goes into a dedicated values file, it's your responsibility to manage this file. Upgrading is simple, just re-install CSI Driver using the latest Helm chart:

```bash
helm repo update

# Assuming the cluster config is saved as values-mycluster.yaml
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
```

### Upgrade via kubectl {#kubectl-upgrade}

If you are to install via kubectl, we advise you against any modifications upon the `k8s.yaml`, as this introduces toil when you carry out upgrades in the future: you'll have to review every difference between versions, and decide what part of change is introduced by the upgrade, and what should be preserved as needed customizations. The toil will grow with more customizations, and should absolutely be avoided on production environments.

So if you're still using kubectl to manage installations, switch to Helm whenever possible. Considering the default mount mode is [a decoupled architecture](../introduction.md#architecture), uninstalling CSI Driver doesn't interfere with existing PVs, so you can switch to Helm installation with ease, and enjoy a much easier maintenance.

Of course, if you haven't made any modifications to the default CSI installation, you can just download the latest [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml), and then perform the upgrade by running:

```shell
kubectl apply -f ./k8s.yaml
```

But if you maintain your own fork of `k8s.yaml`, with all your own customizations, you'll need to compare the differences between the old and new `k8s.yaml`, apply the newly introduced changes, and then install.

```shell
curl https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml > k8s-new.yaml

# Backup the old YAML file
cp k8s.yaml k8s.yaml.bak

# Compare the differences between old and new YAML files, apply the newly introduced changes while maintaining your own modifications
# For example, the CSI component image is usually updated, e.g. image: juicedata/juicefs-csi-driver:v0.21.0
vimdiff k8s.yaml k8s-new.yaml

# After changes have been applied, reinstall
kubectl apply -f ./k8s.yaml
```

If installation raises error stating immutable resources:

```
csidrivers.storage.k8s.io "csi.juicefs.com" was not valid:
* spec.storageCapacity: Invalid value: true: field is immutable
```

This indicates that the new `k8s.yaml` introduces changes on CSI Driver definition (e.g. [v0.21.0](https://github.com/juicedata/juicefs-csi-driver/releases/tag/v0.21.0) introduces `podInfoOnMount: true`), you'll have to delete relevant resources to re-install:

```shell
kubectl delete csidriver csi.juicefs.com

# Re-install
kubectl apply -f ./k8s.yaml
```

Dealing with exceptions like this, alongside with comparing and merging YAML files can be wearisome, that's why [install via Helm](../getting_started.md#helm) is much more recommended on a production environment.

### Migrate to Helm installation {#migrate-to-helm}

Helm installation requires filling in `values.yaml` first - all changes you have made to `k8s.yaml`, as long as they are within the scope of normal use, can find the corresponding configuration fields in `values.yaml`. What you need to do just sort out the current configuration and fill them in `values.yaml`. Of course, if you have not customized `k8s.yaml` (nor directly modified the configuration of the production environment), then the migration will be very simple. Just skip the grooming step and just follow the instructions below to uninstall and reinstall.

#### Sort out the configuration and fill in `values.yaml` {#sort-out-the-configuration-and-fill-in-values-yaml}

Before you start, you need to determine the CSI Driver version you are currently using. You can directly use the method at the beginning of this document to determine. The following takes the upgrade from v0.18.0 to v0.21.0 as an example to explain how to sort out the configuration line by line and fill in `values.yaml`.

1. Use a browser to access GitHub and open the diffs of the two versions. This process requires manually entering the link, pay attention to the version number at the end of the link, for example [https://github.com/juicedata/juicefs-csi-driver/compare/v0.18.0..v0.21.0](https://github.com/juicedata/juicefs-csi-driver/compare/v0.18.0..v0.21.0), found `k8s.yaml` in the file list. All `k8s.yaml` changes introduced by the version update will be displayed on the page. Keep this page, when sorting out the configuration later, if you are not sure which changes are your cluster's customized configuration and which are modifications brought about by the upgrade, you can refer to this page to judge;
1. Find the `k8s.yaml` used in the current online cluster installation, and rename its copy to `k8s-online.yaml`. This document will also use this name to refer to the current online installation file later in this document. It must be noted that the file must accurately reflect the "current online configuration". If your team has temporarily modified the online configuration (such as using `kubectl edit` to temporarily add environment variables and modify the image), you need to confirm these changes and append to `k8s-online.yaml`;
1. Install the new version (the link here takes v0.21.0 as an example) of the CSI Driver [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/94d4f95a5d0f15a7a430ea31257d725306e90ca4/deploy/k8s.yaml) downloaded to the local, and compared with the online configuration, you can directly run `vimdiff k8s.yaml k8s-online.yaml`;
1. Compare the configuration files line by line to determine whether each configuration modification is brought about by the upgrade or customized by your team. Determine whether these customizations need to be retained, and then fill them in `values.yaml`. If you're not sure how to fill it out, you can usually find clues by carefully reading the annotated documentation in `values.yaml`.

We have the following suggestions for writing `values.yaml`:

If the default Mount Pod image is [overridden](../guide/custom-image.md#overwrite-mount-pod-image) in `k8s-online.yaml` (you can pass the `JUICEFS_EE_MOUNT_IMAGE` environment variable, or the `juicefs/mount-image` field of StorageClass ), and an older version of the Mount Pod image is specified, we encourage you to discard this configuration, let the cluster upgrade with the CSI Driver, and enable the new version of the Mount Pod image, which is equivalent to upgrading the JuiceFS client along with the CSI Driver upgrade.

Dynamic provisioning requires [create StorageClass](../guide/pv.md#create-storage-class), while in Helm Values, StorageClass and [volume credentials](../guide/pv.md#volume-credentials) are managed together. In order to avoid leaving sensitive information in `values.yaml`, we generally recommend manually managing the file system authentication information and StorageClass, and then disabling StorageClass in `values.yaml`:

```yaml title="values.yaml"
storageClasses:
- enabled: false
```

#### Uninstall and reinstall {#uninstall-and-reinstall}

If you use the default container mounting or sidecar mode, uninstalling the CSI Driver will not affect the current service (new PVs cannot be created or mounted during this period). Only [process mount mode](../introduction.md#by-process) will interrupt services due to uninstallation. If you are not using this mode, the migration process has no impact on the running PV and can be performed with confidence.

If your environment is an offline cluster and you cannot directly pull the image from the external network, you also need to [move the image](./offline.md) in advance.

Prepare the operation and maintenance commands that need to be run in advance, such as:

```shell
# Uninstall the CSI Driver
kubectl delete -f k8s-online.yaml

# Reinstall with Helm. The configuration of different clusters can be managed using different values.yaml files. For example values-dev.yaml, values-prod.yaml.
# The CSI Driver has no special requirements for the installation namespace. You can modify it as needed, such as jfs-system.
helm upgrade --install juicefs-csi-driver . -f values.yaml -n kube-system
```

Run these commands, and after reinstallation, immediately observe the startup status of each component of the CSI Driver:

```shell
kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
```

Wait for all components to be started, and then simply create an application pod for verification. You can refer to [our demonstration](../guide/pv.md#static-provisioning).

## Upgrade CSI Driver (mount by process mode) {#mount-by-process-upgrade}

[Mount by process](../introduction.md#by-process) means that JuiceFS Client runs inside CSI Node Service Pod, under this mode, upgrading CSI Driver will inevitably interrupt existing mounts, use one of below methods to carry out the upgrade.

Before v0.10.0, JuiceFS CSI Driver only supports mount by process, so if you're still using v0.9.x or earlier versions, follow this section to upgrade as well.

### Method 1: Rolling upgrade

Use this option if applications using JuiceFS cannot be interrupted.

If new resources are introduced in the target version, you'll need to manually create them. All YAML examples listed in this section are only applicable when upgrading from v0.9 to v0.10. Depending on the target version, you might need to compare different versions of `k8s.yaml` files, extract the different Kubernetes resources, and manually install them yourself.

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
