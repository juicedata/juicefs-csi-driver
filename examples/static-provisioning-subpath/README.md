# Static Provisioning Subpath

Persisten volume can be provisioned as a subpath in juicefs filesystem.

## Patches

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

Patch the persistent volume spec with `csi/volumeAttributes/subPath`. The subPath must pre-exist.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-aws-us-east-1
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

## Check JuiceFS filesystem is used

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

Check that file is created under the subpath in [JuiceFS console](https://juicefs.com/console).
