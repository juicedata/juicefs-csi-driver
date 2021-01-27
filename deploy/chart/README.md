# juicefs-csi

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.6.1](https://img.shields.io/badge/AppVersion-0.6.1-informational?style=flat-square)

A Helm chart for JuiceFS CSI Driver

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| backend.accessKey | string | `""` |  |
| backend.bucket | string | `""` |  |
| backend.metaurl | string | `""` |  |
| backend.name | string | `"juice"` |  |
| backend.secretKey | string | `""` |  |
| backend.storage | string | `""` |  |
| controller.affinity | object | Hard node and soft zone anti-affinity | Affinity for gateway pods. Passed through `tpl` and, thus, to be configured as string |
| controller.attacher.pullPolicy | string | `"IfNotPresent"` |  |
| controller.attacher.registry | string | `"quay.io"` | The Docker registry |
| controller.attacher.repository | string | `"k8scsi/csi-attacher"` | Docker image repository |
| controller.attacher.resources | object | `{}` | Resource requests and limits for the gateway |
| controller.attacher.tag | string | `"v1.1.0"` | Overrides the image tag whose default is the chart's appVersion |
| controller.enabled | bool | `true` |  |
| controller.livenessComponent.pullPolicy | string | `"IfNotPresent"` |  |
| controller.livenessComponent.registry | string | `"quay.io"` | The Docker registry |
| controller.livenessComponent.repository | string | `"k8scsi/livenessprobe"` | Docker image repository |
| controller.livenessComponent.resources | object | `{}` | Resource requests and limits for the gateway |
| controller.livenessComponent.tag | string | `"v1.1.0"` | Overrides the image tag whose default is the chart's appVersion |
| controller.nodeSelector | object | `{}` | Node selector for gateway pods |
| controller.provisioner.pullPolicy | string | `"IfNotPresent"` |  |
| controller.provisioner.registry | string | `"quay.io"` | The Docker registry |
| controller.provisioner.repository | string | `"k8scsi/csi-provisioner"` | Docker image repository |
| controller.provisioner.resources | object | `{}` | Resource requests and limits for the gateway |
| controller.provisioner.tag | string | `"v1.6.0"` | Overrides the image tag whose default is the chart's appVersion |
| controller.pullPolicy | string | `"IfNotPresent"` |  |
| controller.registry | string | `"docker.io"` | The Docker registry |
| controller.replicas | int | `1` |  |
| controller.repository | string | `"juicedata/juicefs-csi-driver"` | Docker image repository |
| controller.service.port | int | `9909` |  |
| controller.service.trpe | string | `"ClusterIP"` |  |
| controller.sresources | object | `{}` | Resource requests and limits for the gateway |
| controller.tag | string | `nil` | Overrides the image tag whose default is the chart's appVersion |
| controller.terminationGracePeriodSeconds | int | `30` | Grace period to allow the gateway to shutdown before it is killed |
| controller.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations for gateway pods |
| driver.affinity | object | Hard node and soft zone anti-affinity | Affinity for gateway pods. Passed through `tpl` and, thus, to be configured as string |
| driver.enabled | bool | `true` |  |
| driver.livenessComponent.pullPolicy | string | `"IfNotPresent"` |  |
| driver.livenessComponent.registry | string | `"quay.io"` | The Docker registry |
| driver.livenessComponent.repository | string | `"k8scsi/livenessprobe"` | Docker image repository |
| driver.livenessComponent.resources | object | `{}` | Resource requests and limits for the gateway |
| driver.livenessComponent.tag | string | `"v1.1.0"` | Overrides the image tag whose default is the chart's appVersion |
| driver.nodeSelector | object | `{}` | Node selector for gateway pods |
| driver.pullPolicy | string | `"IfNotPresent"` |  |
| driver.registrarComponent.pullPolicy | string | `"IfNotPresent"` |  |
| driver.registrarComponent.registry | string | `"quay.io"` | The Docker registry |
| driver.registrarComponent.repository | string | `"k8scsi/csi-node-driver-registrar"` | Docker image repository |
| driver.registrarComponent.resources | object | `{}` | Resource requests and limits for the gateway |
| driver.registrarComponent.tag | string | `"v1.1.0"` | Overrides the image tag whose default is the chart's appVersion |
| driver.registry | string | `"docker.io"` | The Docker registry |
| driver.repository | string | `"juicedata/juicefs-csi-driver"` | Docker image repository |
| driver.resources | object | `{}` | Resource requests and limits for the gateway |
| driver.tag | string | `nil` | Overrides the image tag whose default is the chart's appVersion |
| driver.terminationGracePeriodSeconds | int | `30` | Grace period to allow the gateway to shutdown before it is killed |
| driver.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"}]` | Tolerations for gateway pods |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| storageClass.enabled | bool | `true` |  |
| storageClass.name | string | `"juicefs-sc"` |  |
| storageClass.provisioner | string | `"csi.juicefs.com"` |  |
| storageClass.reclaimPolicy | string | `"Delete"` |  |
| storageClass.volumeBindingMode | string | `"Immediate"` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.4.0](https://github.com/norwoodj/helm-docs/releases/v1.4.0)
