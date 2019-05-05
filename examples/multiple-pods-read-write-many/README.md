# Multiple Pods Read Write Many

This example shows how to make a static provisioned JuiceFS persistence volume (PV) mounted inside container.

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

### Edit [Persistence Volume Resource](./resources/PersistentVolume-juicefs-csi-demo.yaml)

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-csi-demo
spec:
  capacity:
    storage: 5Gi
  volumeMode: Filesystem
  accessModes:
    - ReadWriteMany
  persistentVolumeReclaimPolicy: Retain
  storageClassName: juicefs
  csi:
    driver: csi.juicefs.com
    volumeHandle: $(JUICEFS_NAME)
    fsType: juicefs
    nodePublishSecretRef:
      name: $(JUICEFS_AUTH_SECRET_NAME)
      namespace: $(JUICEFS_AUTH_SECRET_NAMESPACE)
```

Replace variables in `$(...)` value with actual value.

- `JUICEFS_NAME`: JuiceFS filesystem name pre-created in [juicefs web console](https://juicefs.com/console)
- `JUICEFS_AUTH_SECRET_NAME`: secret containing JuiceFS `token` and optionally `accesskey` and `secretkey` for `juicefs auth`
- `JUICEFS_AUTH_SECRET_NAMESPACE`: namespace of the secret for `juicefs auth`

### Apply the Example

Create storage class, PV, persistence volume claim (PVC) and sample pod

```sh
>> kubectl apply -f resources
```

In the example, both pods are writing to the same JuiceFS filesystem at the same time.

### Check JuiceFS filesystem is used

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app-1 -- tail -f /data/out-1.txt
>> kubectl exec -ti juicefs-app-2 -- tail -f /data/out-2.txt
```
