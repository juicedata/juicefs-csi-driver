# Troubleshooting

When your pod is not `Running` status (e.g. `ContainerCreating`), there may have some issues. You need check JuiceFS CSI driver logs to get more information, please follow steps blow.

## JuiceFS CSI Driver v0.10+

### Get mount pod

1. Find the node where the pod is deployed. For example, your pod name is `juicefs-app`:

   ```sh {3}
   $ kubectl get pod juicefs-app -o wide
   NAME          READY   STATUS              RESTARTS   AGE   IP       NODE          NOMINATED NODE   READINESS GATES
   juicefs-app   0/1     ContainerCreating   0          9s    <none>   172.16.2.87   <none>           <none>
   ```

   From above output, the node is `172.16.2.87`.

2. Find the volume ID of the PersistentVolume (PV) used by your pod.

   For example, the PersistentVolumeClaim (PVC) used by your pod is named `juicefs-pvc`:

   ```sh {3}
   $ kubectl get pvc juicefs-pvc
   NAME          STATUS   VOLUME       CAPACITY   ACCESS MODES   STORAGECLASS   AGE
   juicefs-pvc   Bound    juicefs-pv   10Pi       RWX                           42d
   ```

   From above output, the name of PV is `juicefs-pv`, then get the YAML of this PV:

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

   From above output, the `spec.csi.volumeHandle` is the volume ID, i.e. `juicefs-volume-abc`.

3. Find JuiceFS mount pod by node name and volume ID. For example:

   ```sh {2}
   $ kubectl -n kube-system get pod -l app.kubernetes.io/name=juicefs-mount -o wide | grep 172.16.2.87 | grep juicefs-volume-abc
   juicefs-172.16.2.87-juicefs-volume-abc   1/1     Running   0          20h    172.16.2.100   172.16.2.87   <none>           <none>
   ```

   From above output, the name of JuiceFS mount pod is `juicefs-172.16.2.87-juicefs-volume-abc`.

### Get logs of mount pod

1. Get JuiceFS mount pod logs. For example:

   ```sh
   kubectl -n kube-system logs juicefs-172.16.2.87-juicefs-volume-abc
   ```

2. Find any log contains `WARNING`, `ERROR` or `FATAL`.

## Before JuiceFS CSI Driver v0.10

1. Find the node where the pod is deployed. For example, your pod name is `juicefs-app`:

   ```sh {3}
   $ kubectl get pod juicefs-app -o wide
   NAME          READY   STATUS              RESTARTS   AGE   IP       NODE          NOMINATED NODE   READINESS GATES
   juicefs-app   0/1     ContainerCreating   0          9s    <none>   172.16.2.87   <none>           <none>
   ```

   From above output, the node is `172.16.2.87`.

2. Find the JuiceFS CSI driver pod in the same node. For example:

   ```sh {2}
   $ kubectl describe node 172.16.2.87 | grep juicefs-csi-node
   kube-system                 juicefs-csi-node-hzczw                  1 (0%)        2 (1%)      1Gi (0%)         5Gi (0%)       61m
   ```

   From above output, the JuiceFS CSI driver pod name is `juicefs-csi-node-hzczw`.

3. Get JuiceFS CSI driver logs. For example:

   ```sh
   kubectl -n kube-system logs juicefs-csi-node-hzczw -c juicefs-plugin
   ```

4. Find any log contains `WARNING`, `ERROR` or `FATAL`.

## Diagnosis Script

You can also use the [diagnosis script](https://github.com/juicedata/juicefs-csi-driver/blob/master/scripts/diagnose.sh) to collect logs and related information.

1. Download the diagnosis script to the node which can exec `kubectl`.

   ```shell
   wget https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/scripts/diagnose.sh
   ```

2. Add execute permission to script.

   ```shell
   chmod a+x diagnose.sh
   ```

3. Collect diagnose information using the script. For example, your JuiceFS CSI Driver is deployed in `kube-system` namespace, and you want to see information in node named `kube-node-2`.

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

   All relevant information is collected and packaged in a ZIP archive under the execution path.
