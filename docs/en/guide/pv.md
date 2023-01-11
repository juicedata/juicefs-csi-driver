---
title: Create and Use PV
sidebar_position: 1
---

## Volume credentials {#volume-credentials}

In JuiceFS, a Volume is a file system. With JuiceFS CSI Driver, Volume credentials are stored inside a Kubernetes Secret, note that for JuiceFS Community Edition and JuiceFS Cloud Service, meaning of volume credentials are different:

* For Community Edition, volume credentials include metadata engine URL, object storage keys, and other options supported by the [`juicefs format`](https://juicefs.com/docs/community/command_reference#format) command.
* For Cloud Service, volume credentials include Token, object storage keys, and other options supported by the [`juicefs auth`](https://juicefs.com/docs/cloud/reference/commands_reference/#auth) command.

:::note
If you're already [managing StorageClass via Helm](#helm-sc), then the needed Kubernetes Secret is already created along the way, in this case we recommend you to continue managing StorageClass and Kubernetes Secret by Helm, rather than creating a separate Secret using kubectl.
:::

### Community edition {#community-edition}

Create Kubernetes Secret:

```yaml {7-16}
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
  # Adjust mount pod timezone, defaults to UTC.
  # envs: "{TZ: Asia/Shanghai}"
  # If you need to format a volume within the mount pod, fill in format options below.
  # format-options: trash-days=1,block-size=4096
```

Fields description:

- `name`: The JuiceFS file system name
- `metaurl`: Connection URL for metadata engine. Read [Set Up Metadata Engine](https://juicefs.com/docs/community/databases_for_metadata) for details
- `storage`: Object storage type, such as `s3`, `gs`, `oss`. Read [Set Up Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage) for the full supported list
- `bucket`: Bucket URL. Read [Set Up Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage) to learn how to setup different object storage
- `access-key`/`secret-key`: Object storage credentials
- `envs`：Mount pod environment variables
- `format-options`: Options used when creating a JuiceFS volume, see [`juicefs format`](https://juicefs.com/docs/community/command_reference#format). This options is only available in v0.13.3 and above

Information like `access-key` can be specified both as a Secret `stringData` field, and inside `format-options`. If provided in both places, `format-options` will take precedence.

### Cloud service {#cloud-service}

Before continue, you should have already [created a file system](https://juicefs.com/docs/cloud/getting_started#create-file-system).

Create Kubernetes Secret:

```yaml {7-16}
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
  # Adjust mount pod timezone, defaults to UTC.
  # envs: "{TZ: Asia/Shanghai}"
  # If you need to specify more authentication options, fill in juicefs auth parameters below.
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

Fields description:

- `name`: The JuiceFS file system name
- `token`: Token used to authenticate against JuiceFS Volume, see [Access token](https://juicefs.com/docs/cloud/acl#access-token)
- `access-key`/`secret-key`: Object storage credentials
- `envs`：Mount pod environment variables
- `format-options`: Options used by the [`juicefs auth`](https://juicefs.com/docs/cloud/commands_reference#auth) command, this command deals with authentication and generate local mount configuration. This options is only available in v0.13.3 and above

Information like `access-key` can be specified both as a Secret `stringData` field, and inside `format-options`. If provided in both places, `format-options` will take precedence.

For Cloud Service, the `juicefs auth` command is somewhat similar to the `juicefs format` in JuiceFS Community Edition, thus CSI Driver uses `format-options` for both scenarios.

### Enterprise edition (on-premises) {#enterprise-edition}

The JuiceFS Web Console is in charge of client authentication and distributing configuration files. In an on-premises deployment, the console address won't be [https://juicefs.com/console](https://juicefs.com/console), so it's required to specify the address for JuiceFS Web Console through `envs` field in volume credentials.

```yaml {12-13}
apiVersion: v1
metadata:
  name: juicefs-secret
  namespace: default
kind: Secret
type: Opaque
stringData:
  name: ${JUICEFS_NAME}
  token: ${JUICEFS_TOKEN}
  access-key: ${ACCESS_KEY}
  secret-key: ${SECRET_KEY}
  # Leave the `%s` placeholder as-is, it'll be replaced with the actual file system name during runtime
  envs: '{"BASE_URL": "$JUICEFS_CONSOLE_URL/static", "CFG_URL": "$JUICEFS_CONSOLE_URL/volume/%s/mount"}'
  # If you need to specify more authentication options, fill in juicefs auth parameters below.
  # format-options: bucket2=xxx,access-key2=xxx,secret-key2=xxx
```

Fields description:

- `name`: The JuiceFS file system name
- `token`: Token used to authenticate against JuiceFS Volume, see [Access token](https://juicefs.com/docs/cloud/acl#access-token)
- `access-key`/`secret-key`: Object storage credentials
- `envs`：Mount pod environment variables, in an on-premises environment, you need to additionally specify `BASE_URL` and `CFG_URL`, pointing to the actual console address
- `format-options`: Options used by the [`juicefs auth`](https://juicefs.com/docs/cloud/commands_reference#auth) command, this command deals with authentication and generate local mount configuration. This options is only available in v0.13.3 and above

### Adding extra files into mount pod {#mount-pod-extra-files}

Some object storage providers (like Google Cloud Storage) requires extra credential files for authentication, this means you'll have to create a new Secret to store these files (different from the previously created Secret for JuiceFS), and reference it in `juicefs-secret`, so that CSI Driver know how to mount these files into the mount pod, and use it for authentication during mount. Here we'll use Google Cloud Storage as example, but the process is the same for any scenarios that needs to add extra files into the mount pod.

To obtain the [service account key file](https://cloud.google.com/docs/authentication/production#create_service_account), you need to first learn about [authentication](https://cloud.google.com/docs/authentication) and [authorization](https://cloud.google.com/iam/docs/overview). Assuming you already have the key file `application_default_credentials.json`, create the corresponding Kubernetes Secret:

```shell
kubectl create secret generic gc-secret \
  --from-file=application_default_credentials.json=application_default_credentials.json
```

Now that the key file is saved in `gc-secret`, we'll reference it in `juicefs-secret`, this tells CSI Driver to mount the files into the mount pod, and set relevant environment variables accordingly:

```yaml {8-11}
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  ...
  # Set Secret name and mount directory in configs, this mounts the whole Secret into specified directory
  configs: "{gc-secret: /root/.config/gcloud}"
  # Define environment variables required by the authentication process
  envs: "{GOOGLE_APPLICATION_CREDENTIALS: /root/.config/gcloud/application_default_credentials.json}"
```

After this is done, newly created PVs will start to use this configuration. You can [enter the mount pod](../administration/troubleshooting.md#check-mount-pod) and verify that the files are correctly mounted, and use `env` command to ensure the variables are set.

## Static provisioning {#static-provisioning}

Static provisioning is the most simple way to use JuiceFS PV inside Kubernetes, read [Usage](../introduction.md#usage) to learn about dynamic provisioning and static provisioning.

Create PersistentVolume (PV), PersistentVolumeClaim (PVC), refer to YAML comments for field descriptions:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  # For now, JuiceFS CSI Driver doesn't support setting storage capacity. Fill in any valid string is fine.
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  csi:
    # A CSIDriver named csi.juicefs.com is created during installation
    driver: csi.juicefs.com
    # volumeHandle needs to be unique within the cluster, simply using the PV name is recommended
    volumeHandle: juicefs-pv
    fsType: juicefs
    # Reference the volume credentials (Secret) created in previous step
    # If you need to use different credentials, or even use different JuiceFS volumes, you'll need to create different volume credentials
    nodePublishSecretRef:
      name: juicefs-secret
      namespace: default
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  volumeMode: Filesystem
  # Must use an empty string as storageClassName
  # Meaning that this PV will not use any StorageClass, instead will use the PV specified by selector
  storageClassName: ""
  # For now, JuiceFS CSI Driver doesn't support setting storage capacity. Fill in any valid string that's lower than the PV capacity.
  resources:
    requests:
      storage: 10Pi
  selector:
    matchLabels:
      juicefs-name: ten-pb-fs
```

And then create an application pod, using the PVC created above:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: data
    resources:
      requests:
        cpu: 10m
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: juicefs-pvc
```

## Create a StorageClass {#create-storage-class}

If you decide to use JuiceFS CSI Driver via [dynamic provisioning](#dynamic-provisioning), you'll need to create a StorageClass in advance.

Learn about dynamic provisioning and static provisioning in [Usage](../introduction.md#usage).

### Create via Helm {#helm-sc}

Create `values.yaml` using below content, note that it only contains the basic configurations, refer to [Values](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/README.md#values) for a full description.

Configuration are different between Cloud Service and Community Edition, below example is for Community Edition, but you will find full description at [Helm chart](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml#L122).

```yaml title="values.yaml"
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  # JuiceFS Volume credentials
  # If volume is already created in advance, then only name and metaurl is needed
  backend:
    name: "<name>"               # JuiceFS volume name
    metaurl: "<meta-url>"        # URL of metadata engine
    storage: "<storage-type>"    # Object storage type (e.g. s3, gcs, oss, cos)
    accessKey: "<access-key>"    # Access Key for object storage
    secretKey: "<secret-key>"    # Secret Key for object storage
    bucket: "<bucket>"           # A bucket URL to store data
    # Adjust mount pod timezone, defaults to UTC
    # envs: "{TZ: Asia/Shanghai}"
  mountPod:
    resources:                   # Resource limit/request for mount pod
      requests:
        cpu: "1"
        memory: "1Gi"
      limits:
        cpu: "5"
        memory: "5Gi"
```

As is demonstrated with the `backend` field, when StorageClass is created by Helm, volume credentials is created along the way, you should manage directly in Helm, rather than [creating volume credentials separately](#volume-credentials).

### Create via kubectl

```yaml
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
```

## Dynamic provisioning {#dynamic-provisioning}

Read [Usage](../introduction.md#usage) to learn about dynamic provisioning. Dynamic provisioning automatically creates PV for you, and the parameters needed by PV resides in StorageClass, thus you'll have to [create a StorageClass](#create-storage-class) in advance.

### Deploy

Create PersistentVolumeClaim (PVC) and example pod:

```yaml {13}
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
---
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: juicefs-pv
  volumes:
  - name: juicefs-pv
    persistentVolumeClaim:
      claimName: juicefs-pvc
EOF
```

Verify that pod is running, and check if data is written into JuiceFS:

```shell
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

## Use generic ephemeral volume {#general-ephemeral-storage}

[Generic ephemeral volumes](https://kubernetes.io/docs/concepts/storage/ephemeral-volumes/#generic-ephemeral-volumes) are similar to `emptyDir`, which provides a per-pod directory for scratch data. When application pods need large volume, per-pod ephemeral storage, consider using JuiceFS as generic ephemeral volume.

Generic ephemeral volume works similar to dynamic provisioning, thus you'll need to [create a StorageClass](#create-storage-class) as well. But generic ephemeral volume uses `volumeClaimTemplate` which automatically creates PVC for each pod.

Declare generic ephemeral volume directly in pod definition:

```yaml {19-30}
apiVersion: v1
kind: Pod
metadata:
  name: juicefs-app
  namespace: default
spec:
  containers:
  - args:
    - -c
    - while true; do echo $(date -u) >> /data/out.txt; sleep 5; done
    command:
    - /bin/sh
    image: centos
    name: app
    volumeMounts:
    - mountPath: /data
      name: juicefs-pv
  volumes:
  - name: juicefs-pv
    ephemeral:
      volumeClaimTemplate:
        metadata:
          labels:
            type: juicefs-ephemeral-volume
        spec:
          accessModes: [ "ReadWriteMany" ]
          storageClassName: "juicefs-sc"
          resources:
            requests:
              storage: 1Gi
```

:::note
As for reclaim policy, generic ephemeral volume works the same as dynamic provisioning, so if you changed [the default PV reclaim policy](./resource-optimization.md#reclaim-policy) to `Retain`, the ephemeral volume introduced in this section will no longer be ephemeral, you'll have to manage PV lifecycle yourself.
:::

## Mount options {#mount-options}

Mount options are really just the options supported by the `juicefs mount` command, in CSI Driver, you need to specify them in the `mountOptions` field, which resides in different manifest locations between static provisioning and dynamic provisioning, see below examples.

### Static provisioning

```yaml {8-10}
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  mountOptions:
    - cache-size=204800
    - subdir=/my/sub/dir
  ...
```

### Dynamic provisioning

Customize mount options in `StorageClass` definition. If you need to use different mount options for different applications, you'll need to create multiple `StorageClass`, each with different mount options.

```yaml {6-8}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
mountOptions:
  - cache-size=204800
  - subdir=/my/sub/dir
parameters:
  ...
```

### Parameter descriptions

Mount options are different between Community Edition and Cloud Service, see:

- [Community Edition](https://juicefs.com/docs/community/command_reference#mount)
- [Cloud Service](https://juicefs.com/docs/cloud/reference/commands_reference/#mount)

If you need to pass extra FUSE options (specified in command line using `-o`), append directly in the YAML list, one option in each line, as demonstrated below:

```yaml
mountOptions:
  - cache-size=204800
  # Extra FUSE options
  - writeback_cache
  - debug
```

## Use more readable names for PV directory {#using-path-pattern}

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

This feature is disabled by default, to enable, you need to add the `--provisioner=true` option to CSI Controller start command, and delete the sidecar container, so that CSI Controller main process is in charge of watching for resource changes, and carrying out actual provisioning. Follow below steps to enable `pathPattern`.

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

### Usage

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
  pathPattern: "${.PVC.namespace}-${.PVC.name}"
```

You can reference any PVC metadata in the pattern, for example:

1. `${.PVC.namespace}-${.PVC.name}` results in the directory name being `<pvc-namespace>-<pvc-name>`.
1. `${.PVC.labels.foo}` results in the directory name being the `foo` label value.
1. `${.PVC.annotations.bar}` results in the PV directory name being the `bar` annotation value.

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

### PV storage capacity {#storage-capacity}

For now, JuiceFS CSI Driver doesn't support setting storage capacity. the storage specified under PersistentVolume and PersistentVolumeClaim is simply ignored, just use a reasonable size as placeholder (e.g. `100Gi`).

```yaml
resources:
  requests:
    storage: 100Gi
```

### Access modes {#access-modes}

JuiceFS PV supports `ReadWriteMany` and `ReadOnlyMany` as access modes, change the `accessModes` field accordingly in above PV/PVC (or `volumeClaimTemplate`) definitions.
