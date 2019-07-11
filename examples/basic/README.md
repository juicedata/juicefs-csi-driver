# Basic

This example shows how a basic example to use JuiceFS in Kubernetes pod

1. Create JuiceFS filesystem in [JuiceFS web console](https://juicefs.com/console)
2. Enter credentials for juicefs and object storage in `k8s.yaml`

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

Verify that data is written onto JuiceFS filesystem:

```sh
>> kubectl exec -ti juicefs-app -- tail -f /data/out.txt
```

Check for the directory created as PV https://juicefs.com/console/vol/<name>/
