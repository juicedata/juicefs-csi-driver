# Mount Options

This example shows how to apply mount options to JuiceFS PersistentVolume (PV).

## Patches

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

The CSI driver support the `juicefs mount` command line options and _fuse_ mount options (`-o` for `juicefs mount` command).

```
juicefs mount --max-uploads=50 --cache-dir=/var/foo --cache-size=2048 --enable-xattr -o allow_other <REDIS-URL> <MOUNTPOINT>
```

The command line options and fuse options of above example can be provided by `mountOptions`.

Patch the PersistentVolume spec with `csi/volumeAttributes/mountOptions`.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
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
kubectl get pods juicefs-app-mount-options
```

Also you can verify that mount options are customized in the mounted JuiceFS file system:

```sh
kubectl exec -ti juicefs-csi-node-2zz7h -c juicefs-plugin sh
ps xf
```
