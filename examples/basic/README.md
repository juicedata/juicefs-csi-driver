# Basic

This example shows how a basic example to use JuiceFS in Kubernetes pod

1. Create JuiceFS filesystem in [JuiceFS web console](https://juicefs.com/console)
2. Create Secret with `kubectl create secret generic juicefs --from-literal=token=${JUICEFS_TOKEN} --from-literal=accesskey=${JUICEFS_ACCESSKEY} --from-literal=secretkey=${JUICEFS_SECRETKEY}`
3. Edit `k8s.yaml` and replace the values `*-not-provided`

Note:

* `PersistentVolume/spec/csi/volumeHandler` should be name of a pre-created JuiceFS filesystem
* `PersistentVolume/spec/csi/nodePublishSecretRef/name` should be the name of the secret

## Apply the Example

Create storage class, PV, persistence volume claim (PVC) and sample pod

```sh
>> kubectl apply -f k8s.yaml
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
