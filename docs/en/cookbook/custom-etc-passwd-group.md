---
slug: /custom-etc-passwd-group
---

# Use customized /etc/passwd and /etc/group in Mount Pod

If enterprise edition users enable the UID/GID auto map feature, when mounting simultaneously in both the host machine and Kubernetes Pods, inconsistencies between `/etc/passwd` and `/etc/group` can often lead to encountering the [UID/GID inconsistency](https://juicefs.com/docs/cloud/guide/guid_auto_map/#uidgid-inconsistency).

At this point, it is necessary to ensure UID/GID consistency by customizing `/etc/passwd` and `/etc/group` to be the same for the CSI Mount Pod as in the host machine.

## Create Secret Based on Host Configuration

The following commands will read the host machine's `/etc/passwd` and `/etc/group` to generate the Kubernetes Secret used by the Mount Pod.

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

## Configure Mount Pod

By default, our Mount Pod has already redirected `/etc/passwd` and `/etc/group` to symbolic links pointing to `~/.acl/passwd` and `~/.acl/group`.

``` bash
$ ls -l /etc/ | grep acl
lrwxrwxrwx 1 root root      16 Aug 27 04:49 group -> /root/.acl/group
lrwxrwxrwx 1 root root      17 Aug 27 04:49 passwd -> /root/.acl/passwd
```

Just simply mount the Secret to `/root/.acl`, referring to [Adding extra files into Mount Pod](../guide/pv.md#mount-pod-extra-files) to include the corresponding field `configs: "{juicefs-uid-gid: /root/.acl}"`.
