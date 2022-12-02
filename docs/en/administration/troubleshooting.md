---
title: Troubleshooting Methods
slug: /troubleshooting
sidebar_position: 5
---

Read this chapter to learn how to troubleshoot JuiceFS CSI Driver, to continue, you should already be familiar with [the JuiceFS CSI Architecture](../introduction.md#architecture), i.e. have a basic understanding of the roles of each CSI Driver component.

## Basic principles for troubleshooting {#basic-principles}

In JuiceFS CSI Driver, most frequently encountered problems are PV creation failures (managed by CSI Controller) and pod creation failures (managed by CSI Node / Mount Pod).

### PV creation failure

Under [dynamic provisioning](../guide/pv.md#dynamic-provisioning), after PVC has been created, CSI Controller will work with kubelet to automatically create PV. During this phase, CSI Controller will create a sub-directory in JuiceFS named after the PV ID (naming pattern can be configured via [`pathPattern`](../examples/subpath.md#using-path-pattern)).

#### Check PVC events

Usually, CSI Controller will pass error information to PVC event:

```shell {7}
$ kubectl describe pvc dynamic-ce
...
Events:
  Type     Reason       Age                From               Message
  ----     ------       ----               ----               -------
  Normal   Scheduled    27s                default-scheduler  Successfully assigned default/juicefs-app to cluster-0003
  Warning  FailedMount  11s (x6 over 27s)  kubelet            MountVolume.SetUp failed for volume "juicefs-pv" : rpc error: code = Internal desc = Could not mount juicefs: juicefs auth error: Failed to fetch configuration for volume 'juicefs-pv', the token or volume is invalid.
```

#### Check CSI Controller

If no error appears in PVC events, we'll need to check if CSI Controller is alive and working correctly:

```shell
# Check CSI Controller aliveness
$ kubectl -n kube-system get po -l app=juicefs-csi-controller
NAME                       READY   STATUS    RESTARTS   AGE
juicefs-csi-controller-0   3/3     Running   0          8d

# Check CSI Controller logs
$ kubectl -n kube-system logs juicefs-csi-controller-0 juicefs-plugin
```

### Application pod failure

Due to the architecture of the CSI Driver, JuiceFS Client runs in a dedicated mount pod, thus, every application pod is accompanied by a mount pod.

CSI Node will create the mount pod, mount the JuiceFS file system within the pod, and finally bind the mount point to the application pod. If application pod fails to start, we shall look for issues in CSI Node, or mount pod.

#### Check application pod events

If error occurs during the mount, look for clues in the application pod events:

```shell {7}
$ kubectl describe po dynamic-ce-1
...
Events:
  Type     Reason       Age               From               Message
  ----     ------       ----              ----               -------
  Normal   Scheduled    53s               default-scheduler  Successfully assigned default/ce-static-1 to ubuntu-node-2
  Warning  FailedMount  4s (x3 over 37s)  kubelet            MountVolume.SetUp failed for volume "ce-static" : rpc error: code = Internal desc = Could not mount juicefs: juicefs status 16s timed out
```

If error event indicates problems within the JuiceFS space, follow below guide to further troubleshoot.

#### Check CSI Node {#check-csi-node}

Verify CSI Node is alive and working correctly:

```shell
# Application pod information will be used in below commands, save them as environment variables.
APP_NS=default  # application pod namespace
APP_POD_NAME=example-app-xxx-xxx

# Locate worker node via application pod name
NODE_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}')

# Print all CSI Node pods
kubectl -n kube-system get po -l app=juicefs-csi-node

# Print CSI Node pod closest to the application pod
kubectl -n kube-system get po -l app=juicefs-csi-node --field-selector spec.nodeName=$NODE_NAME

# Substitute $CSI_NODE_POD with actual CSI Node pod name acquired above
kubectl -n kube-system logs $CSI_NODE_POD -c juicefs-plugin
```

Or simply use this one-liner to print logs of the relevant CSI Node pod (the `APP_NS` and `APP_POD_NAME` environment variables need to be set):

```shell
kubectl -n kube-system logs $(kubectl -n kube-system get po -o jsonpath='{..metadata.name}' -l app=juicefs-csi-node --field-selector spec.nodeName=$(kubectl get po -o jsonpath='{.spec.nodeName}' -n $APP_NS $APP_POD_NAME)) -c juicefs-plugin
```

#### Check mount pod {#check-mount-pod}

If no errors are shown in the CSI Node logs, check if mount pod is working correctly.

Finding corresponding mount pod via given application pod can be tedious, here's a series of commands to help you with this process:

```shell
# Application pod information will be used in below commands, save them as environment variables.
APP_NS=default  # application pod namespace
APP_POD_NAME=example-app-xxx-xxx

# Find Node / PVC / PV name via application pod
NODE_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}')
# If application pod uses multiple PVs, below command only obtains the first one, adjust accordingly.
PVC_NAME=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}'|awk '{print $1}')
PV_NAME=$(kubectl -n $APP_NS get pvc $PVC_NAME -o jsonpath='{.spec.volumeName}')
PV_ID=$(kubectl get pv $PV_NAME -o jsonpath='{.spec.csi.volumeHandle}')

# Find mount pod via application pod
MOUNT_POD_NAME=$(kubectl -n kube-system get po --field-selector spec.nodeName=$NODE_NAME -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $PV_ID)

# Check mount pod state
kubectl -n kube-system get po $MOUNT_POD_NAME

# Check mount pod events
kubectl -n kube-system describe po $MOUNT_POD_NAME

# Print mount pod logs, which contain JuiceFS Client logs.
kubectl -n kube-system logs $MOUNT_POD_NAME

# Find all mount pod for give PV
kubectl -n kube-system get po -l app.kubernetes.io/name=juicefs-mount -o wide | grep $PV_ID
```

Or simply use this one-liner to print name of the relevant Mount Pod (the `APP_NS` and `APP_POD_NAME` environment variables need to be set):

```shell
kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}')
```

## Seeking support

If you are not able to troubleshoot, seek help from the JuiceFS community or the Juicedata team. You should collect some information so others can further diagnose.

### Check JuiceFS CSI Driver version

Obtain CSI Driver version:

```shell
kubectl -n kube-system get po -l app=juicefs-csi-controller -o jsonpath='{.items[*].spec.containers[*].image}'
```

Image tag will contain the CSI Driver version string.

### Diagnosis script

You can also use the [diagnosis script](https://github.com/juicedata/juicefs-csi-driver/blob/master/scripts/diagnose.sh) to collect logs and related information.

First, install the script to any node with `kubectl` access:

```shell
wget https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/diagnose.sh
chmod a+x diagnose.sh
```

Collect diagnose information using the script. For example, assuming CSI Driver is deployed in the `kube-system` namespace, and the problem occurs in worker node `kube-node-2`.

```shell
$ ./diagnose.sh
Usage:
      ./diagnose.sh COMMAND [OPTIONS]
COMMAND:
      help
         Display this help message.
      collect
         Collect pods logs of juicefs.
OPTIONS:
      -no, --node name
         Set the name of node.
      -n, --namespace name
         Set the namespace of juicefs csi driver.

$ ./diagnose.sh -n kube-system -no kube-node-2 collect
Start collecting, node-name=kube-node-2, juicefs-namespace=kube-system
...
please get diagnose_juicefs_1628069696.tar.gz for diagnostics
```

All relevant information is collected and packaged in an archive under the execution path.
