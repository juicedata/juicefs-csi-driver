---
sidebar_label: Quick Start Guide
---

# Quick Start Guide

## Prerequisites

- Kubernetes 1.14 and above

## Installation

There are two ways to install JuiceFS CSI Driver.

### 1. Install via Helm

#### Prerequisites

- Helm 3.1.0 and above

#### Install Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

To install Helm, refer to the [Helm Installation Guide](https://helm.sh/docs/intro/install) and ensure that the `helm` binary is in the `PATH` of your shell.

#### Using Helm To Deploy

1. Prepare a YAML file

   :::info
   If you do not need to create a StorageClass when installing the CSI driver, you can ignore this step.
   :::

   Create a configuration file (e.g. `values.yaml`), copy and complete the following configuration information. Currently only the basic configurations are listed. For more configurations supported by Helm chart of JuiceFS CSI Driver, please refer to [document](https://github.com/juicedata/charts/blob/main/charts/juicefs-csi-driver/README.md#values), unneeded items can be deleted, or their values ​​can be left blank.

   Here is an example of the community edition:

   ```yaml title="values.yaml"
   storageClasses:
   - name: juicefs-sc
     enabled: true
     reclaimPolicy: Retain
     backend:
       name: "<name>"               # JuiceFS volume name
       metaurl: "<meta-url>"        # URL of metadata engine
       storage: "<storage-type>"    # Object storage type (e.g. s3, gcs, oss, cos)
       accessKey: "<access-key>"    # Access Key for object storage
       secretKey: "<secret-key>"    # Secret Key for object storage
       bucket: "<bucket>"           # A bucket URL to store data
       # If you need to set the time zone of the JuiceFS Mount Pod, please uncomment the next line, the default is UTC time.
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

   Among them, the `backend` part is the information related to the JuiceFS file system. If you are using a JuiceFS volume that has been created, you only need to fill in the two items `name` and `metaurl`. For more details on how to use StorageClass, please refer to the document: [Dynamic Provisioning](./examples/dynamic-provisioning.md).

2. Check and update kubelet root directory

   Execute the following command.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   If the result is not empty, it means that the root directory (`--root-dir`) of kubelet is not the default value (`/var/lib/kubelet`) and you need to set `kubeletDir` to the current root directly of kubelet in the configuration file `values.yaml` prepared in the first step.

   ```yaml title="values.yaml"
   kubeletDir: <kubelet-dir>
   ```

3. Deploy

   Execute the following three commands in sequence to deploy the JuiceFS CSI Driver through Helm. If the Helm configuration file is not prepared, you can omit the last `-f ./values.yaml` option when executing the `helm install` command.

   ```sh
   helm repo add juicefs https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

4. Verify installation

   The installation will launch a `StatefulSet` named `juicefs-csi-controller` with replica `1` and a `DaemonSet` named `juicefs-csi-node`, so run `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` should see `n+1` (where `n` is the number of worker nodes of the Kubernetes cluster) pods is running. For example:

   ```sh
   $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
   NAME                       READY   STATUS    RESTARTS   AGE
   juicefs-csi-controller-0   3/3     Running   0          22m
   juicefs-csi-node-v9tzb     3/3     Running   0          14m
   ```

### 2. Install via kubectl

Since Kubernetes will deprecate some old APIs when a new version is released, you need to choose the appropriate deployment configuration file.

1. Check the root directory path of kubelet

   Execute the following command on any non-Master node in the Kubernetes cluster.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. Deploy

   - **If the check command returns a non-empty result**, it means that the root directory (`--root-dir`) of the kubelet is not the default (`/var/lib/kubelet`), so you need to update the `kubeletDir` path in the CSI Driver's deployment file and deploy.

     :::note
     Please replace `{{KUBELET_DIR}}` in the below command with the actual root directory path of kubelet.
     :::

     ```shell
     # Kubernetes version >= v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

     ```shell
     # Kubernetes version < v1.18
     curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
     ```

   - **If the check command returns an empty result**, you can deploy directly without modifying the configuration:

     ```shell
     # Kubernetes version >= v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml
     ```

     ```shell
     # Kubernetes version < v1.18
     kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
     ```

## Usage

Please refer to the "User Guide" category on the left sidebar.

## Troubleshooting & FAQs

If you encounter any issue, please refer to [Troubleshooting](troubleshooting.md) or [FAQ](FAQs.md) document.

## Upgrade CSI Driver

Refer to [Upgrade CSI Driver](upgrade/upgrade-csi-driver.md) document.

## Known issues

- JuiceFS CSI Driver v0.10.0 and above does not support wildcards in `--cache-dir` mount option
