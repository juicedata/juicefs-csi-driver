---
title: Upgrade JuiceFS Client
slug: /upgrade-juicefs-client
sidebar_position: 3
---

Upgrade JuiceFS Client to the latest version to enjoy all kinds of improvements and fixes, read [release notes for JuiceFS Community Edition](https://github.com/juicedata/juicefs/releases) or [release notes for JuiceFS Cloud Service](https://juicefs.com/docs/cloud/release) to learn more.

As a matter of fact, [upgrading JuiceFS CSI Driver](./upgrade-csi-driver.md) will bring upgrade to JuiceFS Client along the way, because every release includes the current latest [mount pod image](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v), but if you'd like to use the latest JuiceFS Client before CSI Driver release, or even before mount pod image release, refer to methods introduced in this chapter.

## Upgrade container image for mount pod {#upgrade-mount-pod-image}

Find the latest mount pod image in [Docker Hub](https://hub.docker.com/r/juicedata/mount/tags?page=1&name=v), and then [overwrite mount pod image](../guide/custom-image.md#overwrite-mount-pod-image).

Pay attention that, with mount pod image overwritten, [upgrading CSI Driver](./upgrade-csi-driver.md) will no longer affect mount pod image.

## Smooth upgrades for Mount Pods {#smooth-upgrade}
Starting from JuiceFS CSI Driver v0.25.0, smooth upgrades for Mount Pods are supported. This allows Mount Pods to be upgraded without interrupting the service.

:::tip Prerequisites

* Smooth upgrades are only applicable to Mount Pod mode.
* The Mount Pod image version must be v1.2.1 or later for the Community Edition, and v5.1.0 or later for the Enterprise Edition.

:::

:::warning Requirements for smooth upgrades
To perform a smooth upgrade, `preStop` of the Mount Pod should not be configured with `umount ${MOUNT_POINT}`. Ensure that `umount` is not configured in [CSI ConfigMap](./../guide/configurations.md#configmap).
:::

Currently, smooth upgrades can only be triggered in CSI Dashboard or the JuiceFS kubectl plugin.

### Trigger a smooth upgrade in Dashboard {#dashboard}

1. In CSI Dashboard, click **Configuration** and update the image version for the Mount Pod that needs to be upgraded.

    ![Upgrade the image](../images/upgrade-image.png)

2. In the Mount Pod details page, there are two upgrade buttons, **Pod Upgrade** and **Binary Upgrade**:
    
    - **Pod Upgrade** rebuilds the Mount Pod, requiring v1.2.1 or later for the Community Edition, and v5.1.0 or later for the Enterprise Edition.
    - **Binary Upgrad** only updates the binary without rebuilding the Mount Pod, requiring v1.2.0 or later for the Community Edition, and v5.0.0 or later for the Enterprise Edition.
    
    Both upgrades are smooth upgrades, allowing services to continue running without interruption.

    ![Upgrade menu](../images/upgrade-menu.png)

3. Click **upgrade** to trigger a smooth upgrade for the Mount Pod.

    ![Smooth upgrade](../images/smooth-upgrade.png)

### Trigger a smooth upgrade in the kubectl plugin {#kubectl-plugin}

Ensure that your JuiceFS kubectl plugin version is v0.3.0 or later.

1. Update the image version for Mount Pod in CSI ConfigMap configuration using kubectl.

    ```yaml
    apiVersion: v1
    data:
       config.yaml: |
          mountPodPatch:
             - ceMountImage: juicedata/mount:ce-v1.2.0
               eeMountImage: juicedata/mount:ee-5.1.1-ca439c2
    kind: ConfigMap
    ```
  
2. Trigger a smooth upgrade for the Mount Pod using the JuiceFS kubectl plugin.

    ```bash
    # Upgrade the Pod with recreation
    kubectl jfs upgrade juicefs-kube-node-1-pvc-52382ebb-f22a-4b7d-a2c6-1aa5ac3b26af-ebngyg --recreate
    # Upgrade binary
    kubectl jfs upgrade juicefs-kube-node-1-pvc-52382ebb-f22a-4b7d-a2c6-1aa5ac3b26af-ebngyg
    ```

## Upgrade JuiceFS Client temporarily

:::tip
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
