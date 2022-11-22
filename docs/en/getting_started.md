---
title: Installation
---

## Installing JuiceFS CSI Driver

JuiceFS CSI Driver requires Kubernetes 1.14 and above, follow below steps to install.

### Install via Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

Installation requires Helm 3.1.0 and above, refer to the [Helm Installation Guide](https://helm.sh/docs/intro/install) and ensure that the `helm` binary is in the `PATH` environment variable.

1. Check kubelet root directory

   Execute the following command.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   If the result is not empty or the default `/var/lib/kubelet`, it means that the kubelet root directory is customized, you need to set `kubeletDir` to the current kubelet root directly in `values.yaml`.

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

2. Deploy

   Execute below commands to deploy JuiceFS CSI Driver.

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

4. Verify installation

   Verify all CSI Driver components are running:

   ```shell
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

   Learn about JuiceFS CSI Driver architecture, and components functionality in [Introduction](./introduction.md).

### Install via kubectl

1. Check kubelet root directory

   Execute the following command on any non-Master node in the Kubernetes cluster.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. Deploy

   - If above command returns a non-empty result other than `/var/lib/kubelet`, it means kubelet root directory (`--root-dir`) was customized, you need to update the `kubeletDir` path in the CSI Driver's deployment file.

     ```shell
     # replace {{KUBELET_DIR}} in the below command with the actual root directory path of kubelet.

     # Kubernetes version >= v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

     # Kubernetes version < v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

   - If the command returns an empty result, deploy without modifications:

     ```shell
     # Kubernetes version >= v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
     ```

     ```shell
     # Kubernetes version < v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
     ```

## Create a StorageClass {#create-storage-class}

If you decide to use JuiceFS CSI Driver via [dynamic provisioning](./guide/pv.md#dynamic-provisioning), you'll need to create a StorageClass in advance.

Learn about dynamic provisioning and static provisioning in [Usage](./introduction.md#usage).

### Helm

Create `values.yaml` using below content, note that it only contains the basic configurations, refer to [Values](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/README.md#values) for a full description.

Configuration are different between Cloud Service and Community Edition, using Community Edition as an example:

```yaml title="values.yaml"
storageClasses:
- name: juicefs-sc
  enabled: true
  reclaimPolicy: Retain
  # JuiceFS volume related configuration
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

### kubectl

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

### Adjust mount options

To customize mount options, append `mountOptions` to the above StorageClass definition. If you need to use different mount options for different applications, you'll need to create multiple StorageClass.

```yaml
mountOptions:
  - enable-xattr
  - max-uploads=50
  - cache-size=2048
  - cache-dir=/var/foo
  - allow_other
```

Mount options are different between Community Edition and Cloud Service, see:

- [Community Edition](https://juicefs.com/docs/zh/community/command_reference#juicefs-mount)
- [Cloud Service](https://juicefs.com/docs/zh/cloud/reference/commands_reference/#mount)

