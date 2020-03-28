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
      mountOptions: "metacache,cache-size=100,cache-dir=/var/foo"
```

Refer to [JuiceFS command reference](https://juicefs.com/docs/zh/commands_reference.html#juicefs-mount) for all supported options.

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

## Check mount options are customized

After the configuration is applied, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that mount options are customized in the mounted JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-csi-node-2zz7h -c juicefs-plugin sh
>> ps xf
...
   66 root      0:00 /usr/local/bin/python2 /usr/bin/juicefs mount aws-us-east-1 /jfs/aws-us-east-1 --metacache --cache-size=100 --cache-dir=/var/foo
   68 root      0:00 {jfsmount} juicefs -mountpoint /jfs/aws-us-east-1 -ssl -cacheDir /var/foo/aws-us-east-1 -cacheSize 100 -o fsname=JuiceFS:aws-us-east-1,allow_other,nonempty -metacacheto 300 -attrcacheto 1 -entrycacheto 1 -direntrycacheto 1 -compress zstd
...
```
