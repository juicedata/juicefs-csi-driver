# FAQs
Some common questions are collected in this document.

## Q: JuiceFS csi driver has been deployed, but my pod created error with message "driver name csi.juicefs.com not found in the list of registered CSI drivers"

1. Check the root directory path of `kubelet`. Run the following command on any non-master node in your Kubernetes cluster:

```shell
ps -ef | grep kubelet | grep root-dir
```

If the result isn't empty, modify the CSI driver deployment `k8s.yaml` file with the new path and redeploy the CSI driver again.

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> **Note**: please replace `{{KUBELET_DIR}}` in the above command with the actual root directory path of kubelet.

2. Check if juicefs node service running in the node your pod running in.

Refer to [issue](https://github.com/juicedata/juicefs-csi-driver/issues/177)

Run the following command to check:

```shell
kubectl -n kube-system get po -owide | grep juicefs
kubectl get po -owide -A | grep <your_pod>
```

If there are no juicefs csi node running in the node where your pod in, check if the node has some taints. You can delete the taint or add the relevant tolerance in csi node deamonset and redeploy it.

