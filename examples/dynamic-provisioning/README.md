# Dynamic Provisioning

This example shows how to make a dynamic provisioned JuiceFS persistence volume (PV) mounted inside container.

## Provide secret information

In order to build the example, you need to provide a secret file `Secret-juicefs.env` containing the required credentials

```ini
name=<juicefs-name>
token=<juicefs-token>
accesskey=<juicefs-accesskey>
secretkey=<juicefs-secretkey>
```

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

## Check JuiceFS filesystem is used

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

Check for the directory created as PV https://juicefs.com/console/vol/<juicefs-name>/
