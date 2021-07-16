# Troubleshooting

When your pod is not `Running` status (e.g. `ContainerCreating`), there may have some issues. You need check JuiceFS CSI driver logs to get more information, please follow steps blow.

## JuiceFS CSI Driver v0.10+

1. Find the node where the pod is deployed. For example, your pod name is `juicefs-app`:

```sh
$ kubectl get pod juicefs-app -o wide
NAME          READY   STATUS              RESTARTS   AGE   IP       NODE          NOMINATED NODE   READINESS GATES
juicefs-app   0/1     ContainerCreating   0          9s    <none>   172.16.2.87   <none>           <none>
```

From above output, the node is `172.16.2.87`.

2. Find the volume ID of the PersistentVolume (PV) used by your pod.

For example, the PersistentVolumeClaim (PVC) used by your pod is named `juicefs-pvc`:

```sh
$ kubectl get pvc juicefs-pvc
NAME          STATUS   VOLUME       CAPACITY   ACCESS MODES   STORAGECLASS   AGE
juicefs-pvc   Bound    juicefs-pv   10Pi       RWX                           42d
```

From above output, the name of PV is `juicefs-pv`, then get the YAML of this PV:

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

From above output, the `spec.csi.volumeHandle` is the volume ID, i.e. `juicefs-volume-abc`.

3. Find JuiceFS mount pod by node name and volume ID. For example:

```sh
$ kubectl get pod -l app.kubernetes.io/name=juicefs-mount -o wide | grep 172.16.2.87 | grep juicefs-volume-abc
juicefs-172.16.2.87-juicefs-volume-abc   1/1     Running   0          20h    172.16.2.100   172.16.2.87   <none>           <none>
```

From above output, the name of JuiceFS mount pod is `juicefs-172.16.2.87-juicefs-volume-abc`.

4. Get JuiceFS mount pod logs. For example:

```sh
$ kubectl logs juicefs-172.16.2.87-juicefs-volume-abc
```

5. Find any log contains `WARNING`, `ERROR` or `FATAL`.

## Before JuiceFS CSI Driver v0.10

1. Find the node where the pod is deployed. For example, your pod name is `juicefs-app`:

```sh
$ kubectl get pod juicefs-app -o wide
NAME          READY   STATUS              RESTARTS   AGE   IP       NODE          NOMINATED NODE   READINESS GATES
juicefs-app   0/1     ContainerCreating   0          9s    <none>   172.16.2.87   <none>           <none>
```

From above output, the node is `172.16.2.87`.

2. Find the JuiceFS CSI driver pod in the same node. For example:

```sh
$ kubectl describe node 172.16.2.87 | grep juicefs-csi-node
  kube-system                 juicefs-csi-node-hzczw                  1 (0%)        2 (1%)      1Gi (0%)         5Gi (0%)       61m
```

From above output, the JuiceFS CSI driver pod name is `juicefs-csi-node-hzczw`.

3. Get JuiceFS CSI driver logs. For example:

```sh
$ kubectl -n kube-system logs juicefs-csi-node-hzczw -c juicefs-plugin
```

4. Find any log contains `WARNING`, `ERROR` or `FATAL`.
