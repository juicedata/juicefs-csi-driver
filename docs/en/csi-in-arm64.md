---
sidebar_label: Install in ARM64 Environment
---

# How to install JuiceFS CSI Driver in ARM64 environment

JuiceFS CSI Driver only supports container images in the ARM64 environment at v0.11.1 and later, so please make sure you are using the correct version. Compared with the installation method in the ["Introduction"](introduction.md) document, the installation method in the ARM64 environment is slightly different. The different installation methods are introduced below.

## 1. Install via Helm

:::note
Please use Helm chart v0.7.1 and later to install
:::

The main difference between the installation in the ARM64 environment is ["Step 1 Prepare a YAML file"](introduction.md#using-helm-to-deploy), you need to add the `sidecars` configuration in the YAML file, the details are as follows:

```yaml {1-10}
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

For the subsequent installation steps, please follow the instructions in the ["Introduction"](introduction.md#1-install-via-helm) document.

## 2. Install via kubectl

The main difference in installation in the ARM64 environment is ["Step 2 Deploy"](introduction.md#2-install-via-kubectl), which requires replacing the image address of several sidecar containers. Assuming that the [`k8s.yaml`](https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml) file has been downloaded to the local directory, the specific commands are as follows:

```shell
cat ./k8s.yaml | \
sed -e 's@quay.io/k8scsi/csi-provisioner:v1.6.0@k8s.gcr.io/sig-storage/csi-provisioner:v2.0.2@' \
-e 's@quay.io/k8scsi/livenessprobe:v1.1.0@k8s.gcr.io/sig-storage/livenessprobe:v2.2.0@' \
-e 's@quay.io/k8scsi/csi-node-driver-registrar:v1.3.0@k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1@' | \
kubectl apply -f -
```

For other installation steps, please follow the instructions in the ["Introduction"](introduction.md#2-install-via-kubectl) document.
