---
title: Introduction
---

## Architecture {#architecture}

[JuiceFS CSI Driver](https://github.com/juicedata/juicefs-csi-driver) implements the [CSI specification](https://github.com/container-storage-interface/spec/blob/master/spec.md), allowing JuiceFS to be integrated with container orchestration systems. Under Kubernetes, JuiceFS can provide storage service to pods via PersistentVolume.

JuiceFS CSI Driver consists of JuiceFS CSI Controller (StatefulSet) and JuiceFS CSI Node Service (DaemonSet), they can be viewed using `kubectl`:

```shell
$ kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-csi-driver
NAME                       READY   STATUS        RESTARTS   AGE
juicefs-csi-controller-0   2/2     Running       0          141d
juicefs-csi-node-8rd96     3/3     Running       0          141d
```

The architecture of the JuiceFS CSI Driver is shown in the figure:

![](./images/csi-driver-architecture.svg)

As shown in above diagram, JuiceFS CSI Driver run JuiceFS Client in a dedicated mount pod, CSI Node Service will manage mount pod lifecycle. This architecture proves several advantages:

* When multiple pods reference a same PV, mount pod will be reused. There'll be reference counting on mount pod to decide its deletion.
* Components are decoupled from application pods, allowing CSI Driver to be easily upgraded, see [Upgrade JuiceFS CSI Driver](./administration/upgrade-csi-driver.md).

On the same node, a PVC corresponds to a Mount Pod. The relationship between PVC, PV, and Mount Pod is shown in the following figure:

![](./images/mount-pod-architecture.svg)

## Usage {#usage}

To use JuiceFS CSI Driver, you can create and manage a PersistentVolume (PV) via ["Static Provisioning"](./guide/pv.md#static-provisioning) or ["Dynamic Provisioning"](./guide/pv.md#dynamic-provisioning). Under static provisioning, you're in charge of creating the PersistentVolume (PV) and PersistentVolumeClaim (PVC), and reference that PVC in pod definition, similar to [Configure a Pod to Use a PersistentVolume for Storage](https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage). Use static provisioning when:

* You already have large amount of data stored in JuiceFS, and wish to access directly inside Kubernetes pods.
* You are evaluating JuiceFS CSI Driver functionalities.

Managing PVs can be wearisome, when using CSI Driver at scale, it's recommended to create PV dynamically via ["Dynamic Provisioning"](./guide/pv.md#dynamic-provisioning): create a PVC and reference it in pod definition, JuiceFS CSI Driver will create the corresponding PV for you. The overall process:

* User creates a PVC using the JuiceFS StorageClass;
* PV is created and provisioned by CSI Controller, by default, a sub-directory named with PV ID will be created under JuiceFS root;
* Kubernetes PV Controller binds user-created PVC with the PV created by CSI Controller, PVC and PV both enter "Bound" state;
* User creates application pod, referencing PVC previously created;
* CSI Node Service creates mount pod on the associating node;
* A JuiceFS Client runs inside the mount pod, and mounts JuiceFS volume to host, path being `/var/lib/juicefs/volume/[pv-name]`;
* CSI Node Service waits until mount pod is up and running, and binds PV with the associated container, the PV sub-directory is mounted in pod, path defined by `volumeMounts`;
* Application pod is created by Kubelet.

As explained above, when using JuiceFS CSI Driver, application pod is always accompanied by a mount pod:

```
default       app-web-xxx            1/1     Running        0            1d
kube-system   juicefs-host-pvc-xxx   1/1     Running        0            1d
```

## Sidecar mode {#sidecar}

Mount pod is created by CSI Node, due to CSI Node being a DaemonSet component, if your Kubernetes cluster does not allow DaemonSets (like some Serverless Kubernetes platform), CSI Node will not be able to install, and JuiceFS CSI Driver cannot be used properly.

For situations like this, you can choose to run CSI Driver in Sidecar mode, which runs JuiceFS Client in Sidecar containers. In this mode, CSI Node is no longer needed, only CSI Controller is installed. For Kubernetes namespaces that need to use CSI Driver, CSI Controller will listen for pod changes, check if JuiceFS PVC is used, and inject Sidecar container accordingly.

![](./images/sidecar-architecture.svg)

Some Sidecar caveats:

* Different from mount by pod, a Sidecar container is injected into the application pod, so sharing PV is not possible. Carefully manage resources when use at scale.
* CSI Controller will listen for all pod change events under namespaces with Sidecar injections enabled. If you'd like to minimize overhead, you can even ignore pods by labeling them with `disable.sidecar.juicefs.com/inject: true`, so that CSI Controller deliberately ignores them.

To use Sidecar mode, [install CSI Driver in Sidecar mode](./getting_started.md#sidecar).

## Mount by process {#by-process}

Apart from using a dedicated mount pod (mount by pod), JuiceFS CSI Driver also supports running JuiceFS Client directly inside CSI Node Service, as processes (mount by process). In this mode, one or several JuiceFS Clients will run inside the CSI Node Service pod, managing all JuiceFS mount points for application pods referencing JuiceFS PV in the associating node.

When all JuiceFS Client run inside CSI Node Service pod, it's not hard to imagine that CSI Node Service will be needing more resource. It's recommended to increase resource requests to 1 CPU and 1GiB Memory, limits to 2 CPU and 5GiB Memory, or adjust according to the actual resource usage.

In Kubernetes, mount by pod is no doubt the more recommended way to use JuiceFS CSI Driver. But outside the Kubernetes world, there'll be scenarios requiring the mount by process mode, for example, [Use JuiceFS CSI Driver in Nomad](./cookbook/csi-in-nomad.md).

For versions before v0.10.0, JuiceFS CSI Driver only supports mount by process. For v0.10.0 and above, mount by pod is the default behavior. To upgrade from v0.9 to v0.10, refer to [Upgrade under mount by process mode](./administration/upgrade-csi-driver.md#mount-by-process-upgrade).

To use mount by process mode, [install CSI Driver in by-process mode](./getting_started.md#by-process).
