# How JuiceFS CSI works

This article describes how the JuiceFS CSI Driver works in Kubernetes.

## Current implementation

The JuiceFS CSI Driver follows the [CSI spec v1.0](https://github.com/container-storage-interface/spec/blob/release-1.0/spec.md) implementation.

### Architecture

There are many different achitecture for CSI. This driver uses the typical one.

```txt
                             CO "Master" Host
+-------------------------------------------+
|                                           |
|  +------------+           +------------+  |
|  |     CO     |   gRPC    | Controller |  |
|  |            +----------->   Plugin   |  |
|  +------------+           +------------+  |
|                                           |
+-------------------------------------------+

                            CO "Node" Host(s)
+-------------------------------------------+
|                                           |
|  +------------+           +------------+  |
|  |     CO     |   gRPC    |    Node    |  |
|  |            +----------->   Plugin   |  |
|  +------------+           +------------+  |
|                                           |
+-------------------------------------------+
Figure 1: The Plugin runs on all nodes in the cluster: a centralized
Controller Plugin is available on the CO master host and the Node
Plugin is available on all of the CO Nodes.
```

* Controller Plugin in JuiceFS CSI driver is just a placeholder since no attach/detach is required for mounting JuiceFS.
* Node Plugin is controlled by a [DaemonSet](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/) which ensures that all Nodes run a copy of a JuiceFS CSI driver.

### Volume Life Cycle

The lifecycle of a volume in the JuiceFS CSI Driver is relatively simple, and the CSI currently implements the CreateVolume, DeleteVolume, [NodePublishVolume](https://github.com/container-storage-interface/spec/blob/v0.3.0/spec.md#nodepublishvolume) and [NodeUnpublishVolume](https://github.com/container-storage-interface/spec/blob/v0.3.0/spec.md#nodeunpublishvolume) interface. 

```txt
       +-+  +-+
       |X|  | |
       +++  +^+
        |    |
   Node |    | Node
Publish |    | Unpublish
 Volume |    | Volume
    +---v----+---+
    | PUBLISHED  |
    +------------+

Figure 2: Plugins may forego other lifecycle steps by contraindicating
them via the capabilities API. Interactions with the volumes of such
plugins is reduced to `NodePublishVolume` and `NodeUnpublishVolume`
calls.
```

### Communication with Kubernetes

> Kubernetes is as minimally prescriptive about packaging and deployment of a CSI Volume Driver as possible. See [Minimum Requirements (for Developing and Deploying a CSI driver for Kubernetes)](https://kubernetes-csi.github.io/docs/introduction.html#minimum-requirements-for-developing-and-deploying-a-csi-driver-for-kubernetes) for details.

JuiceFS CSI driver implements a minimal set of required gRPC calls to satisfy those requirements. It uses a Unix Domain Socket to interact with [Kubernetes CSI sidecar containers](https://kubernetes-csi.github.io/docs/sidecar-containers.html) which encapsulates all the Kubernetes specific code. So the CSI driver is actually orchestrator agnostic.

### Deploy in Kubernetes

The driver is deployed according to the recommended mechanism in [CSI Volume Plugins in Kubernetes Design Doc](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/storage/container-storage-interface.md#recommended-mechanism-for-deploying-csi-drivers-on-kubernetes)

![](images/container-storage-interface_diagram1.png)

Note that `external-attacher` and `external-provisioner` is not used in JuiceFS CSI driver.

### Registering driver

JuiceFS CSI driver is registed using [node-driver-registrar](https://kubernetes-csi.github.io/docs/node-driver-registrar.html#csi-node-driver-registrar) sidecar container.

When `--mode=node-register`, driver is registered to kubelet on nodes. This is how JuiceFS CSI driver is registered.

The sidecar container fetches driver information (using NodeGetInfo) from a CSI endpoint and registers it with the kubelet on that node.

```shell
$ journalctl -u kubelet -f
May 25 19:27:42 iZuf65o45s4xllq6ghmvkhZ kubelet[1458]: I0525 19:27:42.360149    1458 csi_plugin.go:111] kubernetes.io/csi: Trying to register a new plugin with name: csi.juicefs.com endpoint: /var/lib/kubelet/plugins/csi.juicefs.com/csi.sock versions: 0.2.0,0.3.0
May 25 19:27:42 iZuf65o45s4xllq6ghmvkhZ kubelet[1458]: I0525 19:27:42.360204    1458 csi_plugin.go:119] kubernetes.io/csi: Register new plugin with name: csi.juicefs.com at endpoint: /var/lib/kubelet/plugins/csi.juicefs.com/csi.sock
```

### Creating volume

Volume can be created before mounting or the first time mounting in Kubernetes, read [basic example](https://github.com/juicedata/juicefs-csi-driver/tree/master/examples/basic) for details.

Kubernetes will proceed the following actions:

* PV created
* PVC created
  * PV bound to PVC

### Using volume

JuiceFS CSI driver does not require attachment. The node plugin publishes volume when the Pod mounting the volume is placed (scheduled) on a node.

* Pod created
  * Pod placed (scheduled) on a node
    * When `--driver-requires-attachment=false`, volume attach will be skipped.
    * NodePublishVolume called

### End using volume

JuiceFS CSI driver does not require detachment either. The node plugin unpublishes volume when the Pod mounting the volume is removed (unscheduled) from a node.

* Pod deleted
  * NodeUnpublishVolume called
  * When `--driver-requires-attachment=false`, volume detach is not necessary.

### Deleting volume

> Deleting volume is out of scope of JuiceFS CSI driver.

Kubernetes will take the following actions when related resources are deleted:

* PVC deleted
  * PV unbound from PVC
    * PV deleted

The file system is NOT destroyed when PV is deleted. 

## Next in CSI spec v1.x

### Deprecated

```go
-  // NodeGetId is being deprecated in favor of NodeGetInfo and will be
-  // removed in CSI 1.0. Existing drivers, however, may depend on this
-  // RPC call and hence this RPC call MUST be implemented by the CSI
-  // plugin prior to v1.0.
...
-  // Prior to CSI 1.0 - CSI plugins MUST implement both NodeGetId and
-  // NodeGetInfo RPC calls.
```

### New features

* [Volume Expansion](https://github.com/container-storage-interface/spec/blob/master/spec.md#controllerexpandvolume): allow expansion of an online or offline volume.
* New message `VolumeSource` in `VolumeContentSource`: contains identity information for the existing source volume.
* New message `Confirmed` in `ValidateVolumeCapabilitiesResponse`: provides volume context validated by the plugin.
* New values for `ControllerServiceCapability`
  * `CLONE_VOLUME`: indicates the SP supports ControllerPublishVolume.readonly
  * `PUBLISH_READONLY`: indicates the SP supports ControllerPublishVolume.readonly field
  * `EXPAND_VOLUME`: see [VolumeExpansion] for details.
* New message `VolumeUsage` for `NodeGetVolumeStatsRequest` and `NodeGetVolumeStatsResponse`: provides stats of available/total/used of the specified volume.

### Changes

Some interface names changed, e.g. `GetId` => `GetVolumeId`, `GetVolumeAttributes` => `GetVolumeContext`, etc.

### Kubernetes Compatibility

[CSI is promoted to GA in Kubernetes v1.13](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/)

> * Kubernetes is now compatible with CSI spec v1.0 and v0.3 (instead of CSI spec v0.2).
>   * There were breaking changes between CSI spec v0.3.0 and v1.0.0, but Kubernetes v1.13 supports both versions so either version will work with Kubernetes v1.13.
>   * Please note that with the release of the CSI 1.0 API, support for CSI drivers using 0.3 and older releases of the CSI API is deprecated, and is planned to be removed in Kubernetes v1.15.
>   * There were no breaking changes between CSI spec v0.2 and v0.3, so v0.2 drivers should also work with Kubernetes v1.10.0+.
>   * There were breaking changes between the CSI spec v0.1 and v0.2, so very old drivers implementing CSI 0.1 must be updated to be at least 0.2 compatible before use with Kubernetes v1.10.0+.
