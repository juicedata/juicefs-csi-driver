# TencentCloud COS

Tencent Cloud COS requires `AppID` as suffix of bucket.

## Prerequisite

Then create secret for access in Kubernetes.

```sh
kubectl create secret generic juicefs-secret \
    --from-literal=name=<NAME> \
    --from-literal=metaurl=<metaurl> \
    --from-literal=access-key=<access-key> \
    --from-literal=secret-key=<secret-key> \
    --from-literal=storage=<storage> \
    --from-literal=bucket=<bucket>
```

## Apply the Example

Create storage class, persistence volume claim (PVC) and sample pod

```sh
kubectl apply -f k8s.yaml
```

The persisten volume will be dynamically provisioned as a directory in the JuiceFS filesystem configured in storage class.

## Check JuiceFS filesystem is used

After all objects are created, verify that a 10 Pi PV is created:

```sh
kubectl get pv
```

Verify the pod is running:

```sh
kubectl get pods
```

Verify that data is written onto JuiceFS filesystem:

```sh
kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```
