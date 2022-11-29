---
title: Troubleshooting Cases
slug: /troubleshooting-cases
sidebar_position: 6
---

Debugging process for some frequently encountered problems, you can search for your issue using error keywords. Also, we recommend you to have a firm grasp on [Basic principles for troubleshooting](./troubleshooting.md#basic-principles).

## CSI Driver not installed / installation failure

If JuiceFS CSI Driver isn't installed, or not properly configured, then following error will occur:

```
driver name csi.juicefs.com not found in the list of registered CSI drivers
```

Thoroughly follow the steps in [Installation](../getting_started.md), pay special attention to kubelet root directory settings.

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

When you choose to dynamically create file systems inside mount pod, e.g. running the `juicefs format` command, when this process fails, you'll see error logs in the CSI Node pod:

```
format: ERR illegal address: xxxx
```

The `format` in the error message stands for the `juicefs format` command. Above error usually indicates problems accessing the metadata engine, check security group configurations to ensure all Kubernetes worker nodes have access to the metadata engine.

If you use a password protected Redis instance as metadata engine, you may encounter the following error:

```
format: NOAUTH Authentication requested.
```

Make sure you've specified the correct password in the metadata URL, as described in [using Redis as metadata engine](https://juicefs.com/docs/community/databases_for_metadata/#redis).

