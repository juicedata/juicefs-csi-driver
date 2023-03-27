---
title: FAQ
slug: /faq
---

## My question is not answered in the documentations

Try searching for your problems in the top right corner, using different keywords. If nothing comes up:

* For JuiceFS Community Edition users, join [the JuiceFS Community](https://juicefs.com/en/community) and seek for help.
* For JuiceFS Cloud Service users, reach the Juicedata team using Intercom by clicking the bottom right button in [the JuiceFS Web Console](https://juicefs.com/console).

## How to seamlessly remount JuiceFS file system? {#seamless-remount}

JuiceFS file system needs to be remounted for some configuration changes to take effect. If you can accept downtime, simply delete mount pod and JuiceFS is remounted when mount pod is re-created (note that if [automatic mount point recovery](./guide/pv.md#automatic-mount-point-recovery) isn't enabled, you'll need to restart or re-create application pods to bring mount point back into service). But in Kubernetes, we often wish a seamless remount. You can achieve a seamless remount by the following process:

* When [upgrading or downgrading CSI Driver](./administration/upgrade-csi-driver.md), if mount pod image is changed along the way, CSI Driver will create new mount pod when you perform a rolling upgrade on application pods.
* Modify [mount options](./guide/pv.md#mount-options) at PV level, and perform a rolling upgrade on application pods. Note that for dynamic provisioning, although you can modify mount options in [StorageClass](./guide/pv.md#create-storage-class), but the changes made will not be reflected on existing PVs, a rolling upgrade thereafter will not trigger mount pod re-creation.
* Modify [volume credentials](./guide/pv.md#volume-credentials), and perform a rolling upgrade on application pods.
* If no configuration has been modified, but a seamless remount is still in need, you can make some trivial, ineffective changes to mount options (e.g. increase `cache-size` by 1), and then perform a rolling upgrade on application pods.

To learn about the CSI Driver implementation, and find out when will new mount pods are created to achieve seamless remount, see the `GenHashOfSetting()` function in [`pkg/juicefs/mount/pod_mount.go`](https://github.com/juicedata/juicefs-csi-driver/blob/master/pkg/juicefs/mount/pod_mount.go).
