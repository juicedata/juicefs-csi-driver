---
title: Configurations
sidebar_position: 2
---

This chapter introduces JuiceFS PV configurations, as well as CSI Driver configurations.

## ConfigMap {#configmap}

Since CSI Driver v0.24, you can define and adjust settings in a ConfigMap called `juicefs-csi-driver-config`. Various settings are supported to customize Mount Pod & sidecar container, as well as settings for CSI Driver components. CM is updated dynamically: for Mount Pod customizations you no longer have to re-create PV & PVCs, and for CSI Driver settings there's no need to restart any CSI Driver components on update.

ConfigMap is powerful and flexible. It will replace (or have already replaced) existing configuration methods in older versions of CSI Driver.  Sections labeled "deprecated" provide examples of these outdated and less flexible approaches, which are no longer recommended. **If a setting is configurable via ConfigMap, it will take the highest priority within the ConfigMap. It is recommended to always use the ConfigMap method over any practices from legacy versions.**

:::info Update delay
When ConfigMap is updated, changes do not take effect immediately, because CM mounted in a Pod is not updated in real time; instead, it is synced periodically (see [Kubernetes docs](https://kubernetes.io/docs/concepts/configuration/configmap/#mounted-configmaps-are-updated-automatically)).

If you wish for a force update, try adding a temporary label to CSI components:

```shell
kubectl -n kube-system annotate pods -l app.kubernetes.io/name=juicefs-csi-driver useless-annotation=true
```

After ConfigMap is updated across CSI components, subsequent Mount Pods will apply the new configuration, but **existing Mount Pods will not automatically update**. Depending on what was changed, users must re-create the application Pod or the Mount Pod for the changes to take effect. Refer to the sections below for more details.
:::

:::info Sidecar headsup
If a customization item appears to be a valid sidecar setting, it will work for the sidecar; otherwise, it will be ignored. For example:

* `resources` applies to both the Mount Pod and the sidecar, so it works for both.
* `custom-labels` adds customized labels to the Pod. However, since labels are an exclusive Pod attribute, this setting does not apply to the sidecar.
:::

All supported fields are demonstrated in the [example configuration](https://github.com/juicedata/juicefs-csi-driver/blob/master/juicefs-csi-driver-config.example.yaml) and are explained in detail in our documentation.

<details>

<summary>Examples</summary>

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
        # Delayed Mount Pod deletion
        juicefs-delete-delay: 5m
        # Clean cache when Mount Pod exits
        juicefs-clean-cache: "true"

    # Define an environment variable for the Mount Pod
    - pvcSelector:
        matchLabels:
          ...
      env:
      - name: DEMO_GREETING
        value: "Hello from the environment"
      - name: DEMO_FAREWELL
        value: "Such a sweet sorrow"

    # Mount some volumes to Mount Pod
    - pvcSelector:
        matchLabels:
          ...
      volumeDevices:
        - name: block-devices
          devicePath: /dev/sda1
      volumes:
        - name: block-devices
          persistentVolumeClaim:
            claimName: block-pv

    # Select by StorageClass
    - pvcSelector:
        matchStorageClassName: juicefs-sc
      terminationGracePeriodSeconds: 60

    # Select by PVC
    - pvcSelector:
        matchName: pvc-name
      terminationGracePeriodSeconds: 60
```

</details>

## Customize Mount Pod and Sidecar {#customize-mount-pod}

After you modify the ConfigMap, we recommend that you use the [smooth upgrade feature](../administration/upgrade-juicefs-client.md#smooth-upgrade) to apply the changes without interrupting service. To fully utilize this feature, you need v0.25.2 or later. Some items do not support smooth upgrade in v0.25.0 (the initial release of this feature).

If you cannot use the smooth upgrade feature, you need to rebuild the application Pod or the Mount Pod, as described in the sections below. Make sure to configure [automatic mount point recovery](./configurations.md#automatic-mount-point-recovery) in advance. This prevents the mount point in the application Pod from being permanently lost after rebuilding the Mount Pod.

### Custom mount image {#custom-image}

#### Via ConfigMap {#custom-image-via-configmap}

Please refer to the ["Upgrade container images for Mount Pods"](../administration/upgrade-juicefs-client.md#upgrade-mount-pod-image) document.

### Environment variables {#custom-env}

#### Via ConfigMap

The minimum required version is CSI Driver v0.24.5. Upon modification, application Pods need to be re-created for changes to take effect.

```yaml {2-6}
  mountPodPatch:
    - env:
      - name: DEMO_GREETING
        value: "Hello from the environment"
      - name: DEMO_FAREWELL
        value: "Such a sweet sorrow"
```

#### Via Secret

```yaml {11}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
  namespace: default
  labels:
    # Add this label to enable secret validation
    juicefs.com/validate-secret: "true"
type: Opaque
stringData:
  envs: '{"BASE_URL": "http://10.0.0.1:8080/static"}'
```

### Resource definition {#custom-resources}

#### Via ConfigMap {#custom-resources-via-configmap}

The minimum version of the CSI Driver required for this feature is 0.24.0. An example is as follows:

```yaml {2-5}
  mountPodPatch:
    - resources:
        requests:
          cpu: 100m
          memory: 512Mi
```

Read [resource optimization](./resource-optimization.md#mount-pod-resources) to learn how to properly set resource requests and limits.

### Mount options {#mount-options}

Each JuiceFS mount point is created by the `juicefs mount` command, and within the CSI Driver system, `mountOptions` manages all mount options.

`mountOptions` supports both JuiceFS mount options and FUSE options. Note that although FUSE options are specified with `-o` in the JuiceFS command line, you must omit `-o` inside CSI `mountOptions` and just append each option directly in the YAML list. For example, a mount command like this:

```shell
juicefs mount ... --cache-size=204800 -o writeback_cache,debug
```

It would translate to CSI `mountOptions` as follows:

```yaml
mountOptions:
  # JuiceFS mount options
  - cache-size=204800
  # Extra FUSE options
  - writeback_cache
  - debug
```

:::tip
Mount options are different between the Community Edition and Cloud Service. See:

- [Community Edition](https://juicefs.com/docs/community/command_reference#mount)
- [Cloud Service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount)

:::

#### Via ConfigMap

The minimum required version is CSI Driver v0.24.7. Upon modification, application Pods need to be re-created for changes to take effect.

Items inside ConfigMap comes with the highest priority, and mount options defined in CM will recursively overwrite those defined in PV. To avoid confusion, please migrate all mount options to ConfigMap and avoid using PV-level `mountOptions`.

By using `pvcSelector`, you can control mount options for multiple PVCs.

```yaml
  mountPodPatch:
    - pvcSelector:
        matchLabels:
          # Applies to all PVCs with this label
          need-update-options: "true"
      mountOptions:
        - writeback
        - cache-size=204800
```

#### Via PV definition (deprecated) {#static-mount-options}

After modifying the mount options for existing PVs, a rolling upgrade or re-creation of the application Pod is required to apply the changes. This ensures CSI Driver re-creates the Mount Pod for the changes to take effect.

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

#### Via StorageClass definition (deprecated) {#dynamic-mount-options}

You can customize mount options in `StorageClass` definition. If different applications require different mount options, create multiple `StorageClass`, each with its own mount options.

Since StorageClass serves as a template for creating PVs, **modifying mount options in StorageClass will not affect existing PVs**. If you need to adjust mount options for dynamic provisioning, you have to delete existing PVCs, or [directly modify mount options in existing PVs](#static-mount-options).

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

### Health check & Pod lifecycle {#custom-probe-lifecycle}

The minimum version of the CSI Driver required for this feature is 0.24.0. Targeted scenarios:

- Use `readinessProbe` to set up health checks for the Mount Pod, supporting monitoring and alerting.
- Customize `preStop` in sidecars to ensure the mount container exits after the application container. Refer to [sidecar recommendations](../administration/going-production.md#sidecar) for details.

```yaml
  - pvcSelector:
      matchLabels:
        custom-probe: "true"
    readinessProbe:
      exec:
        command:
        - stat
        - ${MOUNT_POINT}/${SUB_PATH}
      failureThreshold: 3
      initialDelaySeconds: 10
      periodSeconds: 5
      successThreshold: 1
```

### Mount extra volumes {#custom-volumes}

Targeted scenarios:

- Some object storage providers (like Google Cloud Storage) require extra credential files for authentication. This means you will have to create a separate Secret to store these files and reference it in volume credentials (JuiceFS-secret in below examples), so that CSI Driver will mount these files into the Mount Pod. The relevant environment variable needs to be added to specify the added files for authentication.
- JuiceFS Enterprise Edition supports [shared block storage device](https://juicefs.com/docs/cloud/guide/block-device), which can be used as cache storage or permanent data storage.

#### Via ConfigMap

The minimum required version is CSI Driver v0.24.7. Upon modification, application Pods need to be re-created for changes to take effect.

```yaml
  # Mount some volumes to the Mount Pod
  - pvcSelector:
      matchLabels:
        need-block-device: "true"
    volumeDevices:
      - name: block-devices
        devicePath: /dev/sda1
    volumes:
      - name: block-devices
        persistentVolumeClaim:
          claimName: block-pv
  - pvcSelector:
      matchLabels:
        need-mount-secret: "true"
    volumeMounts:
      - name: config-1
        mountPath: /root/.config/gcloud
    volumes:
    - name: gc-secret
      secret:
        secretName: gc-secret
        defaultMode: 420
```

#### Via Secret

JuiceFS Secret only supports configuring extra secret mounts within the `configs` field. Shared block device mounts are not supported here.

```yaml {8-9}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  ...
  # Set Secret name and mount directory in configs. This mounts the whole Secret into the specified directory
  configs: "{gc-secret: /root/.config/gcloud}"
```

### Cache {#custom-cachedirs}

Cache usage is also closely related to resource definition, data warmup, and cache cleanup, navigate to [Cache](./cache.md) to learn more.

### Other features

Many features are closely relevant to other topics. For more information:

* Configure delayed deletion for Mount Pods to reduce startup overhead in short application Pod lifecycles. read [delayed deletion](./resource-optimization.md#delayed-mount-pod-deletion).
* Clean cache upon Mount Pod exit. See [cache cleanup](./resource-optimization.md#clean-cache-when-mount-pod-exits).

## Format options / auth options {#format-options}

Format options / auth options are options used in `juicefs [format|auth]` commands, in which:

* The [`format`](https://juicefs.com/docs/community/command_reference/#format) command from JuiceFS CE is used to create a new file system, only then can you mount a file system via the `mount` command;
* The [`auth`](https://juicefs.com/docs/cloud/reference/command_reference/#auth) command from JuiceFS EE authenticates against the web console, and fetch configurations for the client. Its role is somewhat similar to the above `format` command, this due to the differences between the two editions: CE needs to create a file system using our cli, while EE users create file systems directly from the web console, and authenticate later when they need to actually mount the file systems (via the `auth` command).

Considering the similarities between the two commands, options all go to the `format-options` field, as follows.

:::tip
Changing `format-options` does not affect existing mount clients, even if Mount Pods are restarted. You need to rolling update / re-create the application Pods, or re-create PVC for the changes to take effect.
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

```yaml {11}
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

```yaml {9-11,22-24}
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

A murating webhook mutates Kubernetes resources, in our case all Pod creation under the specified namespace will go through our webhook, and if JuiceFS PV is used, webhook will inject the corresponding sidecar container.

### Validating webhook

CSI Driver can optionally run secret validation, helping users to correctly fill in their [volume credentials](./pv.md#volume-credentials). If a wrong [volume token](https://juicefs.com/docs/zh/cloud/acl#client-token) is used, the secret fails to create and user is prompted with relevant errors.

To enable validating webhook, write this in your cluster values (refer to our default [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml#L342)):

```yaml name="values-mycluster.yaml"
validatingWebhook:
  enabled: true
```

## Advanced PV provisoning {#provioner}

CSI Driver provides 2 types of PV provisioning:

* Using standard [Kubernetes CSI provisioner](https://github.com/kubernetes-csi/external-provisioner), which is the default mode for older versions of CSI Driver. When running in this mode, juicefs-csi-controller Pod includes 4 containers
* (Recommended) Move away from the standard CSI provisioner and use our own in-house controller as provisioner. Since v0.23.4, if CSI Driver is installed via Helm, then this feature is already enabled, and juicefs-csi-controller Pod will only have 3 containers

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

If you use the kubectl installation method, enabling this feature requires manual editing of the CSI Controller, which is complicated. Therefore, it is recommended to [migrate to Helm installation method](../administration/upgrade-csi-driver.md#migrate-to-helm).

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

```yaml {12-14}
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

JuiceFS CSI Driver supports defining a path pattern for PV directories created in JuiceFS, making directory names easier to read and locate:

```shell
$ ls /jfs
default-dummy-juicefs-pvc  default-example-juicefs-pvc ...
```

:::tip

* For a StorageClass that is in use, if you change it midway and add `pathPattern`, all subsequent PV directories will employ a new name format, different from the original `pvc-xxx-xxx...` UUID format, where all existing data resides. If you find the new mount directories empty, simply move the data to the new directories.
* Under dynamic provisioning, if you need to use a single shared directory across multiple applications, you can configure `pathPattern` so that multiple PVs can write to the same JuiceFS sub-directory. However, [static provisioning](#share-directory) is a more simple and straightforward way to achieve shared storage across multiple applications (just use a single PVC among multiple applications). If possible, consider using static provisioning for easier setup.

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

In version 0.23.3, metadata of Node and PVC can be injected into the mount parameters and `pathPattern`, such as:

1. `${.node.name}-${.node.podCIDR}`, inject node `metadata.name` and `spec.podCIDR`, e.g. `minikube-10.244.0.0/24`
2. `${.node.labels.foo}`, inject node label `metadata.labels["foo"]`
3. `${.node.annotations.bar}`, inject node annotation `metadata.annotations["bar"]`
4. `${.pvc.namespace}-${.pvc.name}`，inject `metadata.namespace` and `metadata.name` of PVC, e.g. `default-dynamic-pvc`
5. `${.PVC.namespace}-${.PVC.name}`，inject `metadata.namespace` and `metadata.name` of PVC (compatible with older versions)
6. `${.pvc.labels.foo}`, inject `metadata.labels["foo"]` of PVC
7. `${.pvc.annotations.bar}`, inject `metadata.annotations["bar"]` of PVC

In earlier versions (>=0.13.3) only `pathPattern` supports injection, and only supports injecting PVC metadata, such as:

1. `${.PVC.namespace}-${.PVC.name}`，inject `metadata.namespace` and `metadata.name` of PVC (compatible with older versions)
2. `${.PVC.labels.foo}`, inject `metadata.labels["foo"]` of PVC
3. `${.PVC.annotations.bar}`, inject `metadata.annotations["bar"]` of PVC

## Common PV settings {#common-pv-settings}

### Automatic mount point recovery {#automatic-mount-point-recovery}

Since v0.25.0, JuiceFS CSI Driver supports [smooth upgrade of Mount Pods](../administration/upgrade-juicefs-client.md#smooth-upgrade), leveraging the JuiceFS Client's zero-downtime restart capability (learn more in the [Community Edition](https://juicefs.com/docs/community/administration/upgrade) and [Enterprise Edition](https://juicefs.com/docs/cloud/getting_started#upgrade-juicefs) documentation). If a Mount Pod restarts or encounters a crash, CSI Node will hold all open file descriptors, making existing FUSE requests hang until Mount Pod recovers. This is usually fast and there will not be any timeout or other exceptions. Hence, for v0.25.0 and newer versions, practices introduced in this section are **no longer necessary but still recommended**: CSI Node guarantees smooth recovery. However, it is still recommended to configure `mountPropagation` as a safeguard. In rare cases where CSI Node might encounter issues, `mountPropagation` will ensure the mount point automatically recovers, even if the smooth restart mechanism fails.

For CSI Driver versions prior to v0.25.0, if a Mount Pod crashes (for example, due to OOM) and restarts, despite that the mount point within the Mount Pod can recover normally, the mount point inside the application Pod will not recover since it relies on an external binding from CSI Node (`mount --bind`). So by default, upon a Mount Pod restart, mount point within the application Pod is lost permanently, and any access will result in a `Transport endpoint is not connected` error.

To prevent such issues, we recommend enabling mount propagation in all application Pods. This approach allows the recovered mount point to be bound back. However, note that the process is not completely smooth. Although the mount point can be recovered, any existing file handlers are rendered unusable by the Mount Pod restart. Application must be able to handle bad file descriptors and re-open them to avoid further exceptions.

To enable automatic mount point recovery, applications need to [set `mountPropagation` to `HostToContainer` or `Bidirectional`](https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation) in Pod `volumeMounts`. In this way, host mount is propagated to the Pod, so when Mount Pod restarts by accident, CSI Driver will bind mount once again when host mount point recovers.

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

* &#8203;<Badge type="primary">On-prem</Badge> If JuiceFS Web Console suffers from an outage, or clients undergo network issue, Mount Pods & sidecar containers can still mount via the cached config and continue to serve

Caching works like this:

1. Users create or update [volume credential](./pv.md#volume-credentials), CSI Controller will watch for changes and immediately run `juicefs auth` to obtain the new config;
2. CSI Controller injects configuration into the secret, saved as the `initconfig` field;
3. When CSI Node creates Mount Pod or CSI Controller injecting a sidecar container, `initconfig` is mounted into the container;
4. JuiceFS clients within the container run [`juicefs auth`](https://juicefs.com/docs/cloud/reference/command_reference/#auth), since config file is already present inside the container, mount will proceed even if the auth command fails.

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

After PV is created and mounted, verify by executing `df -h` command within the application Pod:

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

The above method will not affect existing PVs. If you need to expand an existing PV, you must manually update the PV to include the Secret configuration.

```yaml {7-9}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pvc-xxxx
spec:
  csi:
    controllerExpandSecretRef:
      name: juicefs-secret
      namespace: default
```

### Access modes {#access-modes}

JuiceFS PV supports `ReadWriteMany` and `ReadOnlyMany` as access modes, change the `accessModes` field accordingly in above PV/PVC (or `volumeClaimTemplate`) definitions.

### Reclaim policy {#relaim-policy}

Under static provisioning, only `persistentVolumeReclaimPolicy: Retain` is supported, static PVs cannot reclaim data with PV deletion.

Dynamic provisioning supports `Delete|Retain` policies, `Delete` causes data to be deleted with PV release, if data security is a concern, remember to enable the "trash" feature of JuiceFS:

* [JuiceFS Community Edition docs](https://juicefs.com/docs/community/security/trash)
* [JuiceFS Enterprise Edition docs](https://juicefs.com/docs/zh/cloud/trash)

### Mount host's directory in Mount Pod {#mount-host-path}

If you need to mount files or directories into the Mount Pod, use `juicefs/host-path`, you can specify multiple path (separated by comma) in this field. Also, this field appears in different locations for static / dynamic provisioning, take `/data/file.txt` for an example:

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

Mount the `/etc/hosts` file into the Pod. In some cases, you might need to directly use the node `/etc/hosts` file inside the container (however, [`HostAliases`](https://kubernetes.io/docs/tasks/network/customize-hosts-file-for-pods/) is usually the better approach).

```yaml
juicefs/host-path: "/etc/hosts"
```

If you need to mount multiple files or directories, specify them using comma:

```yaml
juicefs/host-path: "/data/file1.txt,/data/file2.txt,/data/dir1"
```
