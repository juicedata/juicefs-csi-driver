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

By default, CSI Driver runs in mount pod mode, in which JuiceFS Client runs in a dedicated mount pod, like the architecture shown below:

![CSI-driver-architecture](./images/csi-driver-architecture.svg)

A dedicated mount pod, managed by CSI Node Service, such architecture proves several advantages:

* When multiple pods reference a same PV, mount pod will be reused. There'll be reference counting on mount pod to decide its deletion;
* Components are decoupled from application pods, allowing CSI Driver to be easily upgraded, see [Upgrade JuiceFS CSI Driver](./administration/upgrade-csi-driver.md).

On the same node, a PVC corresponds to a mount pod, while pods using the same PV may share a single mount pod. The relationship between different resources:

![mount-pod-architecture](./images/mount-pod-architecture.svg)

If mount pod mode doesn't suit you, check out [other mount modes](#other-mount-modes) provided by JuiceFS CSI Driver.

## Usage {#usage}

To use JuiceFS CSI Driver, you can create and manage a PersistentVolume (PV) via ["Static Provisioning"](./guide/pv.md#static-provisioning) or ["Dynamic Provisioning"](./guide/pv.md#dynamic-provisioning).

### Static provisioning

Static provisioning is the simpler approach, which by default mounts the whole JuiceFS volume root into application pod (also supports [mounting subdirectories](./guide/pv.md#mount-subdirectory)), the Kubernetes administrator is in charge of creating the PersistentVolume (PV) and [JuiceFS Volume Credentials](./guide/pv.md#volume-credentials) (stored as Kubernetes secret). After that, user will create a PVC binding that PV, and then finally use this PVC in application pod definition. The relationship between different resources:

![static-provisioning](./images/static-provisioning.svg)

Use static provisioning when:

* You already have large amount of data stored in JuiceFS, and wish to access directly inside Kubernetes pods;
* You are evaluating JuiceFS CSI Driver functionalities.

### Dynamic provisioning

Managing PVs can be wearisome, when using CSI Driver at scale, it's recommended to create PV dynamically via dynamic provisioning, relieving the administrator from managing the PVs, while also achieving application data isolation. Under dynamic provisioning, the Kubernetes administrator will create and manage one or more StorageClass, the user only need to create a PVC and reference it in pod definition, and JuiceFS CSI Driver will create the corresponding PV for you, with each PV corresponding to a subdirectory inside JuiceFS.

The relationship between different resources:

![dynamic-provisioning](./images/dynamic-provisioning.svg)

Taking mount pod mode for example, this is the overall process:

* User creates a PVC using an existing JuiceFS StorageClass;
* PV is created and provisioned by CSI Controller, by default, a sub-directory named with PV ID will be created under JuiceFS root, settings controlling this process are defined (or referenced) in StorageClass definition.
* Kubernetes PV Controller binds user-created PVC with the PV created by CSI Controller, PVC and PV both enter "Bound" state;
* User creates application pod, referencing PVC previously created;
* CSI Node Service creates mount pod on the associating node;
* A JuiceFS Client runs inside the mount pod, and mounts JuiceFS volume to host, path being `/var/lib/juicefs/volume/[pv-name]`;
* CSI Node Service waits until mount pod is up and running, and binds PV with the associated container, the PV sub-directory is mounted in pod, path defined by `volumeMounts`;
* Application pod is started by Kubelet.

## Other mount modes {#other-mount-modes}

By default, CSI Driver runs in mount pod mode, which isn't allowed in certain scenarios, other mount mode may come in handy when that happens.

### Sidecar mode {#sidecar}

Mount pod is created by CSI Node, due to CSI Node being a DaemonSet component, if your Kubernetes cluster does not allow DaemonSets (like some Serverless Kubernetes platform), CSI Node will not be able to install, and JuiceFS CSI Driver cannot be used properly. For situations like this, you can choose to run CSI Driver in sidecar mode, which runs JuiceFS Client in sidecar containers.

In this mode, CSI Node is no longer needed, CSI Controller is the only installed component. For Kubernetes namespaces that need to use CSI Driver, CSI Controller will listen for pod changes, check if JuiceFS PVC is used, and inject sidecar container accordingly.

![sidecar-architecture](./images/sidecar-architecture.svg)

The overall process:

* A Webhook is registered to API Server when CSI Controller starts;
* An application pod reference an existing JuiceFS PVC;
* Before actual pod creation, API Server will query against the Webhook API;
* CSI Controller injects the sidecar container (with JuiceFS Client running inside) into the application pod;
* API Server creates the application pod, with JuiceFS Client running in its sidecar container, application container can access JuiceFS once it's started.

Some sidecar caveats:

* FUSE must be supported, meaning that container will run in privileged mode;
* Different from mount by pod, a sidecar container is injected into the application pod, so sharing PV is not possible. Carefully manage resources when use at scale;
* Mount point is shared between sidecar & application container using `hostPath`, which means sidecar container is actually stateful, so in the event of a sidecar container crash, mount point cannot automatically restore without re-creating the whole pod (in contrast, mount pod mode supports [automatic mount point recovery](./guide/pv.md#automatic-mount-point-recovery));
* Do not switch to sidecar mode directly from mount pod mode, as existing mount pods won't automatically migrate to sidecar mode, and just simply stagnate;
* CSI Controller will listen for all pod change events under namespaces with sidecar injections enabled. If you'd like to minimize overhead, you can even ignore pods by labeling them with `disable.sidecar.juicefs.com/inject: true`, so that CSI Controller deliberately ignores them.

To use sidecar mode, [install CSI Driver in sidecar mode](./getting_started.md#sidecar).

### Mount by process {#by-process}

Apart from using a dedicated mount pod or a sidecar container to run JuiceFS Client, JuiceFS CSI Driver also supports running JuiceFS Client directly inside CSI Node Service, as processes (mount by process). In this mode, one or several JuiceFS Clients will run inside the CSI Node Service pod, managing all JuiceFS mount points for application pods referencing JuiceFS PV in the associating node.

![byprocess-architecture](./images/byprocess-architecture.svg)

When all JuiceFS Client run inside CSI Node Service pod, it's not hard to imagine that CSI Node Service will be needing more resource. It's recommended to increase resource requests to 1 CPU and 1GiB Memory, limits to 2 CPU and 5GiB Memory, or adjust according to the actual resource usage.

In Kubernetes, mount by pod is no doubt the more recommended way to use JuiceFS CSI Driver. But outside the Kubernetes world, there'll be scenarios requiring the mount by process mode, for example, [Use JuiceFS CSI Driver in Nomad](./cookbook/csi-in-nomad.md).

For versions before v0.10.0, JuiceFS CSI Driver only supports mount by process. For v0.10.0 and above, mount by pod is the default behavior. To upgrade from v0.9 to v0.10, refer to [Upgrade under mount by process mode](./administration/upgrade-csi-driver.md#mount-by-process-upgrade).

To use mount by process mode, [install CSI Driver in by-process mode](./getting_started.md#by-process).
