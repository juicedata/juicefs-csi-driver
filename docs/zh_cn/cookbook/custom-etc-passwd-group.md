---
slug: /custom-etc-passwd-group
---

# Mount Pod 使用定制的 /etc/passwd 和 /etc/group

企业版用户如果启用了 UID / GID 自动映射功能，那么在宿主机和容器中同时挂载时，由于两者的 `/etc/passwd` 和 `/etc/group` 往往不一致，容易碰到 [UID / GID 不一致](https://juicefs.com/docs/zh/cloud/guide/guid_auto_map/#uid--gid-%E4%B8%8D%E4%B8%80%E8%87%B4) 的情况。

此时需要为 CSI 的 Mount Pod 使用与宿主机相同的定制 `/etc/passwd` 和 `/etc/group` 来保证 UID / GID 一致。

## 根据宿主机配置创建 Secret

以下命令会读取宿主机的 `/etc/passwd` 和 `/etc/group` 来生成 Mount Pod 使用的 Kubernetes Secret。

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

## 配置 Mount Pod

默认的，我们 Mount Pod 已经将 `/etc/passwd` 和 `/etc/group` 变为指向 `~/.acl/passwd` 和 `~/.acl/group` 的软链接。

```bash
$ ls -l /etc/ | grep acl
lrwxrwxrwx 1 root root      16 Aug 27 04:49 group -> /root/.acl/group
lrwxrwxrwx 1 root root      17 Aug 27 04:49 passwd -> /root/.acl/passwd
```

我们只需要将 Secret 挂载到 `/root/.acl` 即可，参考[如何额外添加文件](../guide/pv.md#mount-pod-extra-files)增加对应字段 `configs: "{juicefs-uid-gid: /root/.acl}"` 即可。
