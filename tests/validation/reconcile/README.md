# reconcile

JuiceFS volumes should reconcile when node or controller crash and reconcile.

## Steps

```sh
kubectl apply -k .
kubectl -n default delete pods -l app=juicefs-csi-node
```

## Expected

Application pods reconcile

```sh
kubectl -n default get pods -l juicefs-csi-driver/validation=reconcile
stern -n default -l juicefs-csi-driver/validation=reconcile
```
