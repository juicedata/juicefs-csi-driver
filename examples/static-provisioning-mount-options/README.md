# Mount Options

This example shows how to apply mount options to JuiceFS persistence volume (PV).

## Patches

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

Patch the persistent volume spec with `csi/volumeAttributes/mountOptions`.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-aws-us-east-1
spec:
  csi:
    volumeAttributes:
      mountOptions: "enable-xattr,max-uploads=50,cache-size=100,cache-dir=/var/foo"
```

Refer to [JuiceFS mount command](https://github.com/juicedata/juicefs/#mount-a-volume) for all supported options.

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

## Check mount options are customized

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods
```

Also you can verify that mount options are customized in the mounted JuiceFS filesystem:

```sh
kubectl exec -ti juicefs-csi-node-2zz7h -c juicefs-plugin sh
ps xf
```
