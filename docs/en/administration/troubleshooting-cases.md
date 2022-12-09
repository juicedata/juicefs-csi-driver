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

Similar to what we'll do on a host, let's first check the mount pod's resource usage, make sure there's enough memory for page cache. Use below commands to locate the docker container for our mount pod, and see its stats:

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
