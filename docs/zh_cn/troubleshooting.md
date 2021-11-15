# 故障排查

当 pod 状态不为 `Running` （如 `ContainerCreating`）时，意味着 pod 可能存在一些问题。您需要查看 JuiceFS CSI driver pod 的日志，操作如下：

## JuiceFS CSI Driver v0.10 及以后版本

1. 找到您的 pod 所在的节点。比如，假设您的 pod 名为 `juicefs-app`：

```sh
$ kubectl get pod juicefs-app -o wide
NAME          READY   STATUS              RESTARTS   AGE   IP       NODE          NOMINATED NODE   READINESS GATES
juicefs-app   0/1     ContainerCreating   0          9s    <none>   172.16.2.87   <none>           <none>
```

从以上输出可以看出，pod 所在节点为 `172.16.2.87`。

2. 找到您的 pod 所用的 PersistentVolume (PV) 的 volume ID。

比如，您的 pod 所用的 PersistentVolumeClaim (PVC) 名为 `juicefs-pvc`：

```sh
$ kubectl get pvc juicefs-pvc
NAME          STATUS   VOLUME       CAPACITY   ACCESS MODES   STORAGECLASS   AGE
juicefs-pvc   Bound    juicefs-pv   10Pi       RWX                           42d
```

从以上输出可以看出，PV 名为 `juicefs-pv`，然后获取这个 PV 的完整 YAML：

```yaml
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

从以上输出可以看出，`spec.csi.volumeHandle` 为 volume ID，如：`juicefs-volume-abc`。

3. 通过 node 名和 volume ID 找到 JuiceFS mount pod，比如：

```sh
$ kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-mount -o wide | grep 172.16.2.87 | grep juicefs-volume-abc
juicefs-172.16.2.87-juicefs-volume-abc   1/1     Running   0          20h    172.16.2.100   172.16.2.87   <none>           <none>
```

从以上输出可以看出，JuiceFS mount pod 名为 `juicefs-172.16.2.87-juicefs-volume-abc`。

4. 获取 JuiceFS mount pod 的日志，如：

```sh
$ kubectl -n kube-system logs juicefs-172.16.2.87-juicefs-volume-abc
```

5. 找到所有包含 `WARNING`，`ERROR` 或 `FATAL` 的日志。

## JuiceFS CSI Driver v0.10 以前的版本

1. 找到您的 pod 所在的节点。比如，假设您的 pod 名为 `juicefs-app`：

```sh
$ kubectl get pod juicefs-app -o wide
NAME          READY   STATUS              RESTARTS   AGE   IP       NODE          NOMINATED NODE   READINESS GATES
juicefs-app   0/1     ContainerCreating   0          9s    <none>   172.16.2.87   <none>           <none>
```

从以上输出可以看出，pod 所在节点为 `172.16.2.87`。

2. 找到相同节点上的 JuiceFS CSI driver pod。比如：

```sh
$ kubectl describe node 172.16.2.87 | grep juicefs-csi-node
  kube-system                 juicefs-csi-node-hzczw                  1 (0%)        2 (1%)      1Gi (0%)         5Gi (0%)       61m
```

从以上输出可以看出，JuiceFS CSI driver pod 名为 `juicefs-csi-node-hzczw`。

3. 获取 JuiceFS CSI Driver 的日志。如：

```sh
$ kubectl -n kube-system logs juicefs-csi-node-hzczw -c juicefs-plugin
```

4. 找到所有包含 `WARNING`，`ERROR` 或 `FATAL` 的日志。

## 错误诊断

您也可以使用 [诊断脚本](../../scripts/diagnose.sh) 来收集日志及相关信息。

1. 在您的集群中可以执行 kubectl 的节点上，下载诊断脚本

```shell
wget  https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/diagnose.sh
```

2. 给脚本添加执行权限

```shell
chmod a+x diagnose.sh
```

3. 使用诊断脚本来收集信息。比如，您的 JuiceFS CSI Driver 部署在 `kube-system` 这个 namespace 下，并行您想收集 `kube-node-2` 这台节点上的信息。

```shell
$ ./diagnose.sh
Usage:
    ./diagnose-juicefs.sh COMMAND [OPTIONS]
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

所有相关的信息都被收集和打包在了一个压缩包里。
