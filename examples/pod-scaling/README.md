# Pod Scaling

This example shows how JuiceFS persistence volume (PV) supports large scale pods replicas.

## Provide secret information

In order to build the example, you need to provide a secret file `Secret-juicefs.env` containing the required credentials

```ini
token=<juicefs-token>
accesskey=<juicefs-accesskey>
secretkey=<juicefs-secretkey>
```

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

or apply with kubectl >= 1.14

```s
kubectl apply -k -f .
```

## Scaling

Scale up

```s
kubectl scale -n default deployment juicefs-csi-scaling --replicas=64
```

Scale down

```s
kubectl scale -n default deployment juicefs-csi-scaling --replicas=1
```

## Check JuiceFS filesystem is used

In the example, both pods are writing to the same JuiceFS filesystem at the same time.

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
kubectl exec -ti juicefs-csi-scaling-85676b4c7c-rzzlf -- ls /data
out-juicefs-csi-scaling-85676b4c7c-8xfwc.txt  out-juicefs-csi-scaling-85676b4c7c-hlchk.txt
...
```
