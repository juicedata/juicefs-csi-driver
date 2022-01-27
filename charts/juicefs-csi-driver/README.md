# juicefs-csi-driver

![Version: 0.8.1](https://img.shields.io/badge/Version-0.8.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.12.0](https://img.shields.io/badge/AppVersion-0.12.0-informational?style=flat-square)

A Helm chart for JuiceFS CSI Driver

**Homepage:** <https://github.com/juicedata/juicefs-csi-driver>

## Source Code

* <https://github.com/juicedata/juicefs-csi-driver>

## Requirements

Kubernetes: `>=1.14.0-0`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controller.affinity | object | Hard node and soft zone anti-affinity | Affinity for controller pods. |
| controller.enabled | bool | `true` |  |
| controller.nodeSelector | object | `{}` | Node selector for controller pods |
| controller.replicas | int | `1` |  |
| controller.resources.limits.cpu | string | `"1000m"` |  |
| controller.resources.limits.memory | string | `"1Gi"` |  |
| controller.resources.requests.cpu | string | `"100m"` |  |
| controller.resources.requests.memory | string | `"512Mi"` |  |
| controller.service.port | int | `9909` |  |
| controller.service.trpe | string | `"ClusterIP"` |  |
| controller.terminationGracePeriodSeconds | int | `30` | Grace period to allow the controller to shutdown before it is killed |
| controller.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations for controller pods |
| dnsConfig | object | `{}` |  |
| dnsPolicy | string | `"ClusterFirstWithHostNet"` |  |
| hostAliases | list | `[]` |  |
| image.pullPolicy | string | `""` |  |
| image.repository | string | `"juicedata/juicefs-csi-driver"` |  |
| image.tag | string | `"v0.10.5"` |  |
| jfsConfigDir | string | `"/var/lib/juicefs/config"` |  |
| jfsMountDir | string | `"/var/lib/juicefs/volume"` | juicefs mount dir |
| jfsMountPriority | object | `{"enable":true,"name":"juicefs-mount-critical"}` | juicefs mount pod priority |
| kubeletDir | string | `"/var/lib/kubelet"` | kubelet working directory,can be set using `--root-dir` when starting kubelet |
| namespace | string | `"kube-system"` |  |
| node.affinity | object | Hard node and soft zone anti-affinity | Affinity for node pods. |
| node.enabled | bool | `true` |  |
| node.hostNetwork | bool | `false` |  |
| node.nodeSelector | object | `{}` | Node selector for node pods |
| node.resources.limits.cpu | string | `"2000m"` |  |
| node.resources.limits.memory | string | `"5Gi"` |  |
| node.resources.requests.cpu | string | `"1000m"` |  |
| node.resources.requests.memory | string | `"1Gi"` |  |
| node.terminationGracePeriodSeconds | int | `30` | Grace period to allow the node pod to shutdown before it is killed |
| node.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations for node pods |
| serviceAccount.controller.annotations | object | `{}` |  |
| serviceAccount.controller.create | bool | `true` |  |
| serviceAccount.controller.name | string | `"juicefs-csi-controller-sa"` |  |
| serviceAccount.node.create | bool | `true` |  |
| serviceAccount.node.name | string | `"juicefs-csi-node-sa"` |  |
| sidecars.csiProvisionerImage.repository | string | `"quay.io/k8scsi/csi-provisioner"` |  |
| sidecars.csiProvisionerImage.tag | string | `"v1.6.0"` |  |
| sidecars.livenessProbeImage.repository | string | `"quay.io/k8scsi/livenessprobe"` |  |
| sidecars.livenessProbeImage.tag | string | `"v1.1.0"` |  |
| sidecars.nodeDriverRegistrarImage.repository | string | `"quay.io/k8scsi/csi-node-driver-registrar"` |  |
| sidecars.nodeDriverRegistrarImage.tag | string | `"v1.1.0"` |  |
| storageClasses[0].backend.accessKey | string | `""` | Access key for object storage |
| storageClasses[0].backend.bucket | string | `""` | Bucket URL. Read [this document](https://github.com/juicedata/juicefs/blob/main/docs/en/how_to_setup_object_storage.md) to learn how to setup different object storage. |
| storageClasses[0].backend.metaurl | string | `""` | Connection URL for metadata engine (e.g. Redis). Read [this document](https://github.com/juicedata/juicefs/blob/main/docs/en/databases_for_metadata.md) for more information. |
| storageClasses[0].backend.name | string | `"juice"` | The JuiceFS file system name. |
| storageClasses[0].backend.secretKey | string | `""` | Secret key for object storage |
| storageClasses[0].backend.storage | string | `""` | Object storage type, such as `s3`, `gs`, `oss`. Read [this document](https://github.com/juicedata/juicefs/blob/main/docs/en/how_to_setup_object_storage.md) for the full supported list. |
| storageClasses[0].enabled | bool | `true` | Default is true will create a new StorageClass. It will create Secret and StorageClass used by CSI driver |
| storageClasses[0].mountPod.resources.limits.cpu | string | `"5000m"` |  |
| storageClasses[0].mountPod.resources.limits.memory | string | `"5Gi"` |  |
| storageClasses[0].mountPod.resources.requests.cpu | string | `"1000m"` |  |
| storageClasses[0].mountPod.resources.requests.memory | string | `"1Gi"` |  |
| storageClasses[0].name | string | `"juicefs-sc"` |  |
| storageClasses[0].reclaimPolicy | string | `"Delete"` | Either Delete or Retain. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.5.0](https://github.com/norwoodj/helm-docs/releases/v1.5.0)
