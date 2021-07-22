# Access ceph cluster with librados

If the object storage is [Ceph](https://ceph.io/), we can access [radosgw](https://docs.ceph.com/en/latest/radosgw/) using [S3 RESTful API](https://docs.ceph.com/en/latest/radosgw/s3/) . JuiceFS support using [librados](https://docs.ceph.com/en/latest/rados/api/librados/) which radosgw is built on to access the storage, this increases the performance as it reduce the radosgw layer.

Since version `v0.10.0` , JuiceFS CSI support supplying configuration files to JuiceFS, read the [static-provisioning-config-and-env](../examples/static-provisioning-config-and-env/) example for more details. With this mechanism, we can transfer ceph client configuration files under `/etc/ceph` JuiceFS mount process running in Kubernetes.

Here we demonstrate how to access ceph cluster with librados in Kubernetes.

## Create JuiceFS volume using Ceph storage

Assume we have an ceph cluster, and in one node of this cluster, list the content of `/etc/ceph` :

```
/etc/ceph/
├── ceph.client.admin.keyring
├── ceph.conf
├── ...
└── ...
```

With `ceph.conf` and `ceph.client.admin.keyring` , we can access ceph cluster with librados.

On this node, we create a new JuiceFS volume `ceph-volume` , for more details about the `access-key` and `secret-key` of ceph rados, refer [here](https://github.com/juicedata/juicefs/blob/main/docs/en/how_to_setup_object_storage.md#ceph-rados).

```
# ./juicefs format --storage=ceph \
                 --bucket=ceph://ceph-test \
                 --access-key=ceph \
                 --secret-key=client.admin \
                 redis://juicefs-redis.example.com/2 \
                 ceph-volume
# ceph osd pool ls
ceph-test
```

**Note**: Here we assume the Redis URL is `redis://juicefs-redis.example.com/2`  , replace it with your own.

## Create secret for ceph configuration files

Create a YAML file `ceph-conf.yaml` on the same ceph node where above commands executed:

```yaml
# cat > ceph-conf.yaml <<EOF
apiVersion: v1
data:
  ceph.conf: $(base64 -w 0 /etc/ceph/ceph.conf)
  ceph.client.admin.keyring: $(base64 -w 0 /etc/ceph/ceph.client.admin.keyring)
kind: Secret
metadata:
  name: ceph-conf
  namespace: kube-system
type: Opaque
EOF
```

**Note**:  The `#` at the beginning of line is the shell prompt. `base64` command is required, if it isn't present, try to install `coreutils` package with your OS package manager such as `apt` or `yum` .

Apply the generated `ceph-conf.yaml` to the Kubernetes cluster:

```bash
# kubectl apply -f ceph-conf.yaml
# kubectl -n kube-system describe secret ceph-conf
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

**Note**: The `#` at the beginning of line is the shell prompt.

## Create secret for JuiceFS CSI

```bash
# cat >juicefs-secret.yaml <<EOF
apiVersion: v1
metadata:
  name: juicefs-secret
  namespace: kube-system
data:
  bucket: $(echo -n ceph://ceph-test | base64 -w 0)
  metaurl: $(echo -n redis://juicefs-redis.example.com/2 | base64 -w 0)
  name: $(echo -n ceph-volume | base64 -w 0)
  storage: $(echo -n ceph | base64 -w 0)
  access-key: $(echo -n ceph | base64 -w 0)
  secret-key: $(echo -n client.admin | base64 -w 0)
  configs: $(echo -n '{"ceph-conf": "/etc/ceph"}' | base64 -w 0)
kind: Secret
type: Opaque
EOF
# kubectl apply -f juicefs-secret.yaml
secret/juicefs-secret created
# kubectl -n kube-system describe secret juicefs-secret
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

**Note**:  The `#` at the beginning of line is the shell prompt.

As we want the `ceph-conf` secret we create before to be mounted under `/etc/ceph` , we construct a json string `{"ceph-conf": "/etc/ceph"}` for the key `configs` .

## Access JuiceFS volume in Kubernetes pod

### Dynamic provisioning

Please refer [examples/dynamic-provisioning](../examples/dynamic-provisioning/resources.yaml) for how to access JuiceFS using storage class. Replace `$(SECRET_NAME)` with `juicefs-secret` and `$(SECRET_NAMESPACE)` with `kube-system` .

### Static provisioning

Please refer [examples/static-provisioning](../examples/static-provisioning/resources.yaml) for how to access JuiceFS using storage class. Replace  `name` and `namespace` of `nodePublishSecretRef` with `juicefs-sceret` and `kube-system` .

