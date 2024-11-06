---
title: Upgrade JuiceFS Client
slug: /upgrade-juicefs-client
sidebar_position: 3
---

Upgrade JuiceFS Client to the latest version to enjoy all kinds of improvements and fixes, read [release notes for JuiceFS Community Edition](https://github.com/juicedata/juicefs/releases) or [release notes for JuiceFS Cloud Service](https://juicefs.com/docs/cloud/release) to learn more.

In fact, upgrading [JuiceFS CSI Driver](./upgrade-csi-driver.md) will also upgrade the JuiceFS client, as each release includes the latest [Mount Pod image](../guide/custom-image.md#ce-ee-separation). However, if you'd like to use the latest JuiceFS client ahead of the next CSI Driver release, or even before the Mount Pod image is updated, refer to the methods introduced in this chapter.

## Upgrade container image for Mount Pod {#upgrade-mount-pod-image}

Currently, there are two methods for upgrading Mount Pod container images:

- [Smooth upgrade of Mount Pods](#smooth-upgrade): This method allows you to upgrade an already created Mount Pod without rebuilding the application pod.
- [Lossy upgrade of Mount Pods](../guide/custom-image.md#overwrite-mount-pod-image): This method requires rebuilding the application Pod to upgrade an existing Mount Pod.

Refer to [this document](../guide/custom-image.md#ce-ee-separation) to find the tag for the latest Mount Pod container image in Docker Hub. Then, choose the appropriate upgrade method based on your CSI Driver version and the mount mode you are using:

|                    | Version 0.25.0 and above | Version before 0.25.0   |
|:------------------:|:------------------------:|:-----------------------:|
| **Mount Pod mode** | Smooth upgrade of Mount Pods | Lossy upgrade of Mount Pods |
| **Sidecar mode**   | Lossy upgrade of Mount Pods  | Lossy upgrade of Mount Pods |

**Note:** After overwriting the Mount Pod image, further CSI Driver upgrades will no longer automatically update the Mount Pod image.

### Smooth upgrade of Mount Pods <VersionAdd>0.25.0</VersionAdd> {#smooth-upgrade}

Starting from CSI Driver v0.25.0, smooth upgrade of Mount Pods is supported (note that this does not apply to Sidecar & Mount by process mode). This feature enables Mount Pods to upgrade seamlessly without disrupting services, leveraging the JuiceFS Client's zero-downtime restart capability. For more information, see our documentation for the [Community Edition](https://juicefs.com/docs/community/administration/upgrade) and [Enterprise Edition](https://juicefs.com/docs/cloud/getting_started#upgrade-juicefs). This version comes with another merit that allows smooth restart and recovery of Mount Pods. Learn more in the [automatic recovery](../guide/configurations.md#automatic-mount-point-recovery) section.

:::warning Requirements for smooth upgrades
To perform a smooth upgrade, `preStop` of the Mount Pod should not be configured with `umount ${MOUNT_POINT}`. Ensure that `umount` is not configured in [CSI ConfigMap](./../guide/configurations.md#configmap).
:::

Smooth upgrade of Mount Pods has two upgrade methods: "Pod recreate upgrade" and "Binary upgrade." The difference is as follows:

- Pod recreate upgrade: The Mount Pod will be rebuilt. The minimum version requirement for the Mount Pod is 1.2.1 (Community Edition) or 5.1.0 (Enterprise Edition).
- Binary upgrade: The Mount Pod is not rebuilt; only the binary is upgraded. Other configurations remain unchanged. After the upgrade, the YAML for the Mount Pod will still display the original image. The minimum version requirement for this upgrade is 1.2.0 (Community Edition) or 5.0.0 (Enterprise Edition).

Both upgrade methods are smooth upgrades, allowing services to continue without interruption. Choose the method according to your situation.

Smooth upgrade can be triggered in the [CSI dashboard](./troubleshooting.md#csi-dashboard) or the [JuiceFS kubectl plugin](./troubleshooting.md#kubectl-plugin).

#### Trigger a smooth upgrade in CSI dashboard {#smooth-upgrade-via-csi-dashboard}

1. In the CSI dashboard, click the **Configuration** button in the upper right corner to update and save the new image version for the Mount Pod that needs to be upgraded:

   :::tip
   Compared to manually modifying [CSI ConfigMap configuration](./../guide/configurations.md#configmap), modifications on the CSI dashboard take effect immediately.
   :::

   ![CSI dashboard config Mount Pod image](../images/upgrade-image.png)

2. In the Mount Pod details page, there are two upgrade buttons. One is for a pod recreate upgrade and the other one is for a binary upgrade:

   ![CSI dashboard Mount Pod upgrade button](../images/upgrade-menu.png)

3. Click the **upgrade** button to trigger a smooth upgrade for the Mount Pod:

   ![CSI dashboard Mount Pod smooth upgrade](../images/smooth-upgrade.png)

#### Trigger a smooth upgrade in the kubectl plugin {#smooth-upgrade-via-kubectl-plugin}

:::tip
The minimum version requirement for the JuiceFS kubectl plugin is 0.3.0.
:::

1. Update the image version for the Mount Pod in [CSI ConfigMap configuration](./../guide/configurations.md#configmap) using kubectl:

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    data:
       config.yaml: |
          mountPodPatch:
             - ceMountImage: juicedata/mount:ce-v1.2.0
               eeMountImage: juicedata/mount:ee-5.1.1-ca439c2
    ```

2. Trigger a smooth upgrade for the Mount Pod using the JuiceFS kubectl plugin:

    ```bash
    # Pod recreate upgrade
    kubectl jfs upgrade juicefs-kube-node-1-pvc-52382ebb-f22a-4b7d-a2c6-1aa5ac3b26af-ebngyg --recreate

    # Binary upgrade
    kubectl jfs upgrade juicefs-kube-node-1-pvc-52382ebb-f22a-4b7d-a2c6-1aa5ac3b26af-ebngyg
    ```

## Upgrade JuiceFS client temporarily (not recommended)

:::warning
You are strongly encouraged to upgrade JuiceFS CSI Driver to v0.10 and later versions, the method demonstrated below are not recommended for production use.
:::

If you're using [Mount by process mode](../introduction.md#by-process), or using CSI Driver prior to v0.10.0, and cannot easily upgrade to v0.10, you can choose to upgrade JuiceFS Client independently, inside the CSI Node Service pod.

This is only a temporary solution, if CSI Node Service pods are recreated, or new nodes are added to Kubernetes cluster, you'll need to run this script again.

1. Use this script to replace the `juicefs` binary in `juicefs-csi-node` pod with the new built one:

   ```bash
   #!/bin/bash

   KUBECTL=/path/to/kubectl
   JUICEFS_BIN=/path/to/new/juicefs

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system cp $JUICEFS_BIN '{}':/tmp/juicefs -c juicefs-plugin

   $KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
       xargs -L 1 -P 10 -I'{}' \
       $KUBECTL -n kube-system exec -i '{}' -c juicefs-plugin -- \
       chmod a+x /tmp/juicefs && mv /tmp/juicefs /bin/juicefs
   ```

   :::note
   Replace `/path/to/kubectl` and `/path/to/new/juicefs` in the script with the actual values, then execute the script.
   :::

2. Restart the applications one by one, or kill the existing pods.
