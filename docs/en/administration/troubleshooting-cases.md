---
title: Troubleshooting Cases
slug: /troubleshooting-cases
sidebar_position: 7
---

Debugging process for some frequently encountered problems, you can search for your issue using error keywords. Also, we recommend you to have a firm grasp on [Basic principles for troubleshooting](./troubleshooting.md#basic-principles).

## CSI Driver installation issue

If JuiceFS CSI Driver isn't installed, or not properly configured, then following error will occur:

```
kubernetes.io/csi: attacher.MountDevice failed to create newCsiDriverClient: driver name csi.juicefs.com not found in the list of registered CSI drivers
```

Thoroughly follow the steps in [Installation](../getting_started.md), pay special attention to kubelet root directory settings.

Above error message shows that the CSI Driver named `csi.juicefs.com` isn't found. Please check if you used `mount pod` mode or `sidecar` mode.

If you used `mount pod` mode, follow these steps to troubleshoot:

* Run `kubectl get csidrivers.storage.k8s.io` and check if `csi.juicefs.com` actually missing, if that is indeed the case, CSI Driver isn't installed at all, head to [Installation](../getting_started.md).
* If `csi.juicefs.com` already exists in the above `csidrivers` list, that means CSI Driver is installed, the problem is with CSI Node.
* [Check if CSI Node is working correctly](./troubleshooting.md#check-csi-node).
* There should be a CSI Node pod on the exact Kubernetes node where the application pod is running, if [scheduling strategy](../guide/resource-optimization.md#csi-node-node-selector) has been configured for the CSI Node DaemonSet, or the node itself is [tainted](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration), CSI Node may be missing on such worker nodes.

If you used `sidecar` mode, check if the namespace which application pod running has `juicefs.com/enable-injection=true` label:

```shell
# Change to the namespace where the application pod is located
kubectl get ns <namespace> --show-labels
```

## CSI Node pod failure

If CSI Node pod is not properly running, and the socket file used to communicate with kubelet is gone, you'll observe the following error in application pod events:

```
/var/lib/kubelet/csi-plugins/csi.juicefs.com/csi.sock: connect: no such file or directory
```

[Check CSI Node](./troubleshooting.md#check-csi-node) to debug and troubleshoot. A commonly encountered problem is kubelet being started without authentication webhook, which results in error when getting pod list:

```
kubelet_client.go:99] GetNodeRunningPods err: Unauthorized
reconciler.go:70] doReconcile GetNodeRunningPods: invalid character 'U' looking for beginning of value
```

Read our docs on [enabling kubelet authentication](../administration/going-production.md#kubelet-authn-authz) to fix this.

## Mount Pod failure {#mount-pod-error}

JuiceFS Client runs inside the mount pod and there's a variety of possible causes for error, this section covers some of the more frequently seen problems.

* **Mount pod stuck at `Pending` state, causing application pod to be stuck as well at `ContainerCreating` state**

  When this happens, [Check mount pod events](./troubleshooting.md#check-mount-pod) to debug. Note that `Pending` state usually indicates problem with resource allocation.

  In addition, when kubelet enables the preemption, the mount pod may preempt application resources after startup, resulting in repeated creation and destruction of both the mount pod and the application pod, with the mount pod event saying:

  ```
  Preempted in order to admit critical pod
  ```

  Default resource requests for mount pod is 1 CPU, 1GiB memory, mount pod will refuse to start or preempt application when allocatable resources is low, consider [adjusting resources for mount pod](../guide/resource-optimization.md#mount-pod-resources), or upgrade the worker node to work with more resources.

* **After mount pod is restarted or recreated, application pods cannot access JuiceFS**

  If mount pod crashes and restarts, or manually deleted and recreated, accessing JuiceFS (e.g. running `df`) inside the application pod will result in this error, indicating that the mount point is gone:

  ```
  Transport endpoint is not connected

  df: /jfs: Socket not connected
  ```

  In this case, you'll need to enable [automatic mount point recovery](../guide/pv.md#automatic-mount-point-recovery), so that mount point is propagated to the application pod, as long as the mount pod can continue to run after failure, application will be able to use JuiceFS inside container.

* **Mount pod exits normally (exit code 0), causing application pod to be stuck at `ContainerCreateError` state**

  Mount pod should always be up and running, if it exits and becomes `Completed` state, even if the exit code is 0, PV will not work correctly. Since mount point doesn't exist anymore, application pod will be show error events like this:

  ```shell {4}
  $ kubectl describe pod juicefs-app
  ...
    Normal   Pulled     8m59s                 kubelet            Successfully pulled image "centos" in 2.8771491s
    Warning  Failed     8m59s                 kubelet            Error: failed to generate container "d51d4373740596659be95e1ca02375bf41cf01d3549dc7944e0bfeaea22cc8de" spec: failed to generate spec: failed to stat "/var/lib/kubelet/pods/dc0e8b63-549b-43e5-8be1-f84b25143fcd/volumes/kubernetes.io~csi/pvc-bc9b54c9-9efb-4cb5-9e1d-7166797d6d6f/mount": stat /var/lib/kubelet/pods/dc0e8b63-549b-43e5-8be1-f84b25143fcd/volumes/kubernetes.io~csi/pvc-bc9b54c9-9efb-4cb5-9e1d-7166797d6d6f/mount: transport endpoint is not connected
  ```

  The `transport endpoint is not connected` error in above logs means JuiceFS mount point is missing, and application pod cannot be created. You should inspect the mount pod start-up command to identify the cause for this (the following commands are from the ["Check mount pod"](./troubleshooting.md#check-mount-pod) documentation):

  ```shell
  APP_NS=default  # application pod namespace
  APP_POD_NAME=example-app-xxx-xxx

  # Obtain mount pod name
  MOUNT_POD_NAME=$(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}'))

  # Obtain mount pod start-up command
  # Should look like ["sh","-c","/sbin/mount.juicefs myjfs /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa -o foreground"]
  kubectl get pod -o jsonpath='{..containers[0].command}' $MOUNT_POD_NAME
  ```

  Check the mount pod start-up command carefully. In the above example, the options followed by `-o` are the mount parameters of the JuiceFS file system. If there are multiple mount parameters, they will be connected through `,` (such as `-o aaa,bbb`). If you find a wrong format like `-o debug foreground` (the correct format should be `-o debug,foreground`), it will cause the mount pod to fail to start normally. This type of error is usually caused by erroneous `mountOptions`, refer to [Adjust mount options](../guide/pv.md#mount-options) and thoroughly check for any format errors.

## PVC error {#pvc-error}

* **Under static provisioning, PV uses the wrong `storageClassName`, causing provisioning error and PVC is stuck at `Pending` state**

  StorageClass exists to provide provisioning parameters for [Dynamic provisioning](../guide/pv.md#dynamic-provisioning) when creating a PV. For [Static provisioning](../guide/pv.md#static-provisioning), `storageClassName` must be an empty string, or you'll find errors like:

  ```shell {7}
  $ kubectl describe pvc juicefs-pv
  ...
  Events:
    Type     Reason                Age               From                                                                           Message
    ----     ------                ----              ----                                                                           -------
    Normal   Provisioning          9s (x5 over 22s)  csi.juicefs.com_juicefs-csi-controller-0_872ea36b-0fc7-4b66-bec5-96c7470dc82a  External provisioner is provisioning volume for claim "default/juicefs-pvc"
    Warning  ProvisioningFailed    9s (x5 over 22s)  csi.juicefs.com_juicefs-csi-controller-0_872ea36b-0fc7-4b66-bec5-96c7470dc82a  failed to provision volume with StorageClass "juicefs": claim Selector is not supported
    Normal   ExternalProvisioning  8s (x2 over 23s)  persistentvolume-controller                                                    waiting for a volume to be created, either by external provisioner "csi.juicefs.com" or manually created by system administrator
  ```

* **PVC creation failures due to `volumeHandle` conflicts**

  This happens when an application pod try to use multiple PVCs, but referenced PV uses a same `volumeHandle`, you'll see errors like:

  ```shell {6}
  $ kubectl describe pvc jfs-static
  ...
  Events:
    Type     Reason         Age               From                         Message
    ----     ------         ----              ----                         -------
    Warning  FailedBinding  4s (x2 over 16s)  persistentvolume-controller  volume "jfs-static" already bound to a different claim.
  ```
  
  In addition, the application pod will also be accompanied by the following events. There are volumes (spec.volumes) named `data1` and `data2` in the application pod, and an error will be reported in event that one of the volumes is not mounted:

  ```shell
  Events:
  Type     Reason       Age    From               Message
  ----     ------       ----   ----               -------
  Warning  FailedMount  12s    kubelet            Unable to attach or mount volumes: unmounted volumes=[data1], unattached volumes=[data2 kube-api-access-5sqd8 data1]: timed out waiting for the condition
  ```

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

## Performance issues {#performance-issue}

Compared to using JuiceFS directly on a host mount point, CSI Driver provides powerful functionalities but also comes with higher complexities. This section only covers issues that are specific to CSI Driver, if you suspect your problem at hand isn't related to CSI Driver, learn to debug JuiceFS itself in [Community Edition](https://juicefs.com/docs/community/fault_diagnosis_and_analysis) and [Cloud Service](https://juicefs.com/docs/cloud/administration/fault_diagnosis_and_analysis) documentations.

### Bad read performance {#bad-read-performance}

We'll demonstrate a typical performance issue and troubleshooting path under the CSI Driver, using a simple fio test as example.

The actual fio command involves 5 * 500MB = 2.5GB of data, reaching a not particularly satisfying result:

```shell
$ fio -directory=. \
  -ioengine=mmap \
  -rw=randread \
  -bs=4k \
  -group_reporting=1 \
  -fallocate=none \
  -time_based=1 \
  -runtime=120 \
  -name=test_file \
  -nrfiles=1 \
  -numjobs=5 \
  -size=500MB
...
  READ: bw=9896KiB/s (10.1MB/s), 9896KiB/s-9896KiB/s (10.1MB/s-10.1MB/s), io=1161MiB (1218MB), run=120167-120167msec
```

When encountered with performance issues, take a look at [Real-time statistics](./troubleshooting.md#accesslog-and-stats), the stats during our fio test looks like:

```shell
$ juicefs stats /var/lib/juicefs/volume/pvc-xxx-xxx-xxx-xxx-xxx-xxx
------usage------ ----------fuse--------- ----meta--- -blockcache remotecache ---object--
 cpu   mem   buf | ops   lat   read write| ops   lat | read write| read write| get   put
 302%  287M   24M|  34K 0.07   139M    0 |   0     0 |7100M    0 |   0     0 |   0     0
 469%  287M   29M|  23K 0.10    92M    0 |   0     0 |4513M    0 |   0     0 |   0     0
...
```

Read performance really depends on cache, so when read performance isn't ideal, pay special attention to the `blockcache` related metrics, block cache is data blocks cached on disk, notice how `blockcache.read` is always larger than 0 in the above data, this means kernel page cache isn't built, thus all read requests is handled by the slower disk reads. Now we will investigate why page cache won't build.

Similar to what we'll do on a host, let's first check the mount pod's resource usage, make sure there's enough memory for page cache. Use below commands to locate the Docker container for our mount pod, and see its stats:

```shell
# change $APP_POD_NAME to actual application pod name
$ docker stats $(docker ps | grep $APP_POD_NAME | grep -v "pause" | awk '{print $1}')
CONTAINER ID   NAME          CPU %     MEM USAGE / LIMIT   MEM %     NET I/O   BLOCK I/O   PIDS
90651c348bc6   k8s_POD_xxx   45.1%     1.5GiB / 2GiB       75.00%    0B / 0B   0B / 0B     1
```

Note that the memory limit is 2GiB, while the fio test is trying to read 2.5G of data, which is more than the pod memory limit. Even though memory usage indicated by `docker stats` isn't close to the 2GiB limit, kernel is already unable to build more page cache, because page cache size is a part of cgroup memory limit. In this case, we'll [adjust resources for mount pod](../guide/resource-optimization.md#mount-pod-resources), increase memory limit, re-create PVC / application pod, and then try again.

:::note
`docker stats` counts memory usage differently under cgroup v1/v2, v1 does not include kernel page cache while v2 does, the case described here is carried out under cgroup v1, but it doesn't affect the troubleshooting thought process and conclusion.
:::

For convenience's sake, we'll simply lower the data size for the fio test command, which is then able to achieve the perfect result:

```shell
$ fio -directory=. \
  -ioengine=mmap \
  -rw=randread \
  -bs=4k \
  -group_reporting=1 \
  -fallocate=none \
  -time_based=1 \
  -runtime=120 \
  -name=test_file \
  -nrfiles=1 \
  -numjobs=5 \
  -size=100MB
...
   READ: bw=12.4GiB/s (13.3GB/s), 12.4GiB/s-12.4GiB/s (13.3GB/s-13.3GB/s), io=1492GiB (1602GB), run=120007-120007msec
```

Conclusion: **When using JuiceFS inside containers, memory limit should be larger than the target data set for Linux kernel to fully build the page cache.**

### Bad write performance {#bad-write-performance}

* **Slow write speed for intensive small writes (like untar, unzip)**

  For scenario that does intensive small writes, we usually recommend users to temporarily enable client write cache (read [JuiceFS Community Edition](https://juicefs.com/docs/community/cache_management#writeback), [JuiceFS Cloud Service](https://juicefs.com/docs/cloud/guide/cache#client-write-cache) to learn more), but due to its inherent risks, this is advised against when using CSI Driver, because pod lifecycle is significantly more unstable, and can cause data loss if pod exists unexpectedly.

  If you need to write a large amount of small files into JuiceFS, it's recommended that you find a host mount point, and temporarily enable `--writeback` for such operation. If you absolutely have to use `--writeback` in CSI Driver, try to improve pod stability (for example, [increase resource usage](../guide/resource-optimization.md#mount-pod-resources)).

## Umount error (mount pod hangs) {#umount-error}

JuiceFS cannot be unmounted when files or directories are still opened. If this happens within a Kubernetes cluster, mount pod will exit with the mount point not being released:

```
2m17s       Normal    Started             pod/juicefs-xxx   Started container jfs-mount
44s         Normal    Killing             pod/juicefs-xxx   Stopping container jfs-mount
44s         Warning   FailedPreStopHook   pod/juicefs-xxx   PreStopHook failed
```

A worse case is JuiceFS Client process entering uninterruptible sleep (D) state, mount pod cannot be deleted and will stuck at Terminating state, the attached cgroup cannot be deleted either, causing kubelet to produce the following error:

```
Failed to remove cgroup (will retry)" error="rmdir /sys/fs/cgroup/blkio/kubepods/burstable/podxxx/xxx: device or resource busy
```

For such unmount errors, refer to [Community Edition](https://juicefs.com/docs/community/administration/troubleshooting/#unmount-error) and [Cloud Service](https://juicefs.com/docs/cloud/administration/troubleshooting/#umount-error) documentation (handled similarly).
