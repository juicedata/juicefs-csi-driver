# Develop

Please go through [CSI Spec](https://github.com/container-storage-interface/spec/blob/master/spec.md) and [General CSI driver development guideline](https://kubernetes-csi.github.io/docs/Development.html) to get some basic understanding of CSI driver before you start.

## Requirements

* Golang 1.13+

## Dependency

Dependencies are managed through go module. To build the project, first turn on go mod using `export GO111MODULE=on`, to build the project run: `make`

Minikube is required for local development.

## Development workflow

```sh
make
make test
make image-dev
make push-dev
make deploy-dev
```

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

Use [stern](https://github.com/wercker/stern) to aggregate logs from all `juicefs-csi-driver` containers except `liveness-probe`.

```sh
stern -n kube-system -l juicefs-csi-driver=master --exclude-container liveness-probe
```

`juicefs-csi-driver` **MUST** be deployed to namespace like `kube-system` which supports `system-cluster-critical` priority class.

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

### Using Minikube

When using [minikube](https://www.github.com/kubernetes/minikube) for local development, modify default docker registry as below so that images can be available for minikube

```sh
eval $(minikube docker-env)
```

You may also need to create `plugin_registry` directory manually

```sh
minikube ssh
sudo mkdir -p /var/lib/kubelet/plugins_registry/
```
