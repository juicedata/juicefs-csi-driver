# Mount Options

This example shows how to apply mount options to JuiceFS persistence volume (PV).

## Using kustomize

[kustomize](https://github.com/kubernetes-sigs/kustomize) is a builtin plugin since kubectl **1.14**.

```sh
kustomize build | kubectl apply -f -
```

or with kubectl >= 1.14

```sh
kubectl apply -k -f .
```

## Using kubectl < 1.14 without kustomize

### Make a copy of [base kubernetes manifest](../../deploy/volume/juicefs.yaml)

```yaml
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: juicefs
provisioner: csi.juicefs.com
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs
spec:
  capacity:
    storage: 10Pi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: $(JUICEFS_STORAGE_CLASS_NAME)
  csi:
    driver: $(JUICEFS_CSI_DRIVER)
    volumeHandle: juicefs-name-not-provided
    fsType: juicefs
    nodePublishSecretRef:
      name: $(JUICEFS_SECRET_NAME)
      namespace: $(JUICEFS_SECRET_NAMESPACE)
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs
spec:
  accessModes:
    - ReadWriteMany
  storageClassName: $(JUICEFS_STORAGE_CLASS_NAME)
  resources:
    requests:
      storage: 10Pi
```

Provide mandatory attribute

- `spec/csi/volumeHandle` in `PersistentVolume`: JuiceFS filesystem name pre-created in [juicefs web console](https://juicefs.com/console)

Replace variables in `$(...)` value with actual value.

- `JUICEFS_SECRET_NAME`: secret containing JuiceFS `token` and optionally `accesskey` and `secretkey` for `juicefs auth`
- `JUICEFS_SECRET_NAMESPACE`: namespace of the secret for `juicefs auth`
- `JUICEFS_CSI_DRIVER`: should be consistent with `provisioner` in `StorageClass`
- `JUICEFS_STORAGE_CLASS_NAME`: should be consistent with `metadata/name` in `StorageClass`

### Apply the Example

Create storage class, PV, persistence volume claim (PVC)

```sh
>> kubectl apply -f deploy/volume/juicefs.yaml
```

Create sample pod

```sh
>> kubectl apply -f deploy/pod/juicefs-app.yaml
```

### Check mount options are customized

After the objects are created, verify that pod is running:

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
