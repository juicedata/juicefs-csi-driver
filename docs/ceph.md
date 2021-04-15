# Build CSI with librados support

If the object storage is [Ceph](https://ceph.io/), we can access [radosgw](https://docs.ceph.com/en/latest/radosgw/) using [S3 RESTful API](https://docs.ceph.com/en/latest/radosgw/s3/) . JuiceFS support using [librados](https://docs.ceph.com/en/latest/rados/api/librados/) which radosgw is built on to access the storage, this increases the performance as it reduce the radosgw layer.



## How to build

We use the official [ceph/ceph](https://hub.docker.com/r/ceph/ceph) as the base image. If we want to build JuiceFS CSI from Ceph [Nautilus](https://docs.ceph.com/en/latest/releases/nautilus/) :

```bash
docker build --build-arg BASE_IMAGE=ceph/ceph:v14 -f Dockerfile.ceph -t juicefs-csi-driver:ceph-nautilus .
```

The `ceph/ceph:v14` image is the official ceph image for ceph nautilus. For other ceph release base images, see the [repository](https://hub.docker.com/r/ceph/ceph) .



## How to deploy

If we want to deploy JuiceFS CSI with librados support, we have to supply the configuration files in `/etc/ceph/` for JuiceFS to access ceph cluster.

Use the above CSI image as our base image, add the configuration files in `/etc/ceph/` of the ceph cluster into the base image `/etc/ceph/` directory to generate a new CSI image. Use this CSI image to replace `juicedata/juicefs-csi-driver` in the [k8s.yaml](../deploy/k8s.yaml) .

