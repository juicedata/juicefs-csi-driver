---
sidebar_label: Introduction
---

# JuiceFS CSI Driver

The [JuiceFS CSI Driver](https://github.com/juicedata/juicefs-csi-driver) implements the [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) specification for container orchestrators to manage the lifecycle of JuiceFS file system.

## Prerequisites

- Kubernetes 1.14+

## Installation

There are two ways to install JuiceFS CSI Driver.

### 1. Install via Helm

#### Prerequisites

- Helm 3.1.0+

#### Install Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

To install Helm, refer to the [Helm Installation Guide](https://helm.sh/docs/intro/install) and ensure that the `helm` binary is in the `PATH` of your shell.

#### Using Helm To Deploy

1. Prepare a YAML file

   Create a configuration file, for example: `values.yaml`, copy and complete the following configuration information. Among them, the `backend` part is the information related to the JuiceFS file system,
   you can refer to ["JuiceFS Quick Start Guide"](https://juicefs.com/docs/community/quick_start_guide) for more information. If you are using a JuiceFS volume that has been created, you only need to fill in the two items `name` and `metaurl`.
   The `mountPod` part can specify CPU/memory limits and requests of mount pod for pods using this driver. Unneeded items should be deleted, or its value should be left blank. Take Community edition as an example:

   ```yaml title="values.yaml"
   storageClasses:
   - name: juicefs-sc
     enabled: true
     reclaimPolicy: Retain
     backend:
       name: "<name>"
       metaurl: "<meta-url>"
       storage: "<storage-type>"
       accessKey: "<access-key>"
       secretKey: "<secret-key>"
       bucket: "<bucket>"
     mountPod:
       resources:
         limits:
           cpu: "<cpu-limit>"
           memory: "<memory-limit>"
         requests:
           cpu: "<cpu-request>"
           memory: "<memory-request>"
   ```

2. Check and update kubelet root directory

   Execute the following command.

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   If the result is not empty, it means that the root directory (`--root-dir`) of kubelet is not the default value (`/var/lib/kubelet`) and you need to set `kubeletDir` to the current root directly of kubelet in the configuration file `values.yaml` prepared in the first step.

   ```yaml
   kubeletDir: <kubelet-dir>
   ```

3. Deploy

   ```sh
   helm repo add juicefs-csi-driver https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

4. Check the deployment

   - **Check pods are running**: the deployment will launch a `StatefulSet` named `juicefs-csi-controller` with replica `1` and a `DaemonSet` named `juicefs-csi-node`, so run `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` should see `n+1` (where `n` is the number of worker nodes of the Kubernetes cluster) pods is running. For example:

     ```sh
     $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
     NAME                       READY   STATUS    RESTARTS   AGE
     juicefs-csi-controller-0   3/3     Running   0          22m
     juicefs-csi-node-v9tzb     3/3     Running   0          14m
     ```

   - **Check secret**: `kubectl -n kube-system describe secret juicefs-sc-secret` will show the secret with above `backend` fields in `values.yaml`:

     ```
     Name:         juicefs-sc-secret
     Namespace:    kube-system
     Labels:       app.kubernetes.io/instance=juicefs-csi-driver
                   app.kubernetes.io/managed-by=Helm
                   app.kubernetes.io/name=juicefs-csi-driver
                   app.kubernetes.io/version=0.7.0
                   helm.sh/chart=juicefs-csi-driver-0.1.0
     Annotations:  meta.helm.sh/release-name: juicefs-csi-driver
                   meta.helm.sh/release-namespace: default

     Type:  Opaque

     Data
     ====
     access-key:  0 bytes
     bucket:      47 bytes
     metaurl:     54 bytes
     name:        4 bytes
     secret-key:  0 bytes
     storage:     2 bytes
     ```

   - **Check StorageClass**: `kubectl get sc juicefs-sc` will show the StorageClass like this:

     ```
     NAME         PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
     juicefs-sc   csi.juicefs.com   Retain          Immediate           false                  69m
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

Before getting started, you need:

* Get yourself familiar with how to setup Kubernetes and how to use JuiceFS file system.
* Make sure JuiceFS is accessible from Kuberenetes cluster. It is recommended to create the file system inside the same region as Kubernetes cluster.
* Install JuiceFS CSI driver following the [Installation](#installation) steps.

### Table of contents

* [Static Provisioning](examples/static-provisioning.md)
* [Dynamic Provisioning](examples/dynamic-provisioning.md)
* [Config File System Settings](examples/format-options.md)
* [Config Mount Options](examples/mount-options.md)
* [Mount Subdirectory](examples/subpath.md)
* [Data Encryption](examples/encrypt.md)
* [Manage Permissions in JuiceFS](examples/permission.md)
* [Use ReadWriteMany and ReadOnlyMany](examples/rwx-and-rox.md)
* [Config Mount Pod Resources](examples/mount-resources.md)
* [Set Configuration Files and Environment Variables in Mount Pod](examples/config-and-env.md)
* [Delay Deletion of Mount Pod](examples/delay-delete.md)
* [Configure Mount Pod to Clean Cache When Exiting](examples/cache-clean.md)
* [Reclaim Policy of PV](examples/reclaim-policy.md)
* [Automatic Mount Point Recovery](recover_failed_mountpoint.md)

## Troubleshooting & FAQs

If you encounter any issue, please refer to [Troubleshooting](troubleshooting.md) or [FAQ](faq) document.

## Upgrade CSI Driver

Refer to [Upgrade CSI Driver](upgrade-csi-driver.md) document.

## Known issues

- JuiceFS CSI Driver v0.10.0 and above does not support wildcards in `--cache-dir` mount option
