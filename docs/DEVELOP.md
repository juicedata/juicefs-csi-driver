# Develop

Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

## Requirements

* Golang 1.11.4+

## Dependency

Dependencies are managed through go module. To build the project, first turn on go mod using `export GO111MODULE=on`, to build the project run: `make`

## Build container image

```s
make image-dev
make push-dev
```

## Testing

To execute all unit tests, run: `make test`

## Troubleshooting

If the application pod is hanging in `ContainerCreating` status for a long time, e.g.

```s
$ kubectl get pods
NAME            READY     STATUS              RESTARTS   AGE
juicefs-app-1   0/1       ContainerCreating   0          10m
juicefs-app-2   0/1       ContainerCreating   0          10m
```

Describe it to see the events, e.g.

```s
$ kubectl describe pod juicefs-app-1
Name:               juicefs-app-1
Namespace:          juicefs-csi-demo
...
Events:
  Type     Reason              Age                From                                              Message
  ----     ------              ----               ----                                              -------
  Normal   Scheduled           12m                default-scheduler                                 Successfully assigned juicefs-csi-demo/juicefs-app-1 to ip-10-0-0-31.us-west-2.compute.internal
  Warning  FailedMount         1m (x5 over 10m)   kubelet, ip-10-0-0-31.us-west-2.compute.internal  Unable to mount volumes for pod "juicefs-app-1_juicefs-csi-demo(45654a9b-6fee-11e9-aee6-06b5b6616e3c)": timeout expired waiting for volumes to attach or mount for pod "juicefs-csi-demo"/"juicefs-app-1". list of unmounted volumes=[persistent-storage]. list of unattached volumes=[persistent-storage default-token-xjj8k]
  Warning  FailedAttachVolume  1m (x12 over 12m)  attachdetach-controller                           AttachVolume.Attach failed for volume "juicefs-csi-demo" : attachment timeout for volume csi-demo
```

Check the logs of the following components

* `juicefs-csi-node`
* `juicefs-csi-attacher`
* `kube-controller-manager`
* `kubelet`

`juicefs-csi-driver` **MUST** be deployed to namespace like `kube-system` which supports `system-cluster-critical` priority class.

### kubelet

```s
sudo journalctl -u kubelet -f
```

#### Orphaned pod

```s
May 12 09:58:03 ip-172-20-48-5 kubelet[1028]: E0512 09:58:03.411256    1028 kubelet_volumes.go:154] Orphaned pod "e7d422a7-7495-11e9-937d-0adc9bc4231a" found, but volume paths are still present on disk : There were a total of 1 errors similar to this. Turn up verbosity to see them.
```

Workaround

```s
$ sudo su
# cd /var/lib/kubelet/pods
# rm -rf e7d422a7-7495-11e9-937d-0adc9bc4231a/volumes/kubernetes.io~csi/
```

#### AttachVolume.Attach failed

```s
May 25 17:20:04 iZuf65o45s4xllq6ghmvkhZ kubelet[1458]: I0525 17:20:04.644217    1458 reconciler.go:227] operationExecutor.AttachVolume started for volume "juicefs" (UniqueName: "kubernetes.io/csi/csi.juicefs.com^csi-demo") pod "juicefs-app-1" (UID: "47b8a4e9-7ece-11e9-becf-00163e0e041d")
May 25 17:20:04 iZuf65o45s4xllq6ghmvkhZ kubelet[1458]: E0525 17:20:04.648763    1458 csi_attacher.go:105] kubernetes.io/csi: attacher.Attach failed: volumeattachments.storage.k8s.io is forbidden: User "system:node:cn-shanghai.192.168.0.186" cannot create resource "volumeattachments" in API group "storage.k8s.io" at the cluster scope
May 25 17:20:04 iZuf65o45s4xllq6ghmvkhZ kubelet[1458]: E0525 17:20:04.648831    1458 nestedpendingoperations.go:267] Operation for "\"kubernetes.io/csi/csi.juicefs.com^csi-demo\"" failed. No retries permitted until 2019-05-25 17:20:05.148793189 +0800 CST m=+187.223201321 (durationBeforeRetry 500ms). Error: "AttachVolume.Attach failed for volume \"juicefs\" (UniqueName: \"kubernetes.io/csi/csi.juicefs.com^csi-demo\") from node \"cn-shanghai.192.168.0.186\" : volumeattachments.storage.k8s.io is forbidden: User \"system:node:cn-shanghai.192.168.0.186\" cannot create resource \"volumeattachments\" in API group \"storage.k8s.io\" at the cluster scope"
```

Some service provider e.g. Alibaba cloud set `--enable-controller-attach-detach=false` for the Flexvolume feature. It needs to be set `true` for kubelet in order to use a CSI driver:

SSH login worker node

```s
# vi /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
# systemctl daemon-reload
# systemctl restart kubelet
```

Refer to [Alibaba Cloud Kubernetes CSI Plugin#Config Kubelet](https://github.com/AliyunContainerService/csi-plugin/tree/v0.3.0#config-kubelet).

To SSH login worker node, you may need to

* set node password from web console and restart to make it effective
* login master node first if worker node does not have a public IP
