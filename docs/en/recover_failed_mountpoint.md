# Automatic mount point recovery

JuiceFS CSI Driver started to support automatic mount point recovery since version v0.10.7.

## How to use it in application

Applications need to set `mountPropagation` to `HostToContainer` or `Bidirectional`(privileged required) in the
MountVolume of the pod. In this way, the mount information of the host is transmitted to the pod. The configuration is
as follows:

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

## Test for recovery of mount point

You can see mount pod as follows after application pod is created successfully:

```shell
$ kubectl get po -A
NAMESPACE     NAME                                                           READY   STATUS    RESTARTS   AGE
default       juicefs-app-static-deploy-7fcc667995-cmswc                     1/1     Running   0          6m49s
kube-system   juicefs-kube-node-3-test-bucket                                1/1     Running   0          6m30s
```

We delete mount pod `juicefs-kube-node-3-test-bucket` for testing, and then watch pod for recovery:

```shell
$ kubectl -n kube-system get po -w
NAME                                                           READY   STATUS        RESTARTS   AGE
...
juicefs-kube-node-3-test-bucket                                0/1     Terminating   0          8m8s
juicefs-kube-node-3-test-bucket                                0/1     Terminating   0          8m28s
juicefs-kube-node-3-test-bucket                                0/1     Terminating   0          8m37s
juicefs-kube-node-3-test-bucket                                0/1     Terminating   0          8m37s
juicefs-kube-node-3-test-bucket                                0/1     Pending       0          0s
juicefs-kube-node-3-test-bucket                                0/1     ContainerCreating   0          0s
juicefs-kube-node-3-test-bucket                                0/1     ContainerCreating   0          1s
juicefs-kube-node-3-test-bucket                                0/1     Running             0          2s
juicefs-kube-node-3-test-bucket                                1/1     Running             0          3s
...
```

From the above, we can see mount pod is created again after it has been deleted. Then we check mount point in
application pod:

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
