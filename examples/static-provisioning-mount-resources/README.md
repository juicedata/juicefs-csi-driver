# Mount Resources

This example shows how to apply mount resources to JuiceFS PersistentVolume (PV).

## Patches

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example. Make sure the CSI driver version is above v0.10.0.

The CSI driver support to set resource limits/requests of mount pod.

Patch the PersistentVolume spec with `csi/volumeAttributes`.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
spec:
  csi:
    volumeAttributes:
      juicefs/mount-cpu-limit: 5000m
      juicefs/mount-memory-limit: 5Gi
      juicefs/mount-cpu-request: 1000m
      juicefs/mount-memory-request: 1Gi
```

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`:

```s
kustomize build | kubectl apply -f -
```

## Check mount resources are customized

After the configuration is applied, verify that pod is running:

```sh
kubectl get pods juicefs-app
```

Then check mount pod is running:

```shell
kubectl -n kube-system get pods
```

Also you can verify that mount resources are customized in mount pod:

```sh
kubectl -n kube-system get po juicefs-kube-node-2-test-bucket -o yaml | grep -A 6 resources
```
