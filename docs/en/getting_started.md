---
title: Installation
---

JuiceFS CSI Driver requires Kubernetes 1.14 and above, follow below steps to install.

## Helm

In comparison to kubectl, Helm allows you to manage CSI Driver resources as a whole, and also makes it easier to modify configurations, or enable advanced features. Overall, Helm is recommended over kubectl.

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

Installation requires Helm 3.1.0 and above, refer to the [Helm Installation Guide](https://helm.sh/docs/intro/install).

1. Download the Helm chart for JuiceFS CSI Driver

  ```shell
  helm repo add juicefs https://juicedata.github.io/charts/
  helm repo update
  helm fetch --untar juicefs/juicefs-csi-driver
  cd juicefs-csi-driver
  # Installation configurations is included in values.yaml, review this file and modify to your needs
  cat values.yaml
  ```

1. Check kubelet root directory

   Execute the following command.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   If the result is not empty or the default `/var/lib/kubelet`, it means that the kubelet root directory is customized, you need to set `kubeletDir` to the current kubelet root directly in `values.yaml`.

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

1. Deploy

   Execute below commands to deploy JuiceFS CSI Driver:

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

1. Verify installation

   Verify all CSI Driver components are running:

   ```shell
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

   Learn about JuiceFS CSI Driver architecture, and components functionality in [Introduction](./introduction.md).

1. We recommend that you include the CSI Driver Helm chart in the Git repository, so that any changes to `values.yaml` can be restored.

## kubectl

If installed using kubectl, any configuration changes require manual editing, and can easily cause trouble if you are not familiar with CSI Controller. If you'd like to enable advanced features (e.g. [enable pathPattern](./guide/pv.md#using-path-pattern)), or just want to manage resources easier, consider installing via Helm.

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

## Verify installation

Verify all CSI components are up and running:

```shell
$ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          22m
juicefs-csi-node-v9tzb     3/3     Running   0          14m
```

CSI Node Service is a DaemonSet, and by default runs on all Kubernetes worker nodes, so CSI Node pod amount should match node count. If the numbers don't match, check if any of the nodes are tainted, and resolve them according to the actual situation. You can also [run CSI Node Service on selected nodes](./guide/resource-optimization.md#csi-node-node-selector).

Learn about JuiceFS CSI Driver architecture, and components functionality in [Introduction](./introduction.md#architecture).

## ARM64 caveats

From v0.11.1 and above, JuiceFS CSI Driver supports using container images in the ARM64 environment, if you are faced with an ARM64 cluster, you need to change some image tags before installation. No other steps are required for ARM64 environments.

Images that need to replaced is listed below, find our the suitable version for your cluster via the links:

| Original container image name              | New container image name                                                                                                                  |
|--------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| `quay.io/k8scsi/livenessprobe`             | [`k8s.gcr.io/sig-storage/livenessprobe`](https://kubernetes-csi.github.io/docs/livenessprobe.html#supported-versions)                     |
| `quay.io/k8scsi/csi-provisioner`           | [`k8s.gcr.io/sig-storage/csi-provisioner`](https://kubernetes-csi.github.io/docs/external-provisioner.html#supported-versions)            |
| `quay.io/k8scsi/csi-node-driver-registrar` | [`k8s.gcr.io/sig-storage/csi-node-driver-registrar`](https://kubernetes-csi.github.io/docs/node-driver-registrar.html#supported-versions) |

### Helm

Add `sidecars` to `values.yaml`, to overwrite selected images:

```yaml
sidecars:
  livenessProbeImage:
    repository: k8s.gcr.io/sig-storage/livenessprobe
    tag: "v2.6.0"
  csiProvisionerImage:
    repository: k8s.gcr.io/sig-storage/csi-provisioner
    tag: "v2.2.2"
  nodeDriverRegistrarImage:
    repository: k8s.gcr.io/sig-storage/csi-node-driver-registrar
    tag: "v2.5.0"
```

### kubectl

Replace some container images in `k8s.yaml` (use [gnu-sed](https://formulae.brew.sh/formula/gnu-sed) instead under macOS):

```shell
sed --in-place --expression='s@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.6.0@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.2.2@' k8s.yaml
sed --in-place --expression='s@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.5.0@' k8s.yaml
```
