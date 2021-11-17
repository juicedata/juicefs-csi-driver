# FAQ

## Pod 创建失败，错误信息 "driver name csi.juicefs.com not found in the list of registered CSI drivers"

1. 检查 `kubelet` 的 root-dir

在任意非 master 节点执行以下命令：

```shell
ps -ef | grep kubelet | grep root-dir
```

**如果前面检查命令返回的结果不为空**，则代表 kubelet 的 root-dir 路径不是默认值，因此需要在 CSI Driver 的部署文件中更新 `kubeletDir` 路径并部署：

```shell
# Kubernetes version >= v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

# Kubernetes version < v1.18
curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
```

> 注意：请将上述命令中 `{{KUBELET_DIR}}` 替换成 kubelet 实际的根目录路径。

2. 检查 pod 所在节点是否运行有 juicefs node service

参考 [issue](https://github.com/juicedata/juicefs-csi-driver/issues/177)，运行如下命令：

```shell
kubectl -n kube-system get po -owide | grep juicefs
kubectl get po -owide -A | grep <your_pod>
```

如果您的 pod 所在节点上没有 juicefs csi node，请检查该节点是否存在污点。您可以删除污点，或给 JuiceFS csi driver DaemonSet 打上对应的容忍，并重新部署。

## 两个 pod 分别使用各自的 PVC，但只有一个能创建成功。

请检查每个 PVC 对应的 PV，每个 PV 的 volumeHandle 必须保证唯一。您可以通过以下命令检查 volumeHandle：

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

