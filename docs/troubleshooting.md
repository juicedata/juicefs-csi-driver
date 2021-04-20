# Troubleshooting

When your pod is not `Running` status (e.g. `ContainerCreating`), there may have some issues. You need check JuiceFS CSI driver logs to get more information, please follow steps blow.

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
