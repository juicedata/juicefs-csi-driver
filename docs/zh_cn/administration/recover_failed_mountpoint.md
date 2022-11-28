---
slug: /recover-failed-mountpoint
sidebar_position: 3
---

# 挂载点自动恢复

JuiceFS CSI Driver v0.10.7 开始支持挂载点自动恢复。

## 使用方法

业务应用需要在 pod 的 MountVolume 中设置 `mountPropagation` 为 `HostToContainer` 或 `Bidirectional`（需要设置 pod 为特权 pod），从而将 host 的挂载信息传送给 pod。配置如下：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: juicefs-app-static-deploy
spec:
  ...
  template:
    ...
    spec:
      containers:
      - name: app
        volumeMounts:
        - mountPath: /data
          name: data
          mountPropagation: HostToContainer
        ...
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: juicefs-pvc-static
```

## 挂载点恢复测试

pod 挂载后，可以看到 mount pod 如下：

```shell
$ kubectl get po -A
NAMESPACE     NAME                                                           READY   STATUS    RESTARTS   AGE
default       juicefs-app-static-deploy-7fcc667995-cmswc                     1/1     Running   0          6m49s
kube-system   juicefs-kube-node-3-juicefs-pv                                1/1     Running   0          6m30s
```

为了做测试，我们将 mount pod `juicefs-kube-node-3-juicefs-pv` 删除，然后观察 pod 的恢复情况，结果如下：

```shell
$ kubectl -n kube-system get po -w
NAME                                                           READY   STATUS        RESTARTS   AGE
...
juicefs-kube-node-3-juicefs-pv                                0/1     Terminating   0          8m8s
juicefs-kube-node-3-juicefs-pv                                0/1     Terminating   0          8m28s
juicefs-kube-node-3-juicefs-pv                                0/1     Terminating   0          8m37s
juicefs-kube-node-3-juicefs-pv                                0/1     Terminating   0          8m37s
juicefs-kube-node-3-juicefs-pv                                0/1     Pending       0          0s
juicefs-kube-node-3-juicefs-pv                                0/1     ContainerCreating   0          0s
juicefs-kube-node-3-juicefs-pv                                0/1     ContainerCreating   0          1s
juicefs-kube-node-3-juicefs-pv                                0/1     Running             0          2s
juicefs-kube-node-3-juicefs-pv                                1/1     Running             0          3s
...
```

通过 watch 的结果，可以看到 mount pod 在被删除之后，又被新建出来。接着在业务容器中检查挂载点信息：

```shell
$ kubectl -n default exec -it juicefs-app-static-deploy-7fcc667995-cmswc bash
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
[root@juicefs-app-static-deploy-7fcc667995-cmswc /]#
[root@juicefs-app-static-deploy-7fcc667995-cmswc /]# df
Filesystem                  1K-blocks    Used     Available Use% Mounted on
overlay                      17811456 9162760       8648696  52% /
tmpfs                           65536       0         65536   0% /dev
tmpfs                         3995028       0       3995028   0% /sys/fs/cgroup
JuiceFS:minio           1099511627776     512 1099511627264   1% /data
/dev/mapper/centos-root      17811456 9162760       8648696  52% /etc/hosts
shm                             65536       0         65536   0% /dev/shm
tmpfs                         3995028      12       3995016   1% /run/secrets/kubernetes.io/serviceaccount
tmpfs                         3995028       0       3995028   0% /proc/acpi
tmpfs                         3995028       0       3995028   0% /proc/scsi
tmpfs                         3995028       0       3995028   0% /sys/firmware
[root@juicefs-app-static-deploy-7fcc667995-cmswc /]#
[root@juicefs-app-static-deploy-7fcc667995-cmswc /]#
```
