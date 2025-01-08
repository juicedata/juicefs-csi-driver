---
slug: /custom-etc-passwd-group
description: Learn how to configure customized /etc/passwd and /etc/group files in JuiceFS Mount Pods to resolve UID/GID inconsistencies. 
---

# Use Customized /etc/passwd and /etc/group in Mount Pods

For enterprise edition users who have enabled the UID/GID auto-mapping feature, mounting both on the host machine and in the Kubernetes Pod may lead to [UID/GID inconsistencies](https://juicefs.com/docs/cloud/guide/guid_auto_map/#uidgid-inconsistency) due to inconsistencies between `/etc/passwd` and `/etc/group`.

In such cases, configuring the CSI Mount Pod with customized `/etc/passwd` and `/etc/group` files that match those of the host machine ensures consistent UID/GID mappings.

## Create a Secret based on host configuration

The following commands read the host machine's `/etc/passwd` and `/etc/group` to generate the Kubernetes Secret used by the Mount Pod.

```bash
$ kubectl create secret generic juicefs-uid-gid --from-file=passwd=/etc/passwd --from-file=group=/etc/group 
$ kubectl describe secret juicefs-uid-gid
Name:         juicefs-uid-gid
Namespace:    default
Labels:       <none>
Annotations:  <none>

Type:  Opaque

Data
====
group:   882 bytes
passwd:  1898 byte
```

## Configure the Mount Pod

By default, the Mount Pod has already redirected `/etc/passwd` and `/etc/group` to symbolic links pointing to `~/.acl/passwd` and `~/.acl/group`.

``` bash
$ ls -l /etc/ | grep acl
lrwxrwxrwx 1 root root      16 Aug 27 04:49 group -> /root/.acl/group
lrwxrwxrwx 1 root root      17 Aug 27 04:49 passwd -> /root/.acl/passwd
```

Simply mount the Secret to `/root/.acl`. Refer to [Adding extra files into the Mount Pod](../guide/pv.md#mount-pod-extra-files) to include the corresponding field `configs: "{juicefs-uid-gid: /root/.acl}"`.
