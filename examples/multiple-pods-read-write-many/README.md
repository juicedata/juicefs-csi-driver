# Multiple Pods Read Write Many

This example shows how to make JuiceFS persistence volume (PV) mounted by multiple pods and allow read/write stimultaneously.

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

## Check JuiceFS filesystem is used

In the example, both pods are writing to the same JuiceFS filesystem at the same time.

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app-1 -- tail -f /data/out-1.txt
>> kubectl exec -ti juicefs-app-2 -- tail -f /data/out-2.txt
```
