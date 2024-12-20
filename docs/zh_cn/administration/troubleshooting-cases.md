---
title: 问题排查案例
slug: /troubleshooting-cases
sidebar_position: 7
---

这里收录常见问题的具体排查步骤，你可以直接在本文搜索报错关键字以检索问题。同时，我们也推荐你先掌握[「基础问题排查思路」](./troubleshooting.md#basic-principles)。

## CSI 驱动安装异常 {#csi-driver-installation-issue}

如果 JuiceFS CSI 驱动压根没安装，或者配置错误导致安装失败，那么试图使用 JuiceFS CSI 驱动时，便会有下方报错：

```
kubernetes.io/csi: attacher.MountDevice failed to create newCsiDriverClient: driver name csi.juicefs.com not found in the list of registered CSI drivers
```

上方的报错信息表示，名为 `csi.juicefs.com` 的驱动没有找到，请先确认使用的是 Mount Pod 模式还是 Sidecar 模式。

若使用的是 Mount Pod 模式，遵循以下步骤进行排查：

* 运行 `kubectl get csidrivers.storage.k8s.io`，如果输出的中确没有 `csi.juicefs.com` 字样，说明 CSI 驱动并未安装，仔细回顾[「安装 JuiceFS CSI 驱动」](../getting_started.md)；
* 检查 kubelet 的根目录与 CSI Node 的配置是否一致，如果不一致，会导致 CSI Node 无法正常注册，请修复 CSI Node 的配置，或者重新安装，参考[「安装 JuiceFS CSI 驱动」](../getting_started.md)；

  ```shell
  # kubelet 根目录
  ps -ef | grep kubelet | grep root-dir 
  # CSI Node 配置
  kubectl -n kube-system get ds juicefs-csi-node -oyaml | grep csi.juicefs.com
  ```

* 如果上方的 `csidrivers` 列表中存在 `csi.juicefs.com`，那么说明 CSI 驱动已经安装，问题出在 CSI Node，检查 CSI Node 是否正常运作：
  * 排查开始前，可以简单阅读[检查 CSI Node](./troubleshooting.md#check-csi-node)，代码示范里有一些快捷命令可供参考；
  * 关注应用 Pod 所在节点，检查节点是否正常运行着 CSI Node，如果为 CSI Node 这个 DaemonSet 组件配置了[调度策略](../guide/resource-optimization.md#csi-node-node-selector)，或者节点本身存在[「污点」](https://kubernetes.io/zh-cn/docs/concepts/scheduling-eviction/taint-and-toleration)，都有可能造成 CSI Node 容器缺失，造成该错误；
  * 如果问题节点的 CSI Node 正常运行（处于 Running 状态），核实他的各个容器均没有明显错误日志，比方说：

  ```shell
  # juicefs-plugin 容器负责运行 CSI 驱动的实际工作，如果他访问 Kubernetes API 失败，则会导致 Mount Pod 无法创建
  kubectl logs -n kube-system juicefs-csi-node-xxx juicefs-plugin --tail 100

  # node-driver-registrar 容器负责注册 csidriver，如果注册过程异常，该容器会报错
  kubectl logs -n kube-system juicefs-csi-node-xxx node-driver-registrar --tail 100
  ```

  * 如果以上排查均无结论，则认为 Kubernetes 本身出现了问题，可以尝试重启 kubelet 或者重启系统，如果问题仍得不到解决，需要向 Kubernetes 的管理员或服务提供商寻求帮助。

若使用的是 sidecar 模式，请确认对应的 namespace 有没有打上 JuiceFS sidecar 所需 label（`juicefs.com/enable-injection=true`）：

```shell
# 换成应用 pod 所在 namespace
kubectl get ns <namespace> --show-labels
```

## CSI Node Pod 异常 {#csi-node-pod-failure}

如果 CSI Node Pod 异常，与 kubelet 通信的 socket 文件不复存在，应用 Pod 事件中会看到如下错误日志：

```
/var/lib/kubelet/csi-plugins/csi.juicefs.com/csi.sock: connect: no such file or directory
```

此时需要[检查 CSI Node](./troubleshooting.md#check-csi-node)，确认其异常原因，并排查修复。常见的问题比如 kubelet 没有启用 Authentication webhook，导致获取 Pod 列表时报错：

```
kubelet_client.go:99] GetNodeRunningPods err: Unauthorized
reconciler.go:70] doReconcile GetNodeRunningPods: invalid character 'U' looking for beginning of value
```

面对这种情况，阅读[启用 Kubelet 认证鉴权](../administration/going-production.md#kubelet-authn-authz)了解如何修复该问题。

## Mount Pod 异常 {#mount-pod-error}

Mount Pod 内运行着 JuiceFS 客户端，出错的可能性多种多样，在这里罗列常见错误，指导排查。

<details>
<summary>**Mount Pod 一直卡在 `Pending` 状态，导致应用容器也一并卡死在 `ContainerCreating` 状态**</summary>

此时需要 [查看 Mount Pod 事件](./troubleshooting.md#check-mount-pod)，确定症结所在。不过对于 `Pending` 状态，大概率是资源吃紧，导致容器无法创建。

另外，当节点 kubelet 开启抢占功能，Mount Pod 启动后可能抢占应用资源，导致 Mount Pod 和应用 Pod 均反复创建、销毁，在 Pod 事件中能看到以下信息：

```
Preempted in order to admit critical Pod
```

Mount Pod 默认的资源声明是 1 CPU、1GiB 内存，节点资源不足时，便无法启动，或者启动后抢占应用资源。此时需要根据实际情况 [调整 Mount Pod 资源声明](../guide/resource-optimization.md#mount-pod-resources)，或者扩容宿主机。

集群 IP 不足也可能导致 Mount Pod 一直处于 `Pending` 状态。Mount Pod 默认以 `hostNetwork: false` 的形式启动，可能会占用大量的集群 IP 资源，如果集群资源 IP 不足可能会导致 Mount Pod 启动不成功。请联系云服务提供商对 Kubernetes 集群的 IP 数量进行扩容，或者使用 `hostNetwork: true` 形式启动，参阅：[定制 Mount Pod 和 Sidecar 容器](../guide/configurations.md#customize-mount-pod)。

</details>

<details>
<summary>**Mount Pod 重启或者重新创建后，应用容器无法访问 JuiceFS**</summary>

如果 Mount Pod 发生异常重启，或者经历了手动删除，那么应用 Pod 内访问挂载点（比如 `df`）会产生如下报错，提示挂载点已经不存在：

```
Transport endpoint is not connected

df: /jfs: Socket not connected
```

你需要启用 [「挂载点自动恢复」](../guide/configurations.md#automatic-mount-point-recovery)，这样一来，只要 Mount Pod 能自行重建，恢复挂载点，应用容器就能继续访问 JuiceFS。

</details>

<details>
<summary>**Mount Pod 正常退出（exit code 为 0），应用容器卡在 `ContainerCreateError` 状态**</summary>

Mount Pod 是一个常驻进程，如果它退出了（变为 `Completed` 状态），即便退出状态码为 0，也明显属于异常状态。此时应用容器由于挂载点不复存在，会伴随着以下错误事件：

```shell {4}
$ kubectl describe pod juicefs-app
...
  Normal   Pulled     8m59s                 kubelet            Successfully pulled image "centos" in 2.8771491s
  Warning  Failed     8m59s                 kubelet            Error: failed to generate container "d51d4373740596659be95e1ca02375bf41cf01d3549dc7944e0bfeaea22cc8de" spec: failed to generate spec: failed to stat "/var/lib/kubelet/pods/dc0e8b63-549b-43e5-8be1-f84b25143fcd/volumes/kubernetes.io~csi/pvc-bc9b54c9-9efb-4cb5-9e1d-7166797d6d6f/mount": stat /var/lib/kubelet/pods/dc0e8b63-549b-43e5-8be1-f84b25143fcd/volumes/kubernetes.io~csi/pvc-bc9b54c9-9efb-4cb5-9e1d-7166797d6d6f/mount: transport endpoint is not connected
```

错误日志里的 `transport endpoint is not connected`，其含义就是创建容器所需的 JuiceFS 挂载点不存在，因此应用容器无法创建。这时需要检查 Mount Pod 的启动命令（以下命令来自 [「检查 Mount Pod」](./troubleshooting.md#check-mount-pod) 文档）：

```shell
APP_NS=default  # 应用所在的 Kubernetes 命名空间
APP_POD_NAME=example-app-xxx-xxx

# 获取 Mount Pod 的名称
MOUNT_POD_NAME=$(kubectl -n kube-system get po --field-selector spec.nodeName=$(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{.spec.nodeName}') -l app.kubernetes.io/name=juicefs-mount -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' | grep $(kubectl get pv $(kubectl -n $APP_NS get pvc $(kubectl -n $APP_NS get po $APP_POD_NAME -o jsonpath='{..persistentVolumeClaim.claimName}' | awk '{print $1}') -o jsonpath='{.spec.volumeName}') -o jsonpath='{.spec.csi.volumeHandle}'))

# 获取 Mount Pod 启动命令
# 形如：["sh","-c","/sbin/mount.juicefs myjfs /jfs/pvc-48a083ec-eec9-45fb-a4fe-0f43e946f4aa -o foreground"]
kubectl get pod -o jsonpath='{..containers[0].command}' $MOUNT_POD_NAME
```

仔细检查 Mount Pod 启动命令，以上示例中 `-o` 后面所跟的选项即为 JuiceFS 文件系统的挂载参数，如果有多个挂载参数会通过 `,` 连接（如 `-o aaa,bbb`）。如果发现类似 `-o debug foreground` 这样的错误格式（正确格式应该是 `-o debug,foreground`），便会造成 Mount Pod 无法正常启动。此类错误往往是 `mountOptions` 填写错误造成的，请详读 [「调整挂载参数」](../guide/configurations.md#mount-options)，确保格式正确。

</details>

<details>
<summary>**Mount Pod 没有创建**</summary>

使用 `kubectl describe <app-pod-name>` 查看当前应用 Pod 的事件，确认已经进入挂载流程，而不是调度失败或者其它与挂载 JuiceFS 无关的错误。

如果应用 Pod 的事件为：

- `driver name csi.juicefs.com not found` 或者 `csi.sock no such file`

  检查对应节点上的 CSI Node Pod 是否运行正常，详见[文档](#csi-node-pod-failure)。

- `Unable to attach or mount volumes: xxx`

  查看对应节点上 CSI Node Pod 的日志，过滤出对应 PV 的相关日志。如果没有找到类似于 `NodePublishVolume: volume_id is <pv-name>` 的日志，并且 Kubernetes 版本低于 1.26.0、1.25.1、1.24.5、1.23.11，可能是因为 kubelet 的一个 bug 导致没有触发 volume publish 请求，详见 [#109047](https://github.com/kubernetes/kubernetes/issues/109047)。

  此时可以尝试：

  - 重启 kubelet
  - 升级 Kubernetes

  总之 JuiceFS CSI 驱动需要收到请求才能开始挂载流程。

</details>

## PVC 异常 {#pvc-error}

<details>
<summary>**静态配置中，PV 错误填写了 `storageClassName`，导致初始化异常，PVC 卡在 `Pending` 状态**</summary>

StorageClass 的存在是为了给 [「动态配置」](../guide/pv.md#dynamic-provisioning) 创建 PV 时提供初始化参数。对于 [「静态配置」](../guide/pv.md#static-provisioning)，`storageClassName` 必须填写为空字符串，否则将遭遇类似下方报错：

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

</details>

<details>
<summary>**`volumeHandle` 冲突，导致 PVC 创建失败**</summary>

一个 Pod 使用多个 PVC，但引用的 PV 有着相同的 `volumeHandle`，此时 PVC 将伴随着以下错误事件：

```shell {6}
$ kubectl describe pvc jfs-static
...
Events:
  Type     Reason         Age               From                         Message
  ----     ------         ----              ----                         -------
  Warning  FailedBinding  4s (x2 over 16s)  persistentvolume-controller  volume "jfs-static" already bound to a different claim.
```

另外，应用 Pod 也会伴随着以下错误事件，应用 Pod 中有分别有名为 `data1` 和 `data2` 的 volume（spec.volumes），event 中会报错其中一个 volume 没有 mount：

```shell
Events:
Type     Reason       Age    From               Message
----     ------       ----   ----               -------
Warning  FailedMount  12s    kubelet            Unable to attach or mount volumes: unmounted volumes=[data1], unattached volumes=[data2 kube-api-access-5sqd8 data1]: timed out waiting for the condition
```

请检查每个 PVC 对应的 PV，每个 PV 的 `volumeHandle` 必须保证唯一。可以通过以下命令检查 `volumeHandle`：

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

</details>

## 文件系统创建错误（社区版） {#file-system-creation-failure-community-edition}

如果你选择在 Mount Pod 中动态地创建文件系统，也就是执行 `juicefs format` 命令，那么当创建失败时，应该会在 CSI Node Pod 中看到如下错误：

```
format: ERR illegal address: xxxx
```

这里的 `format`，指的就是 `juicefs format` 命令，以上方的报错，多半是访问元数据引擎出现了问题，请检查你的安全组设置，确保所有 Kubernetes 集群的节点都能访问元数据引擎。

如果使用 Redis 作为元数据引擎，且启用了密码认证，那么可能遇到如下报错：

```
format: NOAUTH Authentication requested.
```

你需要确认元数据引擎 URL 是否正确填写了密码，具体格式请参考[「使用 Redis 作为元数据引擎」](https://juicefs.com/docs/zh/community/databases_for_metadata#redis)。

## 性能问题 {#performance-issue}

相比直接在宿主机上挂载 JuiceFS，CSI 驱动功能更为强大，但也无疑额外增加了复杂度。这里仅介绍一些 CSI 驱动下的特定问题，如果你怀疑所遭遇的性能问题与 CSI 驱动无关，请进一步参考[「社区版」](https://juicefs.com/docs/zh/community/fault_diagnosis_and_analysis)和[「云服务」](https://juicefs.com/docs/zh/cloud/administration/fault_diagnosis_and_analysis)文档学习相关排查方法。

### 读性能差 {#bad-read-performance}

以一个简单的 fio 测试为例，说明在 CSI 驱动下可能面临着怎样的性能问题，以及排查路径。

测试所用的命令，涉及数据集大小为 5 * 500MB = 2.5GB，测得的结果不尽如人意：

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

遇到性能问题，首先查看[「实时统计数据」](./troubleshooting.md#accesslog-and-stats)。在测试期间，实时监控数据大体如下：

```shell
$ juicefs stats /var/lib/juicefs/volume/pvc-xxx-xxx-xxx-xxx-xxx-xxx
------usage------ ----------fuse--------- ----meta--- -blockcache remotecache ---object--
 cpu   mem   buf | ops   lat   read write| ops   lat | read write| read write| get   put
 302%  287M   24M|  34K 0.07   139M    0 |   0     0 |7100M    0 |   0     0 |   0     0
 469%  287M   29M|  23K 0.10    92M    0 |   0     0 |4513M    0 |   0     0 |   0     0
...  # 后续数据与上方相似
```

JuiceFS 的高性能离不开其缓存设计，因此读性能发生问题时，我们首先关注 `blockcache` 相关指标，也就是磁盘上的数据块缓存文件。注意到上方数据中，`blockcache.read` 一直大于 0，这说明内核没能建立页缓存（Page Cache），所有的读请求都穿透到了位于磁盘的 Block Cache。内核页缓存位于内存，而 Block Cache 位于磁盘，二者的读性能相差极大，读请求持续穿透到磁盘，必定造成较差的性能，因此接下来调查内核缓存为何没能建立。

同样的情况如果发生在宿主机，我们会去看宿主机的内存占用情况，首先确认是否因为内存不足，没有足够空间建立页缓存。在容器中也是类似的，定位到 Mount Pod 对应的 Docker 容器，然后查看其资源占用：

```shell
# $APP_POD_NAME 是应用 pod 名称
$ docker stats $(docker ps | grep $APP_POD_NAME | grep -v "pause" | awk '{print $1}')
CONTAINER ID   NAME          CPU %     MEM USAGE / LIMIT   MEM %     NET I/O   BLOCK I/O   PIDS
90651c348bc6   k8s_POD_xxx   45.1%     1.5GiB / 2GiB       75.00%    0B / 0B   0B / 0B     1
```

注意到内存上限是 2GiB，而 fio 面对的数据集是 2.5G，已经超出了容器内存限制。此时，虽然在 `docker stats` 观察到的内存占用尚未到达 2GiB 天花板，但实际上[页缓存也占用了 cgroup 内存额度](https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt)，导致内核已经无法建立页缓存，因此调整 [Mount Pod 资源占用](../guide/resource-optimization.md#mount-pod-resources)，增大 Memory Limits，然后重建 PVC、应用 Pod，然后再次运行测试。

:::note
`docker stats` 在 cgroup v1/v2 下有着不同的统计口径，v1 不包含内核页缓存，v2 则包含。本案例在 cgroup v1 下运行，但不影响排查思路与结论。
:::

此处为了方便，我们反方向调参，降低 fio 测试数据集大小，然后测得了理想的结果：

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

结论：**在容器内使用 JuiceFS，内存上限应大于所访问的数据集大小，否则将无法建立页缓存，损害读性能。**

### 写性能差 {#bad-write-performance}

* **写入大量小文件（比如解压缩），写入速度慢**

  对于大量小文件写入场景，我们一般推荐临时开启客户端写缓存（阅读[社区版文档](https://juicefs.com/docs/zh/community/cache_management/#writeback)、[云服务文档](https://juicefs.com/docs/zh/cloud/guide/cache/#client-write-cache)以了解），但由于该模式本身带来的数据安全风险，我们尤其不推荐在 CSI 驱动中开启 `--writeback`，避免容器出现意外时，写缓存尚未完成上传，造成数据无法访问。

  因此，在容器场景下，如果需要大量写入小文件，我们建议在宿主机挂载点临时启用客户端写缓存来进行操作，如果不得不在 CSI 驱动中启用客户端写缓存，则需要尤其关注容器稳定性（比如适当[提升资源占用](../guide/resource-optimization.md#mount-pod-resources)）。

## 卸载失败（Mount Pod 无法退出） {#umount-error}

卸载 JuiceFS 文件系统时，如果某个文件或者目录正在被使用，那么卸载将会报错。发生在 Kubernetes 集群中，则体现为 Mount Pod 退出时清理失败：

```
2m17s       Normal    Started             pod/juicefs-xxx   Started container jfs-mount
44s         Normal    Killing             pod/juicefs-xxx   Stopping container jfs-mount
44s         Warning   FailedPreStopHook   pod/juicefs-xxx   PreStopHook failed
```

更糟的情况是 JuiceFS 客户端进程进入 D 状态，导致 Mount Pod 卡死在 Terminating 状态无法删除，其关联的 cgroup 也无法删除，最终在 kubelet 产生类似下方的错误日志：

```
Failed to remove cgroup (will retry)" error="rmdir /sys/fs/cgroup/blkio/kubepods/burstable/podxxx/xxx: device or resource busy
```

对于卸载错误，请参考[社区版](https://juicefs.com/docs/zh/community/administration/troubleshooting/#unmount-error)、[云服务](https://juicefs.com/docs/zh/cloud/administration/troubleshooting/#umount-error)文档进行排查（处理手段是类似的）。
