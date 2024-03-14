---
title: Create and Use PV
sidebar_position: 1
---

## Volume credentials {#volume-credentials}

In JuiceFS, a Volume is a file system. With JuiceFS CSI Driver, Volume credentials are stored inside a Kubernetes Secret, note that for JuiceFS Community Edition and JuiceFS Cloud Service, meaning of volume credentials are different:

* For Community Edition, volume credentials include metadata engine URL, object storage keys, and other options supported by the [`juicefs format`](https://juicefs.com/docs/community/command_reference#format) command.
* For Cloud Service, volume credentials include Token, object storage keys, and other options supported by the [`juicefs auth`](https://juicefs.com/docs/cloud/reference/commands_reference/#auth) command.

Although in the examples below, secrets are usually named `juicefs-secret`, they can actually be freely named, and you can create multiples to store credentials for different file systems. This allows using multiple different JuiceFS volumes within the same Kubernetes cluster. Read [using multiple file systems](#multiple-volumes) for more.

:::tip

* If you're already [managing StorageClass via Helm](#helm-sc), you can skip this step as the Kubernetes Secret is already created along the way.
* After modifying the volume credentials, you need to perform a rolling upgrade or restart the application pod, and the CSI Driver will recreate the Mount Pod for the configuration changes to take effect.
* Secret only stores the volume credentials (that is, the options required by the `juicefs format` command (community version) and the `juicefs auth` command (cloud service)), and does not support filling in the mount options. If you want to modify the mount options, refer to ["Mount options"](#mount-options).

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

```yaml {7-14}
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
  # Replace $JUICEFS_CONSOLE_URL with the actual on-premise web console URL
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

### Using multiple file systems {#multiple-volumes}

Secret name can be customized, you can create multiple secrets with different names, or even put in different namespaces, in order to use multiple JuiceFS volumes, or use the same volume across different Kubernetes namespaces.

```yaml {4-5,11-12}
---
apiVersion: v1
metadata:
  name: vol-secret-1
  namespace: default
kind: Secret
...
---
apiVersion: v1
metadata:
  name: vol-secret-2
  namespace: kube-system
kind: Secret
...
```

Depending on whether you're using static or dynamic provisioning, the secrets created above have to be referenced in the PV or StorageClass definition, in order to correctly mount. Taking the above volume credentials for an example, the corresponding static / dynamic provisioning config may look look like below.

For static provisioning (if you aren't yet familiar, read [static provisioning](#static-provisioning)).

```yaml {10-11,14-15,25,28-29}
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: vol-1
spec:
  ...
  csi:
    driver: csi.juicefs.com
    # This field should be globally unique, thus it's recommended to use the PV name
    volumeHandle: vol-1
    fsType: juicefs
    nodePublishSecretRef:
      name: vol-secret-1
      namespace: default
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: vol-2
spec:
  ...
  csi:
    driver: csi.juicefs.com
    volumeHandle: vol-2
    fsType: juicefs
    nodePublishSecretRef:
      name: vol-secret-2
      namespace: kube-system
```

For dynamic provisioning (if you aren't yet familiar, read [dynamic provisioning](#static-provisioning)).

```yaml {8-11,19-22}
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vol-1
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: vol-1
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: vol-1
  csi.storage.k8s.io/node-publish-secret-namespace: default
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vol-2
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: vol-2
  csi.storage.k8s.io/provisioner-secret-namespace: kube-system
  csi.storage.k8s.io/node-publish-secret-name: vol-2
  csi.storage.k8s.io/node-publish-secret-namespace: kube-system
```

### Adding extra files / environment variables into mount pod {#mount-pod-extra-files}

Some object storage providers (like Google Cloud Storage) requires extra credential files for authentication, this means you'll have to create a separate Secret to store these files, and reference it in volume credentials (`juicefs-secret` in below examples), so that CSI Driver will mount these files into the mount pod. The relevant environment variable needs to be added to specify the added files for authentication.

If you need to add environment variables for mount pod, use the `envs` field in volume credentials. For example MinIO may require clients to set the `MINIO_REGION` variable.

Here we'll use Google Cloud Storage as example, to demonstrate how to add extra files / environment variables into mount pod.

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

Static provisioning is the most simple way to use JuiceFS PV inside Kubernetes, follow below steps to mount the whole file system info the application pod (also refer to [mount subdirectory](#mount-subdirectory) if in need), read [Usage](../introduction.md#usage) to learn about dynamic provisioning and static provisioning.

Create the following Kubernetes resources, refer to YAML comments for field descriptions:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  labels:
    juicefs-name: ten-pb-fs
spec:
  # For now, JuiceFS CSI Driver doesn't support setting storage capacity for static PV. Fill in any valid string is fine.
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
  # For now, JuiceFS CSI Driver doesn't support setting storage capacity for static PV. Fill in any valid string that's lower than the PV capacity.
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

After pod is up and running, you'll see `out.txt` being created by the container inside the JuiceFS mount point. For static provisioning, if [mount subdirectory](#mount-subdirectory) is not explicitly specified, the root directory of the file system will be mounted into the container. Mount a subdirectory or use [dynamic provisioning](#dynamic-provisioning) if data isolation is required.

## Create a StorageClass {#create-storage-class}

[StorageClass](https://kubernetes.io/docs/concepts/storage/storage-classes) handles configurations to create different PVs, think of it as a profile for dynamic provisioning: each StorageClass may contain different volume credentials and mount options, so that you can use multiple settings under dynamic provisioning. Thus if you decide to use JuiceFS CSI Driver via [dynamic provisioning](#dynamic-provisioning), you'll need to create a StorageClass in advance.

Due to StorageClass being the template used for creating PVs, **modifying mount options in StorageClass will not affect existing PVs**, if you need to adjust mount options under dynamic provisioning, you'll have to delete existing PVCs, or [directly modify mount options in existing PVs](#static-mount-options).

### Create via Helm {#helm-sc}

:::tip

* Managing StorageClass via Helm requires putting credentials directly in `values.yaml`, thus is usually advised against in production environments.
* As is demonstrated with the `backend` field in the below examples, when StorageClass is created by Helm, volume credentials is created along the way, you should manage directly in Helm, rather than [creating volume credentials separately](#volume-credentials).
:::

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

### Create via kubectl

[Volume credentials](#volume-credentials) is referenced in the StorageClass definition, so you'll have to create them in advance, and then fill in its information into the StorageClass definition shown below.

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

Read [Usage](../introduction.md#usage) to learn about dynamic provisioning. Dynamic provisioning automatically creates PV for you, each corresponds to a sub-directory inside the JuiceFS volume, and the parameters needed by PV resides in StorageClass, thus you'll have to [create a StorageClass](#create-storage-class) in advance.

Create PVC and example pod:

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
      # request 10GiB storage capacity from StorageClass
      storage: 10Gi
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
  namespace: ns1
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
  namespace: ns2
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

## Use more readable names for PV directory {#using-path-pattern}

:::tip
Not supported in [mount by process mode](../introduction.md#by-process).
:::

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

Under dynamic provisioning, if you need to use a single shared directory across multiple applications, you can configure `pathPattern` so that multiple PVs write to the same JuiceFS sub-directory. However, [static provisioning](#share-directory) is a more simple & straightforward way to achieve shared storage across multiple applications (just use a single PVC among multiple applications), use this if the situation allows.

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

You can also use tools provided by a community developer to automatically add `mountPropagation: HostToContainer` to application container. For details, please refer to [Project Documentation](https://github.com/breuerfelix/juicefs-volume-hook).

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

In JuiceFS CSI Driver version 0.21.0 and above, PersistentVolume expansion is supported (only [dynamic provisioning](#dynamic-provisioning) is supported). You need to specify `allowVolumeExpansion: true` in [StorageClass](#create-storage-class), and specify the Secret to be used when expanding the capacity, which mainly provides authentication information of the file system, for example:

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
