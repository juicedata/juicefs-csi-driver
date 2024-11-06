---
title: FAQ
slug: /faq
---

## My question is not answered in the documentations

Try searching for your problems in the top right corner, using different keywords. If nothing comes up:

* Use [troubleshooting tools](./administration/troubleshooting.md#tools) to self-troubleshoot problems;
* For JuiceFS Community Edition users, join [the JuiceFS Community](https://juicefs.com/en/community) and seek for help;
* For JuiceFS Cloud Service users, reach the Juicedata team using Intercom by clicking the bottom right button in [the JuiceFS Web Console](https://juicefs.com/console).

## How to seamlessly remount JuiceFS file system? {#seamless-remount}

If you can accept downtime, simply delete the Mount Pod and JuiceFS is remounted when the Mount Pod is re-created (note that if [automatic mount point recovery](./guide/configurations.md#automatic-mount-point-recovery) isn't enabled, you'll need to restart or re-create application Pods to bring mount point back into service). But in Kubernetes, we often wish a seamless remount. You can achieve a seamless remount by the following process:

* When [upgrading or downgrading CSI Driver](./administration/upgrade-csi-driver.md), if the Mount Pod image is changed along the way, CSI Driver will create a new Mount Pod when you perform a rolling upgrade on application pods.
* Modify [mount options](./guide/configurations.md#mount-options) at PV level, and perform a rolling upgrade on application pods. Note that for dynamic provisioning, although you can modify mount options in [StorageClass](./guide/pv.md#create-storage-class), but the changes made will not be reflected on existing PVs, a rolling upgrade thereafter will not trigger Mount Pod re-creation.
* Modify [volume credentials](./guide/pv.md#volume-credentials), and perform a rolling upgrade on application pods.
* If no configuration has been modified, but a seamless remount is still in need, you can make some trivial, ineffective changes to mount options (e.g. increase `cache-size` by 1), and then perform a rolling upgrade on application pods.

To learn about the CSI Driver implementation and find out when new Mount Pods will be created to achieve seamless remount, see the `GenHashOfSetting()` function in [`pkg/juicefs/mount/pod_mount.go`](https://github.com/juicedata/juicefs-csi-driver/blob/master/pkg/juicefs/mount/pod_mount.go).

## `exec format error` {#format-error}

The most common cause is CPU architecture misalignment, like running ARM64 images on a x86 system. If you must transfer images using a personal computer of different architecture, remember to specify the `platform`:

```shell
# Use the architecture of the actual environment
docker pull --platform=linux/amd64 juicedata/mount:ee-5.0.17-0c63dc5
```

Apart from architecture misalignment, we've also seen this type of errors caused by bad container runtime, in which case `exec format error` can occur even if images are of the same architecture. When this happens, you might need to purge and reinstall container runtime, take containerd as an example:

```shell
systemctl stop kubelet
rm -rf /var/lib/containerd
# reinstall containerd
```
