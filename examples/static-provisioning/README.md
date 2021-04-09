# Static Provisioning

This example shows how to make a static provisioned JuiceFS persistence volume (PV) mounted inside container.

## Provide secret information

In order to build the example, you need to provide a secret file `secrets.env` containing the required credentials

```ini
name=<name>
metaurl=<metaurl>
access-key=<access-key>
secret-key=<secret-key>
storage=<storage>
bucket=<bucket>
```

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`:

```sh
kustomize build | kubectl apply -f -
```

Or apply with `kubectl` >= 1.14:

```sh
kubectl apply -k .
```

## Check JuiceFS file system is used

After the objects are created, verify that pod is running:

```sh
kubectl get pods
```

Also you can verify that data is written onto JuiceFS file system:

```sh
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```
