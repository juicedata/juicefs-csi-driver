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

   Execute below commands to deploy JuiceFS CSI Driver:

   ```shell
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

3. Verify installation

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

3. Verify installation

   Verify all CSI Driver components are running:

   ```shell
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

   Learn about JuiceFS CSI Driver architecture, and components functionality in [Introduction](./introduction.md#architecture).
