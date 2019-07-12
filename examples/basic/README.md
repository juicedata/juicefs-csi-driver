# Basic

This example shows how a basic example to use JuiceFS in Kubernetes pod.

## Prerequisite

Create JuiceFS filesystem in [JuiceFS web console](https://juicefs.com/console)

Then create secret for access in Kubernetes.

```sh
>> kubectl create secret generic juicefs-secret --from-literal=name=$JFS_NAME --from-literal=token=$JFS_TOKEN --from-literal=accesskey=$JFS_ACCESSKEY --from-literal=secretkey=$JFS_SECRETKEY
```

JuiceFS token can be found in `https://juicefs.com/console/vol/{name}/setting`

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
