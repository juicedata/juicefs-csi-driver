# 使用 `librados` 访问 Ceph 集群

如果对象存储使用的是 [Ceph](https://ceph.io/) ，我们可以使用 [S3 RESTful API](https://docs.ceph.com/en/latest/radosgw/s3/)
来访问 [Ceph Object Gateway (RGW)](https://docs.ceph.com/en/latest/radosgw/) 。JuiceFS
支持使用 [`librados`](https://docs.ceph.com/en/latest/rados/api/librados/) 来访问存储，这种方式性能更高。

从 v0.10.0 版本开始，JuiceFS CSI Driver 支持向 JuiceFS
提供配置文件，更多细节可以参考文档 ["static-provisioning-config-and-env"](../../examples/static-provisioning-config-and-env/)。
利用这种机制，我们可以将 `/etc/ceph` 路径下的 Ceph Client 配置文件传给运行在 Kubernetes 中的 JuiceFS mount 进程。

这篇文档来描述如何在 Kubernetes 中使用 `librados` 访问 Ceph 集群。

## 使用 Ceph 存储创建 JuiceFS volume

假设我们有一个 Ceph 集群，在任意一台节点上，查看 `/etc/ceph` 路径下的文件：

```
/etc/ceph/
├── ceph.client.admin.keyring
├── ceph.conf
├── ...
└── ...
```

在这个节点上，我们创建一个 JuiceFS volume `ceph-volume`。关于 Ceph RADOS `--access-key` 和 `--secret-key`
的更多细节，可以参考 [这篇文档](https://github.com/juicedata/juicefs/blob/main/docs/en/how_to_setup_object_storage.md#ceph-rados) 。

```sh
$ ./juicefs format --storage=ceph \
    --bucket=ceph://ceph-test \
    --access-key=ceph \
    --secret-key=client.admin \
    redis://juicefs-redis.example.com/2 \
    ceph-volume

$ ceph osd pool ls
ceph-test
```

> **注**：这里我们假设 Redis URL 为 `redis://juicefs-redis.example.com/2`，需要将其换成您自己环境中的参数。

## 根据 Ceph 配置文件创建 Secret

在 Ceph 所在节点创建一个名为 `ceph-conf.yaml` 的 YAML 文件，执行以下命令，将 `CEPH_CLUSTER_NAME` 替换成掉：

```yaml
$ cat > ceph-conf.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ceph-conf
  namespace: kube-system
type: Opaque
data:
  <CEPH_CLUSTER_NAME>.conf: $(base64 -w 0 /etc/ceph/ceph.conf)
  <CEPH_CLUSTER_NAME>.client.admin.keyring: $(base64 -w 0 /etc/ceph/ceph.client.admin.keyring)
  EOF
```

> **Note**: 行首的 `$` 是 shell 提示符。`base64` 命令是必需的，如果不存在，请尝试使用您的操作系统包管理器安装 `coreutils` 包，例如 `apt-get` 或 `yum`。

将生成出来的 `ceph-conf.yaml` 文件 apply 到 Kubernetes 集群中：

```bash
$ kubectl apply -f ceph-conf.yaml

$ kubectl -n kube-system describe secret ceph-conf
Name:         ceph-conf
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
ceph.client.admin.keyring:  63 bytes
ceph.conf:                  257 bytes
```

## 创建 JuiceFS CSI Driver 需要的 Secret

```sh
$ cat > juicefs-secret.yaml <<EOF
apiVersion: v1
metadata:
  name: juicefs-secret
  namespace: kube-system
kind: Secret
type: Opaque
data:
  bucket: $(echo -n ceph://ceph-test | base64 -w 0)
  metaurl: $(echo -n redis://juicefs-redis.example.com/2 | base64 -w 0)
  name: $(echo -n ceph-volume | base64 -w 0)
  storage: $(echo -n ceph | base64 -w 0)
  access-key: $(echo -n ceph | base64 -w 0)
  secret-key: $(echo -n client.admin | base64 -w 0)
  configs: $(echo -n '{"ceph-conf": "/etc/ceph"}' | base64 -w 0)
EOF

$ kubectl apply -f juicefs-secret.yaml
secret/juicefs-secret created

$ kubectl -n kube-system describe secret juicefs-secret
Name:         juicefs-secret
Namespace:    kube-system
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
access-key:  4 bytes
bucket:      16 bytes
configs:     26 bytes
metaurl:     35 bytes
name:        11 bytes
secret-key:  12 bytes
storage:     4 bytes
```

因为我们希望我们之前创建的 `ceph-conf` Secret 被挂载到 `/etc/ceph` 下，我们构造了一个 Key 为 `configs` 的 JSON 字符串 `{"ceph-conf": "/etc/ceph"}`。

## 在 Kubernetes pod 中访问 JuiceFS volume

### 动态挂载

如何使用 StorageClass 访问 JuiceFS，请参考 ["examples/dynamic-provisioning"](../../examples/dynamic-provisioning/resources.yaml)
将 `$(SECRET_NAME)` 替换为 `juicefs-secret`，将 `$(SECRET_NAMESPACE)` 替换为 `kube-system`。

### 静态挂载

如何使用 Persistent Volume 访问
JuiceFS，请参考 ["examples/static-provisioning"](../../examples/static-provisioning/resources.yaml)
将 `nodePublishSecretRef` 的 `name` 和 `namespace` 替换为 `juicefs-sceret` 和 `kube-system`。

## 其他 Ceph 版本

最新的 JuiceFS 版本只支持 Ceph 12。如果您的 Ceph 版本高于 12，您需要自己构建镜像。

### 如何构建

我们使用官方的  [ceph/ceph](https://hub.docker.com/r/ceph/ceph) 作为基础镜像。根据
Ceph [Nautilus](https://docs.ceph.com/en/latest/releases/nautilus/) 构建 JuiceFS CSI Driver 镜像：

```bash
docker build --build-arg BASE_IMAGE=ceph/ceph:v14 --build-arg JUICEFS_REPO_TAG=v0.16.2 -f ceph.Dockerfile -t juicefs-csi-driver:ceph-nautilus .
```

`ceph/ceph:v14` 镜像是 ceph nautilus 的官方 ceph 镜像。对于其他 ceph 发布基础镜像，请参考 [repository](https://hub.docker.com/r/ceph/ceph) 。
