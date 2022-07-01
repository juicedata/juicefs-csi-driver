---
slug: faq
---

# FAQ

## Pod created error with message "driver name csi.juicefs.com not found in the list of registered CSI drivers"

1. Check the root directory path of kubelet.

   Run the following command on any non-master node in your Kubernetes cluster:

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   **If the check command returns a non-empty result**, it means that the root directory (`--root-dir`) of the kubelet is not the default (`/var/lib/kubelet`), so you need to update the `kubeletDir` path in the CSI Driver's deployment file and deploy.

   ```shell
   # Kubernetes version >= v1.18
   curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

   # Kubernetes version < v1.18
   curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
   ```

   :::note
   Please replace `{{KUBELET_DIR}}` in the above command with the actual root directory path of kubelet.
   :::

2. Check if JuiceFS node service running in the node your pod running in.

   Refer to [issue #177](https://github.com/juicedata/juicefs-csi-driver/issues/177), run the following command to check:

   ```shell
   kubectl -n kube-system get po -owide | grep juicefs
   kubectl get po -owide -A | grep <your_pod>
   ```

   If there are no JuiceFS CSI node running in the node where your pod in, check if the node has some taints. You can delete the taint or add the relevant tolerance in CSI node DeamonSet and re-deploy it.

## Two pods use PVC separately, but only one runs well and the other can't get up.

Please check the PV corresponding to each PVC, the `volumeHandle` must be unique for each PV. You can check `volumeHandle` with the following command:

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
## Using Alibaba Cloud Redis service as metadata engine, pod created error with CSI node error message "format: ERR illegal address: xxxx"

Please check whether the node IP of the Kubernetes cluster is added to the whitelist of the Alibaba Cloud Redis service.

## CSI node pod error message "format: NOAUTH Authentication requested."

When using Redis as the metadata engine, the metadata engine URL needs to contain a password. For the specific format, please refer to [document](https://juicefs.com/docs/community/databases_for_metadata#redis).

## How to check the currently installed JuiceFS CSI Driver version?

Please refer to the ["Troubleshooting"](troubleshooting.md#check-juicefs-csi-driver-version) document for detailed steps.

## How to set the time zone of JuiceFS Mount Pod?

You need to add `envs: "{TZ: <YOUR-TIME-ZONE>}"` configuration in `stringData` of `Secret`, please replace `<YOUR-TIME-ZONE>` with the actual value (e.g. `Asia/Shanghai`). For specific examples, please refer to the ["Static Provisioning"](examples/static-provisioning.md) or ["Dynamic Provisioning"](examples/dynamic-provisioning.md) documentation.
