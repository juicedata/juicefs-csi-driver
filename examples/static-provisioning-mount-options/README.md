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

or apply with kubectl >= 1.14

```s
kubectl apply -k .
```

## Check mount options are customized

After the configuration is applied, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that mount options are customized in the mounted JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-csi-node-2zz7h -c juicefs-plugin sh

sh-4.2# yum install procps
sh-4.2# ps xf
...
root       342  0.0  1.1 122484 11596 ?        S    12:02   0:00 /usr/bin/python2 /sbin/mount.juicefs csi-demo /var/lib/kubelet/pods/f513c3e5-7576-11e9-a400-0aa5dd01d816/volumes/kubernetes.io~csi/juicefs/mount -o rw,cache-dir=/var/foo,cache-size=124,metacache HOSTNAME=ip-
root       344  0.5  5.1  70632 52892 ?        S<l  12:02   0:03  \_ juicefs -mountpoint /var/lib/kubelet/pods/f513c3e5-7576-11e9-a400-0aa5dd01d816/volumes/kubernetes.io~csi/juicefs/mount -ssl -cacheDir /var/foo/csi-demo -cacheSize 124 -o fsname=JuiceFS:csi-demo,allow_oth
```

Note that `-cacheDir` is different from default value `/var/jfsCache/csi-demo` and `-cacheSize` is customized as `124`.
