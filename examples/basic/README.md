# Basic

This example shows how a basic example to use JuiceFS in Kubernetes pod.

## Prerequisite

Create JuiceFS filesystem using [juicefs format](https://github.com/juicedata/juicefs/#format-a-volume). Take Amazon S3 as an example:

```
./juicefs format --storage=s3 --bucket=https://<BUCKET>.s3.<REGION>.amazonaws.com redis://user:password@<HOST>:6379/1 <NAME>
```



Then create secret for access in Kubernetes.

```sh
>> kubectl -n default create secret generic juicefs-secret --from-literal=metaurl=redis://user:password@<HOST>:6379/1
```

## Apply the Example

Create storage class, persistence volume claim (PVC) and sample pod

```sh
>> kubectl apply -f k8s.yaml
```

The persisten volume will be dynamically provisioned as a directory in the JuiceFS filesystem configured in storage class.

## Check JuiceFS filesystem is used

After all objects are created, verify that a 10 Pi PV is created:

```sh
kubectl get pv
```

Verify the pod is running:

```sh
>> kubectl get pods
```

Verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

Verify the directory created as PV in `https://juicefs.com/console/vol/{name}/`
