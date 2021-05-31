# Static Provisioning Subpath

Persisten volume can be provisioned as a subpath in juicefs file system.

## Patches

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

Patch the persistent volume spec with `csi/volumeAttributes/subPath`. The `subPath` must pre-exist.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
spec:
  csi:
    volumeAttributes:
      subPath: fluentd
```

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

or apply with kubectl >= 1.14

```s
kubectl apply -k .
```

## Check JuiceFS file system is used

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods juicefs-app-subpath
```

Also you can verify that data is written onto JuiceFS file system:

```sh
>> kubectl exec -ti juicefs-app-subpath -- tail -f /data/out.txt
```
