---
title: Troubleshooting Cases
slug: /troubleshooting-cases
sidebar_position: 7
---

Debugging process for some frequently encountered problems, you can search for your issue using error keywords. Also, we recommend you to have a firm grasp on [Basic principles for troubleshooting](./troubleshooting.md#basic-principles).

## CSI Driver not installed / installation failure

If JuiceFS CSI Driver isn't installed, or not properly configured, then following error will occur:

```
driver name csi.juicefs.com not found in the list of registered CSI drivers
```

Thoroughly follow the steps in [Installation](../getting_started.md), pay special attention to kubelet root directory settings.

## CSI Node pod failure

If CSI Node pod is not properly running, and the socket file used to communicate with kubelet is gone, you'll observe the following error in application pod events:

```
/var/lib/kubelet/csi-plugins/csi.juicefs.com/csi.sock: connect: no such file or directory
```

[Check CSI Node](./troubleshooting.md#check-csi-node) to debug and troubleshoot.

## Mount Pod failure

One of the most seen problems is mount pod stuck at `Pending` state, causing application pod to stuck as well at `ContainerCreating` state. When this happens, [Check mount pod events](./troubleshooting.md#check-mount-pod) to debug. Also, `Pending` state usually indicates problem with resource allocation.

In addition, when kubelet enables the preemption, the mount pod may preempt application resources after startup, resulting in repeated creation and destruction of both the mount pod and the application pod, with the mount pod event saying:

```
Preempted in order to admit critical pod
```

Default resource requests for mount pod is 1 CPU, 1GiB memory, mount pod will refuse to start or preempt application when allocatable resources is low, consider [adjusting resources for mount pod](../guide/resource-optimization.md#mount-pod-resources), or upgrade the worker node to work with more resources.

## PVC creation failures due to configuration conflicts

For example, two app pods try to use their own PVC, but only one runs well and the other can't get up.

Check `volumeHandle` of all relevant PV, ensure `volumeHandle` is unique :

```yaml {12}
$ kubectl get pv -o yaml juicefs-pv
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  ...
spec:
  ...
  csi:
    driver: csi.juicefs.com
    fsType: juicefs
    volumeHandle: juicefs-volume-abc
    ...
```

## File system creation failure (Community Edition)

When you choose to dynamically create file system inside mount pod, i.e. running the `juicefs format` command, when this process fails, you'll see error logs in the CSI Node pod:

```
format: ERR illegal address: xxxx
```

The `format` in the error message stands for the `juicefs format` command. Above error usually indicates problems accessing the metadata engine, check security group configurations to ensure all Kubernetes worker nodes could access the metadata engine.

If you use a password protected Redis instance as metadata engine, you may encounter the following error:

```
format: NOAUTH Authentication requested.
```

Make sure you've specified the correct password in the metadata engine URL, as described in [using Redis as metadata engine](https://juicefs.com/docs/community/databases_for_metadata/#redis).
