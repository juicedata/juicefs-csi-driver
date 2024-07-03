---
title: Configurations
sidebar_position: 2
---

This chapter introduces JuiceFS PV configurations, as well as CSI Driver configurations.

## ConfigMap {#configmap}

Since CSI Driver v0.24, you can define and adjust settings in a ConfigMap called `juicefs-csi-driver-config`. Various settings are supported to customize mount pod & sidecar container, as well as settings for CSI Driver components. CM is updated dynamically: for mount pod customizations you no longer have to re-create PV & PVCs, and for CSI Driver settings there's no need to restart any CSI Driver components on update.

ConfigMap is powerful and flexible, it will replace (or have already replaced) existing configuration methods that's been around in older versions of CSI Driver, below sections that's titled "deprecated" are all examples of outdated, less flexible methods and should be eschewed. **If something can be configured in ConfigMap, you should always prefer the ConfigMap way, rather than practices available in legacy versions.**

:::info Update delay
When ConfigMap changes, the changes won't take effect immediately, this is because CM mounted in a pod isn't updated in real-time, but synced periodically (see [Kubernetes docs](https://kubernetes.io/docs/concepts/configuration/configmap/#mounted-configmaps-are-updated-automatically)).

If you wish for a force update, try adding a temporary label to CSI components:

```shell
kubectl -n kube-system annotate pods -l app.kubernetes.io/name=juicefs-csi-driver useless-annotation=true
```

:::

All supported fields are demonstrated in the [example config](https://github.com/juicedata/juicefs-csi-driver/blob/master/juicefs-csi-driver-config.example.yaml), and also introduced in detail in our docs.

### Customize mount pod and sidecar container {#customize-mount-pod}

Since mount pods are created by CSI Node, and sidecar containers injected by [webhook](#webhook), users cannot directly control their definition. To customize, refer to the following methods.

The `mountPodPatch` field from the [ConfigMap](#configmap) controls all mount pod & sidecar container customization, all supported fields are demonstrated below, but before use please notice:

* **Changes do not take effect immediately**, Kubernetes periodically syncs ConfigMap mounts, see [update delay](#configmap)
* For sidecar mount mode, if a customization field appears to be a valid sidecar setting, it'll work with sidecar. otherwise it'll be ignored. For example, `custom-labels` adds customized labels to pod, since labels are an exclusive pod attribute, this setting is not applicable to sidecar

```yaml title="values-mycluster.yaml"
globalConfig:
  # Template variables are supported, e.g. ${MOUNT_POINT}、${SUB_PATH}、${VOLUME_ID}
  mountPodPatch:
    # Without a pvcSelector, the patch is global
    - lifecycle:
        preStop:
          exec:
            command:
            - sh
            - -c
            - +e
            - umount -l ${MOUNT_POINT}; rmdir ${MOUNT_POINT}; exit 0

    # If multiple pvcSelector points to the same PVC
    # later items will recursively overwrites the former ones
    - pvcSelector:
        matchLabels:
          mylabel1: "value1"
      # Enable host network
      hostNetwork: true

    - pvcSelector:
        matchLabels:
          mylabel2: "value2"
      # Add labels
      labels:
        custom-labels: "mylabels"

    - pvcSelector:
        matchLabels:
          ...
      # Change resource definition
      resources:
        requests:
          cpu: 100m
          memory: 512Mi

    - pvcSelector:
        matchLabels:
          ...
      readinessProbe:
        exec:
          command:
          - stat
          - ${MOUNT_POINT}/${SUB_PATH}
        failureThreshold: 3
        initialDelaySeconds: 10
        periodSeconds: 5
        successThreshold: 1

    - pvcSelector:
        matchLabels:
          ...
      # For now, avoid actually using liveness probe, and prefer readiness probe instead
      # JuiceFS client carries out its own liveness checks and restarts,
      # giving no reason for additional external liveness checks
      livenessProbe:
        exec:
          command:
          - stat
          - ${MOUNT_POINT}/${SUB_PATH}
        failureThreshold: 3
        initialDelaySeconds: 10
        periodSeconds: 5
        successThreshold: 1

    - pvcSelector:
        matchLabels:
          ...
      annotations:
        # delayed mount pod deletion
        juicefs-delete-delay: 5m
        # clean cache when mount pod exits
        juicefs-clean-cache: "true"
      
```

### Inherit from CSI Node (deprecated) {#inherit-from-csi-node}

:::tip
Starting from v0.24, CSI Driver can customize mount pods and sidecar containers in the [ConfigMap](#configmap), legacy method introduced in this section is not recommended.
:::

Mount pod specs are mostly inherited from CSI Node, for example if you need to enable `hostNetwork` for mount pods, you have to instead add the config to CSI Node:

```yaml title="values-mycluster.yaml"
node:
  hostNetwork: true
```

After the change, newly created mount pods will use hostNetwork.

As mentioned earlier, "most" specs are inherited from CSI-node, this leaves component specific content like labels, annotations, etc. These fields will not work through inheritance so we provide separate methods for customization, read the next section for more.

### Customize via annotations (deprecated) {#others}

:::tip
Starting from v0.24, CSI Driver can customize mount pods and sidecar containers in the [ConfigMap](#configmap), legacy method introduced in this section is not recommended.
:::

Some of the fields that doesn't support CSI Node inheritance, are customized using the following fields in the code block, they can be defined both in storageClass parameters (for dynamic provisioning), and also PVC annotations (static provisioning).

```yaml
juicefs/mount-cpu-limit: ""
juicefs/mount-memory-limit: ""
juicefs/mount-cpu-request: ""
juicefs/mount-memory-request: ""

juicefs/mount-labels: ""
juicefs/mount-annotations: ""
juicefs/mount-service-account: ""
juicefs/mount-image: ""
juicefs/mount-delete-delay: ""

# Clean cache at mount pod exit
juicefs/clean-cache: ""
juicefs/mount-cache-pvc: ""
juicefs/mount-cache-emptydir: ""
juicefs/mount-cache-inline-volume: ""

# Mount the hosts file or directory to pod
# Container mount path will be the same as host path, this doesn't support customization
juicefs/host-path: "/data/file.txt"
```

## Format options / auth options {#format-options}

Format options / auth options are options used in `juicefs [format|auth]` commands, in which:

* The [`format`](https://juicefs.com/docs/community/command_reference/#format) command from JuiceFS CE is used to create a new file system, only then can you mount a file system via the `mount` command;
* The [`auth`](https://juicefs.com/docs/cloud/reference/command_reference/#auth) command from JuiceFS EE authenticates against the web console, and fetch configurations for the client. Its role is somewhat similar to the above `format` command, this due to the differences between the two editions: CE needs to create a file system using our cli, while EE users create file systems directly from the web console, and authenticate later when they need to actually mount the file systems (via the `auth` command).

Considering the similarities between the two commands, options all go to the `format-options` field, as follows.

:::tip
Changing `format-options` does not affect existing mount clients, even if mount pods are restarted. You need to rolling update / re-create the application pods, or re-create PVC for the changes to take effect.
:::

JuiceFS Community Edition:

```yaml {13}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <JUICEFS_NAME>
  metaurl: <META_URL>
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
  format-options: trash-days=1
```

JuiceFS Enterprise Edition:

```yaml {13}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${ACCESS_KEY}
  secret-key: ${SECRET_KEY}
  format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

## Mount options {#mount-options}

Mount options are really just the options supported by the `juicefs mount` command, in CSI Driver, you need to specify them in the `mountOptions` field, which resides in different manifest locations between static provisioning and dynamic provisioning, see below examples.

### Static provisioning {#static-mount-options}

After modifying the mount options for existing PVs, you need to perform a rolling upgrade or re-create the application pod, so that CSI Driver starts re-create the mount pod for the changes to take effect.

```yaml {8-9}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  mountOptions:
    - cache-size=204800
  ...
```

### Dynamic provisioning {#dynamic-mount-options}

Customize mount options in `StorageClass` definition. If you need to use different mount options for different applications, you'll need to create multiple `StorageClass`, each with different mount options.

Due to StorageClass being the template used for creating PVs, **modifying mount options in StorageClass will not affect existing PVs**, if you need to adjust mount options for dynamic provisioning, you'll have to delete existing PVCs, or [directly modify mount options in existing PVs](#static-mount-options).

```yaml {6-7}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
mountOptions:
  - cache-size=204800
parameters:
  ...
```

### Parameter descriptions

Mount options are different between Community Edition and Cloud Service, see:

- [Community Edition](https://juicefs.com/docs/community/command_reference#mount)
- [Cloud Service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount)

`mountOptions` in PV/StorageClass supports both JuiceFS mount options and FUSE options. Keep in mind that although FUSE options is specified using `-o` when using JuiceFS command line, the `-o` is to be omitted inside CSI `mountOptions`, just append each option directly in the YAML list. For a mount command example like below:

```shell
juicefs mount ... --cache-size=204800 -o writeback_cache,debug
```

Translated to CSI `mountOptions`:

```yaml
mountOptions:
  # JuiceFS mount options
  - cache-size=204800
  # Extra FUSE options
  - writeback_cache
  - debug
```

## Share directory among applications {#share-directory}

If you have existing data in JuiceFS, and would like to mount into container for application use, or plan to use a shared directory for multiple applications, here's what you can do:

### Static provisioning

#### Mount subdirectory {#mount-subdirectory}

There are two ways to mount subdirectory, one is through the `--subdir` mount option, the other is through the [`volumeMounts.subPath` property](https://kubernetes.io/docs/concepts/storage/volumes/#using-subpath), which are introduced below.

- **Use the `--subdir` mount option**

  Modify [mount options](#mount-options), specify the subdirectory name using the `subdir` option. CSI Controller will automatically create the directory if not exists.

  ```yaml {8-9}
  apiVersion: v1
  kind: PersistentVolume
  metadata:
    name: juicefs-pv
    labels:
      juicefs-name: ten-pb-fs
  spec:
    mountOptions:
      - subdir=/my/sub/dir
    ...
  ```

- **Use the `volumeMounts.subPath` property**

  ```yaml {11-12}
  apiVersion: v1
  kind: Pod
  metadata:
    name: juicefs-app
    namespace: default
  spec:
    containers:
      - volumeMounts:
          - name: data
            mountPath: /data
            # Note that subPath can only use relative path, not absolute path.
            subPath: my/sub/dir
        ...
    volumes:
      - name: data
        persistentVolumeClaim:
          claimName: juicefs-pvc
  ```

  If multiple application Pods may be running on the same host, and these application Pods need to mount different subdirectories of the same file system, it is recommended to use the `volumeMounts.subPath` property for mounting as this way only 1 Mount Pod will be created, which saves the resources of the host.

#### Sharing the same file system across different namespaces {#sharing-same-file-system-across-namespaces}

If you'd like to share the same file system across different namespaces, use the same set of volume credentials (Secret) in the PV definition:

```yaml {10-12,24-26}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: mypv1
  labels:
    pv-name: mypv1
spec:
  csi:
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
  ...
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: mypv2
  labels:
    pv-name: mypv2
spec:
  csi:
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
  ...
```

### Dynamic provisioning

Strictly speaking, dynamic provisioning doesn't inherently support mounting a existing directory. But you can [configure subdirectory naming pattern (path pattern)](#using-path-pattern), and align the pattern to match with the existing directory name, to achieve the same result.

## Webhook related features {#webhook}

Special options can be added to run CSI Controller as a webhook, so that more advanced features are supported.

### Mutating webhook

If [sidecar mount mode](../introduction.md#sidecar) is used, then Controller also runs as a mutating webhook, and its args will contain [`--webhook`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/templates/controller.yaml#L76), you can use this argument to verify if sidecar mode is enabled.

A murating webhook mutates Kubernetes resources, in our case all pod creation under the specified namespace will go through our webhook, and if JuiceFS PV is used, webhook will inject the corresponding sidecar container.

### Validating webhook

:::tip
This feature only works for JuiceFS Enterprise Edition.
:::

CSI Driver can optionally run secret validation, helping users to correctly fill in their [volume credentials](./pv.md#volume-credentials). If a wrong [volume token](https://juicefs.com/docs/zh/cloud/acl#client-token) is used, the secret fails to create and user is prompted with relevant errors.

To enable validating webhook, write this in your cluster values (refer to our default [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml#L342)):

```yaml name="values-mycluster.yaml"
validatingWebhook:
  enabled: true
```

## Advanced PV provisoning {#provioner}

CSI Driver provides 2 types of PV provisioning:

* Using standard [Kubernetes CSI provisioner](https://github.com/kubernetes-csi/external-provisioner), which is the default mode for older versions of CSI Driver. When running in this mode, juicefs-csi-controller pod includes 4 containers
* (Recommended) Move away from the standard CSI provisioner and use our own in-house controller as provisioner. Since v0.23.4, if CSI Driver is installed via Helm, then this feature is already enabled, and juicefs-csi-controller pod will only have 3 containers

Our in-house provisioner is favored because it opens up a series of new functionalities:

* [Use more readable names for PV directory](#using-path-pattern), instead of subdirectory names like `pvc-4f2e2384-61f2-4045-b4df-fbdabe496c1b`, they can be customized for readability, e.g. `default-juicefs-myapp`
* Use templates in mount options, this allows for advanced features like [regional cache groups](#regional-cache-group)

Under dynamic provisioning, provisioner will create PVs according to its StorageClass settings. Once created, their mount options are fixed (inherited from SC). But if our in-house provisioner is used, mount options can be customized for each PVC.

This feature is disabled by default, to enable, you need to add the `--provisioner=true` option to CSI Controller start command, and delete the sidecar container, so that CSI Controller main process is in charge of watching for resource changes, and carrying out actual provisioning.

:::tip
Advanced provisioning is not supported in [mount by process mode](../introduction.md#by-process).
:::

### Helm

Add below content to `values.yaml`:

```yaml title="values.yaml"
controller:
  provisioner: true
```

Then reinstall JuiceFS CSI Driver:

```shell
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### kubectl

Helm is absolutely recommended since kubectl installation means a lot of complex manual edits. Please migrate to Helm installation as soon as possible.

Manually edit CSI Controller:

```shell
kubectl edit sts -n kube-system juicefs-csi-controller
```

Sections that require modification have been highlighted and annotated below:

```diff
 apiVersion: apps/v1
 kind: StatefulSet
 metadata:
   name: juicefs-csi-controller
   ...
 spec:
   ...
   template:
     ...
     spec:
       containers:
         - name: juicefs-plugin
           image: juicedata/juicefs-csi-driver:v0.17.4
           args:
             - --endpoint=$(CSI_ENDPOINT)
             - --logtostderr
             - --nodeid=$(NODE_NAME)
             - --v=5
+            # Make juicefs-plugin listen for resource changes, and execute provisioning steps
+            - --provisioner=true
         ...
-        # Delete the default csi-provisioner, do not use it to listen for resource changes and provisioning
-        - name: csi-provisioner
-          image: quay.io/k8scsi/csi-provisioner:v1.6.0
-          args:
-            - --csi-address=$(ADDRESS)
-            - --timeout=60s
-            - --v=5
-          env:
-            - name: ADDRESS
-              value: /var/lib/csi/sockets/pluginproxy/csi.sock
-          volumeMounts:
-            - mountPath: /var/lib/csi/sockets/pluginproxy/
-              name: socket-dir
         - name: liveness-probe
           image: quay.io/k8scsi/livenessprobe:v1.1.0
           args:
             - --csi-address=$(ADDRESS)
             - --health-port=$(HEALTH_PORT)
           env:
             - name: ADDRESS
               value: /csi/csi.sock
             - name: HEALTH_PORT
               value: "9909"
           volumeMounts:
             - mountPath: /csi
               name: socket-dir
         ...
```

You can also use a one-liner to achieve above modifications, but note that **this command isn't idempotent and cannot be executed multiple times**:

```shell
kubectl -n kube-system patch sts juicefs-csi-controller \
  --type='json' \
  -p='[{"op": "remove", "path": "/spec/template/spec/containers/1"}, {"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["--endpoint=$(CSI_ENDPOINT)", "--logtostderr", "--nodeid=$(NODE_NAME)", "--v=5", "--provisioner=true"]}]'
```

### Scenarios

#### Set regional cache group {#regional-cache-group}

Using mount option templates, we can customize `cache-group` names for clients scattered in different regions. First, mark the region name in node annotation:

```shell
kubectl annotate --overwrite node minikube myjfs.juicefs.com/cacheGroup=region-1
```

And then modify relevant fields in SC:

```yaml {11-13}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
mountOptions:
  - cache-group="${.node.annotations.myjfs.juicefs.com/cacheGroup}"
# Must use WaitForFirstConsumer, otherwise PV will be provisioned prematurely, injection won't work
volumeBindingMode: WaitForFirstConsumer
```

After PVC & PV is created, verify the injection inside PV:

```bash {8}
$ kubectl get pv pvc-4f2e2384-61f2-4045-b4df-fbdabe496c1b -o yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pvc-4f2e2384-61f2-4045-b4df-fbdabe496c1b
spec:
  mountOptions:
  - cache-group="region-1"
```

#### Use more readable names for PV directory {#using-path-pattern}

Under dynamic provisioning, CSI Driver will create a sub-directory named like `pvc-234bb954-dfa3-4251-9ebe-8727fb3ad6fd`, for every PVC created. And if multiple applications are using CSI Driver, things can get messy quickly:

```shell
$ ls /jfs
pvc-76d2afa7-d1c1-419a-b971-b99da0b2b89c  pvc-a8c59d73-0c27-48ac-ba2c-53de34d31944  pvc-d88a5e2e-7597-467a-bf42-0ed6fa783a6b
...
```

From 0.13.3 and above, JuiceFS CSI Driver supports defining path pattern for the PV directory created in JuiceFS, making them easier to reason about:

```shell
$ ls /jfs
default-dummy-juicefs-pvc  default-example-juicefs-pvc ...
```

:::tip
Under dynamic provisioning, if you need to use a single shared directory across multiple applications, you can configure `pathPattern` so that multiple PVs write to the same JuiceFS sub-directory. However, [static provisioning](#share-directory) is a more simple & straightforward way to achieve shared storage across multiple applications (just use a single PVC among multiple applications), use this if the situation allows.
:::

Define `pathPattern` in StorageClass:

```yaml {11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  pathPattern: "${.pvc.namespace}-${.pvc.name}"
```

### Injection field reference

You can reference any Node / PVC metadata in the pattern, for example:

1. `${.node.name}-${.node.podCIDR}`, inject node `metadata.name` and `spec.podCIDR`, e.g. `minikube-10.244.0.0/24`
1. `${.node.labels.foo}`, inject node label `metadata.labels["foo"]`
1. `${.node.annotations.bar}`, inject node annotation `metadata.annotations["bar"]`
1. `${.PVC.namespace}-${.PVC.name}` results in the directory name being `<pvc-namespace>-<pvc-name>`
1. `${.PVC.labels.foo}` results in the directory name being the `foo` label value
1. `${.PVC.annotations.bar}` results in the PV directory name being the `bar` annotation value

## Common PV settings

### Automatic mount point recovery {#automatic-mount-point-recovery}

JuiceFS CSI Driver supports automatic mount point recovery since v0.10.7, when mount pod run into problems, a simple restart (or re-creation) can bring back JuiceFS mount point, and application pods can continue to work.

:::note
Upon mount point recovery, application pods will not be able to access files previously opened. Please retry in the application and reopen the files to avoid exceptions.
:::

To enable automatic mount point recovery, applications need to [set `mountPropagation` to `HostToContainer` or `Bidirectional`](https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation) in pod `volumeMounts`. In this way, host mount is propagated to the pod, so when mount pod restarts by accident, CSI Driver will bind mount once again when host mount point recovers.

```yaml {12-18}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: juicefs-app-static-deploy
spec:
  ...
  template:
    ...
    spec:
      containers:
        - name: app
          # Required when using Bidirectional
          # securityContext:
          #   privileged: true
          volumeMounts:
            - mountPath: /data
              name: data
              mountPropagation: HostToContainer
          ...
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: juicefs-pvc-static
```

You can also use tools provided by a community developer to automatically add `mountPropagation: HostToContainer` to application container. For details, please refer to [Project Documentation](https://github.com/breuerfelix/juicefs-volume-hook).

### Cache client config {#cache-client-conf}

Starting from v0.23.3, CSI Driver by default caches the configuration file for JuiceFS Client, i.e. [mount configuration for JuiceFS Enterprise Edition](https://juicefs.com/docs/cloud/reference/command_reference/#auth), which has the following benefits:

* &#8203;<Badge type="primary">On-prem</Badge> If JuiceFS Web Console suffers from an outage, or clients undergo network issue, mount pods & sidecar containers can still mount via the cached config and continue to serve

Caching works like this:

1. Users create or update [volume credential](./pv.md#volume-credentials), CSI Controller will watch for changes and immediately run `juicefs auth` to obtain the new config;
1. CSI Controller injects configuration into the secret, saved as the `initconfig` field;
1. When CSI Node creates mount pod or CSI Controller injecting a sidecar container, `initconfig` is mounted into the container;
1. JuiceFS clients within the container run [`juicefs auth`](https://juicefs.com/docs/cloud/reference/command_reference/#auth), since config file is already present inside the container, mount will proceed even if the auth command fails.

If you wish to disable this feature, set [`cacheClientConf`](https://github.com/juicedata/charts/blob/96dafec08cc20a803d870b38dcc859f4084a5251/charts/juicefs-csi-driver/values.yaml#L114-L115) to `false` in your cluster values.

### PV storage capacity {#storage-capacity}

From v0.19.3, JuiceFS CSI Driver supports setting storage capacity under dynamic provisioning (and dynamic provisioning only, static provisioning isn't supported).

In static provisioning, the storage specified in PVC/PV is simply ignored, fill in any a reasonably large size for future-proofing.

```yaml
storageClassName: ""
resources:
  requests:
    storage: 10Ti
```

Under dynamic provisioning, you can specify storage capacity in PVC definition, and it'll be translated into a `juicefs quota` command, which will be executed within CSI Controller, to properly apply the specified capacity quota upon the corresponding subdir. To learn more about `juicefs quota`, check [Community Edition docs](https://juicefs.com/docs/community/command_reference/#quota) and Cloud Service docs (work in progress).

```yaml
...
storageClassName: juicefs-sc
resources:
  requests:
    storage: 100Gi
```

After PV is created and mounted, verify by executing `df -h` command within the application pod:

```bash
$ df -h
Filesystem         Size  Used Avail Use% Mounted on
overlay             84G   66G   18G  80% /
tmpfs               64M     0   64M   0% /dev
JuiceFS:ce-secret  100G     0  100G   0% /data-0
```

### PV expansion {#pv-expansion}

In JuiceFS CSI Driver version 0.21.0 and above, PersistentVolume expansion is supported (only [dynamic provisioning](./pv.md#dynamic-provisioning) is supported). You need to specify `allowVolumeExpansion: true` in [StorageClass](./pv.md#create-storage-class), and specify the Secret to be used when expanding the capacity, which mainly provides authentication information of the file system, for example:

```yaml {9-11}
apiVersion: storage.k8s.io/v1
kind: StorageClass
...
parameters:
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/controller-expand-secret-name: juicefs-secret   # same as provisioner-secret-name
  csi.storage.k8s.io/controller-expand-secret-namespace: default     # same as provisioner-secret-namespace
allowVolumeExpansion: true         # indicates support for expansion
```

Expansion of the PersistentVolume can then be triggered by specifying a larger storage request by editing the PVC's `spec` field:

```yaml {10}
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: myclaim
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi  # Specify a larger size here
```

### Access modes {#access-modes}

JuiceFS PV supports `ReadWriteMany` and `ReadOnlyMany` as access modes, change the `accessModes` field accordingly in above PV/PVC (or `volumeClaimTemplate`) definitions.

### Reclaim policy {#relaim-policy}

Under static provisioning, only `persistentVolumeReclaimPolicy: Retain` is supported, static PVs cannot reclaim data with PV deletion.

Dynamic provisioning supports `Delete|Retain` policies, `Delete` causes data to be deleted with PV release, if data security is a concern, remember to enable the "trash" feature of JuiceFS:

* [JuiceFS Community Edition docs](https://juicefs.com/docs/community/security/trash)
* [JuiceFS Enterprise Edition docs](https://juicefs.com/docs/zh/cloud/trash)

### Mount host's directory in Mount Pod {#mount-host-path}

If you need to mount files or directories into the mount pod, use `juicefs/host-path`, you can specify multiple path (separated by comma) in this field. Also, this field appears in different locations for static / dynamic provisioning, take `/data/file.txt` for an example:

#### Static provisioning

```yaml {17}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  ...
  csi:
    driver: csi.juicefs.com
    volumeHandle: juicefs-pv
    fsType: juicefs
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
    volumeAttributes:
      juicefs/host-path: /data/file.txt
```

#### Dynamic provisioning

```yaml {7}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  juicefs/host-path: /data/file.txt
```

#### Advanced usage

Mount the `/etc/hosts` file into the pod. In some cases, you might need to directly use the node `/etc/hosts` file inside the container (however, [`HostAliases`](https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/) is usually the better approach).

```yaml
juicefs/host-path: "/etc/hosts"
```

If you need to mount multiple files or directories, specify them using comma:

```yaml
juicefs/host-path: "/data/file1.txt,/data/file2.txt,/data/dir1"
```
