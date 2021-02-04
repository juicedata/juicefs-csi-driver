# Read Write Many

Persistent volume provisioned by JuiceFS supports ReadWriteMany access mode.

## Resources

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

We shall create a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) to scale the pods sharing the same PVC.

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

or apply with kubectl >= 1.14

```s
kubectl apply -k .
```

## Scaling

Scale up

```s
kubectl scale -n default deployment scaling-app-rwx --replicas=64
```

Scale down

```s
kubectl scale -n default deployment scaling-app-rwx --replicas=1
```

## Check JuiceFS file system is used

After the configuration is applied, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS file system:

```sh
>> kubectl exec -ti juicefs-csi-scaling-85676b4c7c-rzzlf -- ls /data
out-scaling-app-rwx-5686d45f6f-2t874.txt  out-scaling-app-rwx-5686d45f6f-cbdwl.txt  out-scaling-app-rwx-5686d45f6f-j5j4n.txt  out-scaling-app-rwx-5686d45f6f-phrzm.txt	out-scaling-app-rwx-5686d45f6f-s7x2x.txt  out-scaling-app-rwx-5686d45f6f-xhjm9.txt
...
```
