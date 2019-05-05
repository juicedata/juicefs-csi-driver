# Static Provisioning

This example shows how to make a static provisioned JuiceFS persistence volume (PV) mounted inside container.

## Provide secret information

1. Copy `sample-Secret-juicefs-csi-demo.env` to `Secret-juicefs-csi-demo.env`
2. Enter actual values of `name` and `token` of the pre-created JuiceFS filesystem
3. Enter `accesskey` and `secretkey` for object storage service used by this JuiceFS filesystem if needed

## Deploy Using kustomize

[kustomize](https://github.com/kubernetes-sigs/kustomize) is a builtin plugin since kubectl **1.14**.

1. Copy `sample-Secret-juicefs-csi-demo.env` to `Secret-juicefs-csi-demo.env`
2. Enter actual values of `name` and `token` of the pre-created JuiceFS filesystem
3. Enter `accesskey` and `secretkey` for object storage service used by this JuiceFS filesystem if needed

```sh
kustomize build | kubectl apply -f -
```

or with kubectl >= 1.14

```sh
kubectl apply -k -f .
```

## Deploy Using kubectl < 1.14 Without kustomize

### Create Secret for Node Publish Volume

```sh
kubectl create secret generic juicefs-csi-demo --from-file=Secret-juicefs-csi-demo.env
```

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
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: juicefs
  csi:
    driver: csi.juicefs.com
    volumeHandle: $(JUICEFS_NAME)
    fsType: juicefs
    nodePublishSecretRef:
      name: $(JUICEFS_SECRET_NAME)
      namespace: $(JUICEFS_SECRET_NAMESPACE)
```

Replace variables in `$(...)` value with actual value.

- `JUICEFS_NAME`: JuiceFS filesystem name pre-created in [juicefs web console](https://juicefs.com/console)
- `JUICEFS_SECRET_NAME`: secret containing JuiceFS `token` and optionally `accesskey` and `secretkey` for object storage
- `JUICEFS_SECRET_NAMESPACE`: namespace of the secrets for `juicefs auth` or `juicefs mount`

### Apply the Example

Create storage class, PV, persistence volume claim (PVC) and sample pod

```sh
>> kubectl apply -f resources
```

### Check JuiceFS filesystem is used

After the objects are created, verify that pod is running:

```sh
>> kubectl get pods
```

Also you can verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```
