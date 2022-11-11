# Upgrade JuiceFS CSI Driver

Please check the [release notes](https://github.com/juicedata/juicefs-csi-driver/releases) page of the JuiceFS CSI Driver for information on all released versions.

## CSI Driver version >= v0.10

Juicefs CSI Driver separated JuiceFS client from CSI Driver since v0.10.0, CSI Driver upgrade will not interrupt existing PVs.

### Upgrade via Helm

Please run the following commands in sequence to upgrade the JuiceFS CSI Driver:

```bash
helm repo update
helm upgrade juicefs-csi-driver juicefs/juicefs-csi-driver -n kube-system -f ./values.yaml
```

### Upgrade via kubectl

Please modify the image label of the JuiceFS CSI Driver in [`k8s.yaml`](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml) to the version that needs to be upgraded (such as `v0.14.0`), then run the following command:

```sh
kubectl apply -f ./k8s.yaml
```

## CSI Driver version < v0.10

### Minor version upgrade

Upgrade of CSI Driver requires restart the DaemonSet, which has all the JuiceFS client running inside. The restart will
cause all PVs become unavailable, so we need to stop all the application pod first.

1. Stop all pods using this driver.
2. Upgrade driver:
    * If you're using `latest` tag, simple run `kubectl rollout restart -f k8s.yaml` and make
      sure `juicefs-csi-controller` and `juicefs-csi-node` pods are restarted.
    * If you have pinned to a specific version, modify your `k8s.yaml` to a newer version, then
      run `kubectl apply -f k8s.yaml`.
    * Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.
3. Start all the application pods.

### Major version upgrade

If you want to upgrade CSI Driver from v0.9.0 to v0.10.0 and above, follow ["How to upgrade CSI Driver from v0.9.0 to v0.10.0+"](upgrade-csi-driver-from-0.9-to-0.10.md).

### Other

For versions prior to v0.10.0, you can upgrade only the JuiceFS client without upgrading the CSI Driver. For details, refer to [this document](upgrade-juicefs.md).
