---
sidebar_label: Install in arm64
---

# How to install JuiceFS CSI Driver in arm64 environment

JuiceFS CSI Driver supports image of the arm64 environment in v0.11.1 and later version. There are two ways to install JuiceFS CSI Driver.

## 1. Install via Helm

### Prerequisites

- Helm 3.1.0+

### Install Helm

Helm is a tool for managing Kubernetes charts. Charts are packages of pre-configured Kubernetes resources.

To install Helm, refer to the [Helm install guide](https://github.com/helm/helm#install) and ensure that the `helm` binary is in the `PATH` of your shell.

### Using Helm To Deploy

1. Prepare a YAML file

    Create a configuration file, for example: `values.yaml`, copy and complete the following configuration information. Among them, the `backend` part is the information related to the JuiceFS file system, you can refer to [JuiceFS Quick Start Guide](https://github.com/juicedata/juicefs/blob/main/docs/zh_cn/quick_start_guide.md) for more information. If you are using a JuiceFS volume that has been created, you only need to fill in the two items `name` and `metaurl`. The `mountPod` part can specify CPU/memory limits and requests of mount pod for pods using this driver. Unneeded items should be deleted, or its value should be left blank.

    ```yaml
    sidecars:
      livenessProbeImage:
        repository: k8s.gcr.io/sig-storage/livenessprobe
        tag: "v2.2.0"
      nodeDriverRegistrarImage:
        repository: k8s.gcr.io/sig-storage/csi-node-driver-registrar
        tag: "v2.0.1"
      csiProvisionerImage:
        repository: k8s.gcr.io/sig-storage/csi-provisioner
        tag: "v2.0.2"
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
   $ ps -ef | grep kubelet | grep root-dir
   ```

   If the result is not empty, it means that the root directory (`--root-dir`) of kubelet is not the default value (`/var/lib/kubelet`) and you need to set `kubeletDir` to the current root directly of kubelet in the configuration file `values.yaml` prepared in the first step.

   ```yaml
   kubeletDir: <kubelet-dir>
   ```

3. Deploy

   ```sh
   helm repo add juicefs-csi-driver https://juicedata.github.io/juicefs-csi-driver/
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

    - **Check storage class**: `kubectl get sc juicefs-sc` will show the storage class like this:

      ```
      NAME         PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
      juicefs-sc   csi.juicefs.com   Retain          Immediate           false                  69m
      ```
      
## 2. Install via kubectl

Since Kubernetes will deprecate some old APIs when a new version is released, you need to choose the appropriate deployment configuration file.

1. Check the root directory path of `kubelet`.

    Execute the following command on any non-Master node in the Kubernetes cluster.

    ```shell
    $ ps -ef | grep kubelet | grep root-dir
    ```

2. Deploy

**If the check command returns a non-empty result**, it means that the root directory (`--root-dir`) of the kubelet is not the default (`/var/lib/kubelet`), so you need to update the `kubeletDir` path in the CSI Driver's deployment file and deploy.

    ```shell
    # Kubernetes version >= v1.18
    curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | \
    sed -e 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' \
    -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
    -e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
    -e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -

    # Kubernetes version < v1.18
    curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | \
    sed -e 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' \
    -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
    -e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
    -e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -
    ```

:::note
Please replace `{{KUBELET_DIR}}` in the above command with the actual root directory path of kubelet.
:::

**If the check command returns an empty result**, you can deploy directly without modifying the configuration:

    ```shell
    # Kubernetes version >= v1.18
    curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | \
    sed -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
    -e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
    -e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -

    # Kubernetes version < v1.18
    curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | \
    sed -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
    -e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
    -e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | kubectl apply -f -
    ```
