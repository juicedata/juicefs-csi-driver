# Access Ceph Cluster with librados

If you use [Ceph](https://ceph.io/) as the underlying storage for JucieFS, you can either use the standard [S3 RESTful API](https://docs.ceph.com/en/latest/radosgw/s3/) to access the [Ceph Object Gateway (RGW)](https://docs.ceph.com/en/latest/radosgw/), or the more efficient [`librados`](https://docs.ceph.com/en/latest/rados/api/librados/ ) to access Ceph storage.

Since version v0.10.0, JuiceFS CSI Driver supports supplying configuration files to JuiceFS, read the ["How to set config files and environment in mount pod"](../examples/config-and-env.md) example for more details. With this mechanism, we can transfer Ceph client configuration files under `/etc/ceph` JuiceFS mount process running in Kubernetes.

Here we demonstrate how to access Ceph cluster with `librados` in Kubernetes.

## Create JuiceFS volume using Ceph storage

Assume we have an Ceph cluster, and in one node of this cluster, list the content of `/etc/ceph`:

```
/etc/ceph/
├── ceph.client.admin.keyring
├── ceph.conf
├── ...
└── ...
```

With `ceph.conf` and `ceph.client.admin.keyring`, we can access Ceph cluster with `librados`.

On this node, we create an new JuiceFS volume `ceph-volume`

```sh
juicefs format --storage=ceph \
  --bucket=ceph://ceph-test \
  --access-key=ceph \
  --secret-key=client.admin \
  redis://juicefs-redis.example.com/2 \
  ceph-volume
```

:::note
Here we assume the Redis URL is `redis://juicefs-redis.example.com/2`, replace it with your own. For more details about the `--access-key` and `--secret-key` of Ceph RADOS, refer [How to Setup Object Storage](https://juicefs.com/docs/community/how_to_setup_object_storage#ceph-rados).
:::

View Ceph storage status.

```sh
$ ceph osd pool ls
ceph-test
```

## Create secret for Ceph configuration files

The following command creates a YAML file named `ceph-conf.yaml` on the node where Ceph is located, replacing `CEPH_CLUSTER_NAME` with the actual name.

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

:::note
The `$` at the beginning of line is the shell prompt. `base64` command is required, if it isn't present, try to install `coreutils` package with your OS package manager such as `apt-get` or `yum`.
:::

Apply the generated `ceph-conf.yaml` to the Kubernetes cluster:

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

## Create secret for JuiceFS CSI Driver

Create a Secret profile by referring to the following command.

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

Apply the configuration.

```sh
$ kubectl apply -f juicefs-secret.yaml
secret/juicefs-secret created
```

To see if the configuration is in effect.

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

As we want the `ceph-conf` secret we created before to be mounted under `/etc/ceph`, we construct a JSON string `{"ceph-conf": "/etc/ceph"}` for the key `configs` .

## Access JuiceFS volume in Kubernetes pod

### Dynamic provisioning

Please refer ["Dynamic Provisioning"](../examples/dynamic-provisioning.md) for how to access JuiceFS using storage class. Replace `$(SECRET_NAME)` with `juicefs-secret` and `$(SECRET_NAMESPACE)` with `kube-system`.

### Static provisioning

Please refer ["Static Provisioning"](../examples/static-provisioning.md) for how to access JuiceFS using static provisioning. Replace `name` and `namespace` of `nodePublishSecretRef` with `juicefs-sceret` and `kube-system`.

## Other Ceph versions

JuiceFS currently supports up to Ceph 12, if you are using a version of Ceph higher than 12, please refer to the following method to build the image.

### How to build Docker image

We use the official [ceph/ceph](https://hub.docker.com/r/ceph/ceph) as the base image. If we want to build JuiceFS CSI from Ceph [Nautilus](https://docs.ceph.com/en/latest/releases/nautilus/):

```bash
docker build --build-arg BASE_IMAGE=ceph/ceph:v14 --build-arg JUICEFS_REPO_TAG=v0.16.2 -f docker/ceph.Dockerfile -t juicefs-csi-driver:ceph-nautilus .
```

The `ceph/ceph:v14` image is the official Ceph image for Ceph Nautilus. For other Ceph release base images, see the [Ceph image repository](https://hub.docker.com/r/ceph/ceph) .
