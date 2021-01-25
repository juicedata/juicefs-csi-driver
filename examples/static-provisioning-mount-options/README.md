# Mount Options

This example shows how to apply mount options to JuiceFS persistence volume (PV).

## Patches

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

CSI driver support the `juicefs mount` command line options and _fuse_ mount options (`-o` for `juicefs mount` command).

```
juicefs mount max-uploads=50 cache-dir=/var/foo cache-size=2048 --enable-xattr -o allow_other
```

The command line options and fuse options of above example can be provided by `mountOptions`.

Patch the persistent volume spec with `csi/volumeAttributes/mountOptions`.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-aws-us-east-1
spec:
  csi:
    volumeAttributes:
      mountOptions: "enable-xattr,max-uploads=50,cache-size=2048,cache-dir=/var/foo,allow_other"
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
