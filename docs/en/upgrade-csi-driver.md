# Upgrade JuiceFS CSI Driver

## CSI Driver version >= v0.10

Juicefs CSI Driver separated JuiceFS client from CSI Driver since v0.10.0, CSI Driver upgrade will not interrupt
existing PVs.

### v0.13.0

Compared with v0.12.0, v0.13.0 updated the ClusterRole of CSI node & controller, and cannot directly update the image.

1. modify your `k8s.yaml` to [v0.13.0](https://github.com/juicedata/juicefs-csi-driver/blob/master/deploy/k8s.yaml), then run `kubectl apply -f k8s.yaml`.
2. Alternatively, if JuiceFS CSI driver is installed using Helm, you can also use Helm to upgrade it.

   ```bash
   helm repo update
   helm upgrade juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver -n kube-system -f ./values.yaml
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

### Cross-version upgrade

If you want to upgrade CSI Driver from v0.9.0 to v0.10.0+, follow ["How to upgrade CSI Driver from v0.9.0 to v0.10.0+"](upgrade-csi-driver-from-0.9-to-0.10.md).

### Other

For versions prior to v0.10.0, you can upgrade only the JuiceFS client without upgrading the CSI Driver. For details, refer to [this document](upgrade-juicefs.md).

Visit [Docker Hub](https://hub.docker.com/r/juicedata/juicefs-csi-driver) for more versions.
