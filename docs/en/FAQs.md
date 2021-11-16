# FAQ
## Pod created error with message "driver name csi.juicefs.com not found in the list of registered CSI drivers"

1. Check the root path of `kubelet`.

Run the following command on any non-master node in your Kubernetes cluster:

```shell
ps -ef | grep kubelet | grep root-dir
```

**If the check command returns a non-empty result**, it means that the `root-dir` path of the kubelet is not the default, so you need to update the `kubeletDir` path in the CSI Driver's deployment file and deploy.

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> **Note**: please replace `{{KUBELET_DIR}}` in the above command with the actual root directory path of kubelet.

2. Check if juicefs node service running in the node your pod running in.

Refer to [issue](https://github.com/juicedata/juicefs-csi-driver/issues/177), run the following command to check:

```shell
kubectl -n kube-system get po -owide | grep juicefs
kubectl get po -owide -A | grep <your_pod>
```

If there are no juicefs csi node running in the node where your pod in, check if the node has some taints. You can delete the taint or add the relevant tolerance in csi node deamonset and redeploy it.

## Q: Two pods use PVC separately, but only one runs well and the other can't get up.

Please check the PV corresponding to each PVC, the `volumeHandle` must be unique for each PV. You can check volumeHandle with the following cmd:

```yaml
$ kubectl get pv -o yaml juicefs-pv
apiVersion: v1
kind: PersistentVolume
metadata:
  name: juicefs-pv
  ...
spec:
  ...
  csi:
    driver: csi.juicefs.com
    fsType: juicefs
    volumeHandle: juicefs-volume-abc
    ...
```
