---
sidebar_label: 介绍
---

# JuiceFS CSI 驱动

[JuiceFS CSI 驱动](https://github.com/juicedata/juicefs-csi-driver)遵循 [CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md) 规范，实现了容器编排系统与 JuiceFS 文件系统之间的接口，支持动态配置 JuiceFS 卷提供给 Pod 使用。

## 版本要求

- Kubernetes 1.14+

## 安装

以下提供了两种安装 JuiceFS CSI 驱动的方式。

### 方法一：通过 Helm 安装

#### 版本要求

- Helm 3.1.0+

#### 安装 Helm

Helm 是 Kubernetes 的包管理器，Chart 是 Helm 管理的包。你可以把它看作是 Homebrew formula，APT dpkg，或 YUM RPM 在 Kubernetes 中的等价物。

请参照 [Helm 文档](https://helm.sh/docs/intro/install) 进行安装，并确保 `helm` 二进制能在 `PATH` 环境变量中找到。

#### 安装 JuiceFS CSI 驱动

1. 准备配置文件

   创建一个配置文件，例如：`values.yaml`，复制并完善下列配置信息。其中，`backend` 部分是 JuiceFS 文件系统相关的信息，你可以参照[「JuiceFS 快速上手指南」](https://juicefs.com/docs/zh/community/quick_start_guide)了解相关内容。如果使用的是已经提前创建好的 JuiceFS 卷，则只需填写 `name` 和 `metaurl` 这两项即可。`mountPod` 部分可以对使用此驱动的 Pod 设置 CPU 和内存的资源配置。不需要的项可以删除，或者将它的值留空。

   ```yaml title="values.yaml"
   storageClasses:
   - name: juicefs-sc
     enabled: true
     reclaimPolicy: Retain
     backend:
       name: "<name>"
       metaurl: "<meta-url>"
       storage: "<storage-type>"
       accessKey: "<access-key>"
       secretKey: "<secret-key>"
       bucket: "<bucket>"
     mountPod:
       resources:
         limits:
           cpu: "<cpu-limit>"
           memory: "<memory-limit>"
         requests:
           cpu: "<cpu-request>"
           memory: "<memory-request>"
   ```

2. 检查 kubelet 根目录

   执行以下命令

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

   如果结果不为空，则代表 kubelet 的根目录（`--root-dir`）不是默认值（`/var/lib/kubelet`），需要在第一步准备的配置文件 `values.yaml` 中将 `kubeletDir` 设置为 kubelet 当前的根目录路径：

   ```yaml
   kubeletDir: <kubelet-dir>
   ```

3. 部署

   依次执行以下三条命令，通过 Helm 部署 JuiceFS CSI 驱动。

   ```sh
   helm repo add juicefs-csi-driver https://juicedata.github.io/charts/
   helm repo update
   helm install juicefs-csi-driver juicefs-csi-driver/juicefs-csi-driver -n kube-system -f ./values.yaml
   ```

4. 检查部署状态

   - **检查 Pods**：部署过程会启动一个名为 `juicefs-csi-controller` 的 `StatefulSet` 及一个 replica，以及一个名为 `juicefs-csi-node` 的 `DaemonSet`。执行命令 `kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver` 会看到有 `n+1` 个（`n` 指 Kubernetes 的 Node 数量）pod 在运行，例如：

     ```sh
     $ kubectl -n kube-system get pods -l app.kubernetes.io/name=juicefs-csi-driver
     NAME                       READY   STATUS    RESTARTS   AGE
     juicefs-csi-controller-0   3/3     Running   0          22m
     juicefs-csi-node-v9tzb     3/3     Running   0          14m
     ```

   - **检查 Secret**：通过命令 `kubectl -n kube-system describe secret juicefs-sc-secret` 可以看到前面 `values.yaml` 配置文件中 `backend` 部分的 secret 信息。

     ```
     Name:         juicefs-sc-secret
     Namespace:    kube-system
     Labels:       app.kubernetes.io/instance=juicefs-csi-driver
                   app.kubernetes.io/managed-by=Helm
                   app.kubernetes.io/name=juicefs-csi-driver
                   app.kubernetes.io/version=0.7.0
                   helm.sh/chart=juicefs-csi-driver-0.1.0
     Annotations:  meta.helm.sh/release-name: juicefs-csi-driver
                   meta.helm.sh/release-namespace: default

     Type:  Opaque

     Data
     ====
     access-key:  0 bytes
     bucket:      47 bytes
     metaurl:     54 bytes
     name:        4 bytes
     secret-key:  0 bytes
     storage:     2 bytes
     ```

   - **检查存储类（StorageClass）**：通过命令 `kubectl get sc juicefs-sc` 会看到类似下面的存储类信息。

     ```
     NAME         PROVISIONER       RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
     juicefs-sc   csi.juicefs.com   Retain          Immediate           false                  69m
     ```

### 方法二：通过 kubectl 安装

由于 Kubernetes 在版本变更过程中会废弃部分旧的 API，因此需要根据你使用 Kubernetes 版本选择适用的部署文件。

1. 检查 kubelet 根目录

   在 Kubernetes 集群中任意一个非 Master 节点上执行以下命令：

   ```shell
   ps -ef | grep kubelet | grep root-dir
   ```

2. 部署

   **如果上一步检查命令返回的结果不为空**，则代表 kubelet 的根目录（`--root-dir`）不是默认值（`/var/lib/kubelet`），因此需要在 CSI 驱动的部署文件中更新 `kubeletDir` 路径并部署：

   ```shell
   # Kubernetes 版本 >= v1.18
   curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -

   # Kubernetes 版本 < v1.18
   curl -sSL https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml | sed 's@/var/lib/kubelet@{{KUBELET_DIR}}@g' | kubectl apply -f -
   ```

   :::note 注意
   请将上述命令中 `{{KUBELET_DIR}}` 替换成 kubelet 当前的根目录路径。
   :::

   **如果前面检查命令返回的结果为空**，无需修改配置，可直接部署：

   ```shell
   # Kubernetes 版本 >= v1.18
   kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s.yaml

   # Kubernetes 版本 < v1.18
   kubectl apply -f https://raw.githubusercontent.com/juicedata/juicefs-csi-driver/master/deploy/k8s_before_v1_18.yaml
   ```

## 故障排查

请参考 [故障排查](troubleshooting.md) 或 [FAQ](faq) 文档。

## 升级 CSI 驱动

请参考 [升级 JuiceFS CSI 驱动](upgrade-csi-driver.md) 文档

## 示例

开始之前，你需要：

* 了解如何设置 Kubernetes 和 JuiceFS
* 确保 JuiceFS 能够被 Kuberenetes 集群访问。建议在与 Kubernetes 集群相同的区域创建文件系统。
* 参照[说明](#安装)安装 JuiceFS CSI 驱动。

### 目录

* [静态配置](examples/static-provisioning.md)
* [动态配置](examples/dynamic-provisioning.md)
* [配置文件系统设置](examples/format-options.md)
* [设置挂载选项](examples/mount-options.md)
* [设置缓存路径](examples/cache-dir.md)
* [挂载子目录](examples/subpath.md)
* [数据加密](examples/encrypt.md)
* [使用 ReadWriteMany 和 ReadOnlyMany](examples/rwx-and-rox.md)
* [配置 Mount Pod 的资源限制](examples/mount-resources.md)
* [在 Mount Pod 中设置配置文件和环境变量](examples/config-and-env.md)
* [延迟删除 Mount Pod](examples/delay-delete.md)

:::info 说明
由于 JuiceFS 是一个弹性文件系统，它不需要强制分配容量。你在 `PersistentVolume` 和 `PersistentVolumeClaim` 中指定的容量并不是实际存储容量。但是，由于存储容量是 Kubernetes 的必填字段，因此您可以使用任何有效值，例如 `10Pi` 表示容量。
:::

## 已知问题

- JuiceFS CSI 驱动 v0.10.0 及以上版本不支持在 `--cache-dir` 挂载选项中使用通配符
