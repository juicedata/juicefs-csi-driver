---
title: Installation
---

JuiceFS CSI Driver requires Kubernetes 1.14 and above, follow below steps to install.

:::note
No special steps is required to install CSI Driver in an on-premises environment, however, you'll need to specify the Web Console address in [volume credentials](./guide/pv.md#enterprise-edition), within the `envs` field.
:::

## Helm {#helm}

In comparison to kubectl, Helm allows you to manage CSI Driver resources as a whole, and also makes it easier to modify configurations, or enable advanced features. Overall, Helm is recommended over kubectl, but if you are not familiar with Helm, and are simply trying to evaluate CSI Driver, it's OK to [install using kubectl](#kubectl).

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

Installation requires Helm 3.1.0 and above, refer to the [Helm Installation Guide](https://helm.sh/docs/intro/install).

1. Add the Helm repo, and then create a values file to store your cluster-specific configs, for example, if your cluster is named mycluster, then it's recommended to create a `values-mycluster.yaml` and put your configs there. The contents in this file will be recursively updated to the original [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml).

     ```shell
     helm repo add juicefs https://juicedata.github.io/charts/
     helm repo update

     mkdir juicefs-csi-driver && cd juicefs-csi-driver

     vi values-mycluster.yaml
     ```

1. Check kubelet root directory

   Execute the following command on any non-Master node in the Kubernetes cluster.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   If the result is not empty or the default `/var/lib/kubelet`, it means that the kubelet root directory is customized, you need to set `kubeletDir` to the current kubelet root directly in `values-mycluster.yaml`.

   ```yaml title="values-mycluster.yaml"
   kubeletDir: <kubelet-dir>
   ```

1. Go through [`values.yaml`](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/values.yaml) and see if there's other items that need adjustment. Add them to the above `values-mycluster.yaml` as well. The common configs are:

    * Search for `repository` and optionally change to your private image repository. If this is in need, you'll also need to [copy the Docker images](./administration/offline.md)
    * Search for `resources` and optionally adjust resource definitions for components

  After the above adjustments, your values file may look like:

   ```yaml title="values-mycluster.yaml"
   kubeletDir: <kubelet-dir>

   image:
     repository: registry.example.com/juicefs-csi-driver
   dashboardImage:
     repository: registry.example.com/csi-dashboard
   sidecars:
     livenessProbeImage:
       repository: registry.example.com/k8scsi/livenessprobe
     nodeDriverRegistrarImage:
       repository: registry.example.com/k8scsi/csi-node-driver-registrar
     csiProvisionerImage:
       repository: registry.example.com/k8scsi/csi-provisioner
     csiResizerImage:
       repository: registry.example.com/k8scsi/csi-resizer
   ```

1. Execute below commands to deploy JuiceFS CSI Driver:

    ```shell
    # Use this command for both initial installation, and subsequent config changes
    helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
    ```

It's recommended that you include the values file used above in the version control system, so that any changes to the config can be restored.

## kubectl {#kubectl}

kubectl is the simpler installation method compared to Helm, if you are simply trying to evaluate CSI Driver, this is recommended, **but in a production environment, installing via kubectl is strongly advised against**, because any configuration changes require manual editing, and can easily cause trouble if you are not familiar with CSI Controller. If you'd like to enable advanced features (e.g. [enable pathPattern](./guide/configurations.md#using-path-pattern)), or just want to manage resources easier, consider installing via Helm.

1. Check kubelet root directory

   Execute the following command on any non-Master node in the Kubernetes cluster.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. Deploy

   - If above command returns a non-empty result other than `/var/lib/kubelet`, it means kubelet root directory (`--root-dir`) was customized, you need to update the kubelet path in the CSI Driver's deployment file.

     ```shell
     # Replace {{KUBELET_DIR}} in the below command with the actual root directory path of kubelet.

     # Kubernetes version >= v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

     # Kubernetes version < v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

   - If the command returns an empty result, deploy without modifications:

     ```shell
     # Kubernetes version >= v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

     # Kubernetes version < v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
     ```

## Verify installation {#verify-installation}

Verify all CSI components are up and running:

```shell
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

CSI Node Service is a DaemonSet, and by default runs on all Kubernetes worker nodes, so CSI Node Pod amount should match node count. If the numbers don't match, check if any of the nodes are tainted, and resolve them according to the actual situation. You can also [run CSI Node Service on selected nodes](./guide/resource-optimization.md#csi-node-node-selector).

Learn about JuiceFS CSI Driver architecture, and components functionality in [Introduction](./introduction.md#architecture).

## Installing in sidecar mode {#sidecar}

:::tip Serverless Headsup
Since v0.23.5, Helm chart supports `mountMode: serverless`, a special form of sidecar mode which removes everything not supported in a serverless environment, e.g. hostPath mount points, and container privileges.

The `serverless` mode allows CSI Driver to be installed in full virtual nodes, in comparison, the default `sidecar` mode still requires an actual VM.
:::

Sidecar is very different from the default Mount Pod mode, for example, sharing JuiceFS Client is not available, neither does it support [automatic mount point recovery](./guide/configurations.md#automatic-mount-point-recovery).

### Helm

Modify your cluster values:

```yaml title='values-mycluster.yaml'
mountMode: sidecar
```

If [CertManager](https://github.com/cert-manager/cert-manager) is used to manage certificates in the cluster, add the following configuration in values:

```yaml title='values-mycluster.yaml'
mountMode: sidecar
webhook:
  certManager:
    enabled: true
```

Reinstall to apply:

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
```

:::warning
After installation, you must wait until all components are up and running, and then carry on with the labeling. If namespace is labeled before controller is up, all Pods within the namespace will stuck on creating, waiting for the webhook injection check.
:::

Label all namespaces that need to use JuiceFS CSI Driver:

```shell
kubectl label namespace $NS juicefs.com/enable-injection=true --overwrite
```

### kubectl

The files used for installation are generated using a script, which isn't ideal for source code management, while making it difficult to upgrade CSI Driver. Please don't install via kubectl in a production environment.

```shell
# Sidecar mode uses local generated certificates, rendered into the YAML files, this is all handled in the installation script
wget https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/juicefs-csi-webhook-install.sh
chmod +x ./juicefs-csi-webhook-install.sh

# Generate installation files
./juicefs-csi-webhook-install.sh print > juicefs-csi-sidecar.yaml

# Thoroughly check this YAML file, and install
kubectl apply -f ./juicefs-csi-sidecar.yaml
```

Or directly install using this command:

```shell
./juicefs-csi-webhook-install.sh install
```

:::warning
After installation, you must wait until all components are up and running, and then carry on with the labeling. If namespace is labeled before controller is up, all Pods within the namespace will stuck on creating, waiting for the webhook injection check.
:::

Label all namespaces that need to use JuiceFS CSI Driver, note that the label is different for serverless.

```shell
# Normal Kubernetes cluster
kubectl label namespace $NS juicefs.com/enable-injection=true --overwrite
# Serverless cluster
kubectl label namespace $NS juicefs.com/enable-serverless-injection=true --overwrite
```

If [CertManager](https://github.com/cert-manager/cert-manager) is used to manage certificates in the cluster, use the following command to generate an installation file or install it directly:

```shell
# Generate installation files
./juicefs-csi-webhook-install.sh print --with-certmanager > juicefs-csi-sidecar.yaml
kubectl apply -f ./juicefs-csi-sidecar.yaml

# Directly install
./juicefs-csi-webhook-install.sh install --with-certmanager
```

If you had to use this installation method in a production environment, be sure to include the generated `juicefs-csi-sidecar.yaml` into source code management, so that you can track any future config modifications.

## Install in by-process mode {#by-process}

In the process mount mode, the JuiceFS client no longer runs in a separate Pod, but runs in the CSI Node Service container. All JuiceFS PVs that need to be mounted will be mounted in the CSI Node Service container in process mode. For more details, please refer to the [Process Mount Mode](./introduction.md#by-process).

### Helm

Modify your config file, e.g. `values-mycluster.yaml`:

```YAML title='values-mycluster.yaml'
mountMode: process
```

Reinstall to apply:

```shell
helm upgrade --install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values-mycluster.yaml
```

### kubectl

To enable mount by process, add `--by-process=true` to CSI Node Service and CSI Controller startup command.

## Installing in ARM64 {#arm64}

From v0.11.1 and above, JuiceFS CSI Driver supports using container images in the ARM64 environment, if you are faced with an ARM64 cluster, you need to change some image tags before installation. No other steps are required for ARM64 environments.

Images that need to replaced is listed below, find our the suitable version for your cluster via the links:

| Original container image name              | New container image name                                                                                                                       |
|--------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------|
| `quay.io/k8scsi/livenessprobe`             | [`registry.k8s.io/sig-storage/livenessprobe`](https://kubernetes-csi.github.io/docs/livenessprobe.html#supported-versions)                     |
| `quay.io/k8scsi/csi-provisioner`           | [`registry.k8s.io/sig-storage/csi-provisioner`](https://kubernetes-csi.github.io/docs/external-provisioner.html#supported-versions)            |
| `quay.io/k8scsi/csi-node-driver-registrar` | [`registry.k8s.io/sig-storage/csi-node-driver-registrar`](https://kubernetes-csi.github.io/docs/node-driver-registrar.html#supported-versions) |
| `quay.io/k8scsi/csi-resizer:`              | [`registry.k8s.io/sig-storage/csi-resizer`](https://kubernetes-csi.github.io/docs/external-resizer.html#supported-versions)                    |

### Helm

Add `sidecars` to your installation config, e.g. `values-mycluster.yaml`, and overwrite selected images:

```yaml
sidecars:
  livenessProbeImage:
    repository: registry.k8s.io/sig-storage/livenessprobe
    tag: "v2.6.0"
  csiProvisionerImage:
    repository: registry.k8s.io/sig-storage/csi-provisioner
    tag: "v2.2.2"
  nodeDriverRegistrarImage:
    repository: registry.k8s.io/sig-storage/csi-node-driver-registrar
    tag: "v2.5.0"
  csiResizerImage:
    repository: registry.k8s.io/sig-storage/csi-resizer
    tag: "v1.8.0"
```

### kubectl

Replace some container images and parameters of `provisioner` sidecar in `k8s.yaml` (use [gnu-sed](https://formulae.brew.sh/formula/gnu-sed) instead under macOS):

```shell
sed --in-place --expression='s@quay.io/k8scsi/livenessprobe:v1.1.0@registry.k8s.io/sig-storage/livenessprobe:v2.6.0@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-provisioner:v1.6.0@registry.k8s.io/sig-storage/csi-provisioner:v2.2.2@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.5.0@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-resizer:v1.0.1@registry.k8s.io/sig-storage/csi-resizer:v1.8.0@' k8s.yaml
sed --in-place --expression='s@enable-leader-election@leader-election@' k8s.yaml
```

## Uninstall

Uninstalling is the reverse operation of installation. For Helm-installed applications, you can execute the following command:

```shell
helm uninstall juicefs-csi-driver
```

If you used the kubectl installation method, you just need to replace the `apply` with `delete` in the corresponding installation command. For example:

```shell
kubectl delete -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
```
