---
slug: /ceph
---

# 使用 librados 访问 Ceph 集群

如果使用 [Ceph](https://ceph.io) 作为 JuiceFS 的底层存储，既可以使用标准的 [S3 RESTful API](https://docs.ceph.com/en/latest/radosgw/s3) 来访问 [Ceph Object Gateway（RGW）](https://docs.ceph.com/en/latest/radosgw)，也可以使用效率更高的 [`librados`](https://docs.ceph.com/en/latest/rados/api/librados) 访问 Ceph 存储。

JuiceFS CSI 驱动支持[「为 Mount Pod 额外添加文件」](../guide/pv.md#mount-pod-extra-files)。利用这种机制，可以将主机 `/etc/ceph` 路径下的 Ceph Client 配置文件导入 Mount Pod。

## 使用 Ceph 存储创建 JuiceFS volume

假设我们有一个 Ceph 集群，在任意一台节点上，查看 `/etc/ceph` 路径下的文件：

```
/etc/ceph/
├── ceph.client.admin.keyring
├── ceph.conf
├── ...
└── ...
```

通过 `ceph.conf` 和 `ceph.client.admin.keyring` 就可以用 `librados` 访问 Ceph 集群。

在这个节点上创建一个 JuiceFS volume `ceph-volume`：

```sh
juicefs format --storage=ceph \
  --bucket=ceph://ceph-test \
  --access-key=ceph \
  --secret-key=client.admin \
  redis://juicefs-redis.example.com/2 \
  ceph-volume
```

:::note 注意
这里我们假设 Redis URL 为 `redis://juicefs-redis.example.com/2`，需要将其换成您自己环境中的参数。关于 Ceph RADOS `--access-key` 和 `--secret-key` 的更多细节，可以参考 [JuiceFS 支持的对象存储和设置指南](https://juicefs.com/docs/zh/community/how_to_setup_object_storage#ceph-rados)。
:::

查看 Ceph 存储状态：

```sh
$ ceph osd pool ls
ceph-test
```

## 根据 Ceph 配置文件创建 Secret

以下命令会在 Ceph 所在节点创建一个名为 `ceph-conf.yaml` 的 YAML 文件，请将 `CEPH_CLUSTER_NAME` 替换成实际的名称：

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

:::note 注意
行首的 `$` 是 shell 提示符。`base64` 命令是必需的，如果不存在，请尝试使用您的操作系统包管理器安装 `coreutils` 包，例如 `apt-get` 或 `yum`。
:::

将生成出来的 `ceph-conf.yaml` 文件应用到 Kubernetes 集群中：

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

参考以下命令创建 Secret 配置文件：

```yaml
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
```

应用配置：

```sh
$ kubectl apply -f juicefs-secret.yaml
secret/juicefs-secret created
```

查看配置是否生效：

```sh
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

我们希望之前创建的 `ceph-conf` Secret 被挂载到 `/etc/ceph` 下，因此这里构造了一个 Key 为 `configs` 的 JSON 字符串 `{"ceph-conf": "/etc/ceph"}`。

## 在 Kubernetes Pod 中访问 JuiceFS volume

### 动态挂载

如何使用 StorageClass 访问 JuiceFS，请参考[「动态配置」](../guide/pv.md#dynamic-provisioning)将 `$(SECRET_NAME)` 替换为 `juicefs-secret`，将 `$(SECRET_NAMESPACE)` 替换为 `kube-system`。

### 静态挂载

如何使用 Persistent Volume 访问 JuiceFS，请参考[「静态配置」](../guide/pv.md#static-provisioning)将 `nodePublishSecretRef` 的 `name` 和 `namespace` 替换为 `juicefs-sceret` 和 `kube-system`。

## Ceph 版本 兼容

JuiceFS 目前对 Ceph 版本支持如下：

| JuiceFS 版本 | Ceph 版本          |
| ------------ | ------------------ |
| v1.0.x       | v12, v13, v14, v15 |
| v1.1.x       | v15, v16, v17      |
| v1.2.x       | v15, v16, v17      |

如果你使用的 Ceph 版本不在上述列表中，请参考以下方法构建镜像。

### 如何构建镜像

使用官方的 [`ceph/ceph`](https://hub.docker.com/r/ceph/ceph) 作为基础镜像，根据 Ceph [Nautilus](https://docs.ceph.com/en/latest/releases/nautilus) 构建 JuiceFS CSI Driver 镜像，例如：

```bash
docker build --build-arg BASE_IMAGE=ceph/ceph:v14 --build-arg JUICEFS_REPO_TAG=v0.16.2 -f docker/ceph.Dockerfile -t juicefs-csi-driver:ceph-nautilus .
```

`ceph/ceph:v14` 镜像是 Ceph Nautilus 的官方 Ceph 镜像，对于其他 Ceph 发布基础镜像，请参考 [Ceph 镜像仓库](https://hub.docker.com/r/ceph/ceph)。
