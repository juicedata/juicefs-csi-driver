---
slug: /csi-in-serverless
sidebar_label: 在 Serverless 中使用
---

# 在 Serverless 环境使用 CSI 驱动

:::note 注意
此特性需使用 0.23.1 及以上版本的 JuiceFS CSI 驱动
:::

不同的云厂商的 Serverless 环境实现不尽相同，本篇文档会详细描述在不同的云厂商的 Serverless 环境中如何使用 JuiceFS CSI 驱动，包括[华为云 CCI](https://www.huaweicloud.com/product/cci.html)、[火山引擎 VCI](https://www.volcengine.com/theme/1224494-D-7-1)、[阿里云 ACS](https://www.aliyun.com/product/acs) 以及[腾讯云 Serverless 容器服务](https://cloud.tencent.com/product/tkeserverless)，[阿里云 ECI](https://www.aliyun.com/product/eci) 还不支持使用 JuiceFS CSI 驱动，需要通过 Fluid，请参考文档 [《以 Serverless Container 的方式在 ACK 使用 JuiceFS》](https://juicefs.com/docs/zh/cloud/kubernetes/use_in_eci)。

## 安装 {#install}

参考[安装](../getting_started.md#sidecar)小节（使用 `mountMode: serverless` 模式），唯一需要注意的是，安装完毕后，给需要用到使用 JuiceFS CSI 驱动的命名空间打上下述标签：

```shell
kubectl label namespace $NS juicefs.com/enable-serverless-injection=true --overwrite
```

## 华为云 CCI {#cci}

目前只能通过在 CCE 集群中对接 CCI 的方式使用，参考文档 [CCE 突发弹性引擎](https://support.huaweicloud.com/usermanual-cce/cce_10_0135.html)。环境配置好后，在应用 Pod 中加入以下 Label 即可在 CCI 环境中使用 JuiceFS PV：

```yaml {6}
apiVersion: v1
kind: Pod
metadata:
  name: mypod
  labels:
    virtual-kubelet.io/burst-to-cci: "enforce"
spec:
  volumes:
    - name: myjfs
      persistentVolumeClaim:
        claimName: myjfs
  containers:
    - name: myapp
      volumeMounts:
        - mountPath: /app
          name: myjfs
      ...
```

## 火山引擎 VCI {#vci}

火山引擎 VCI 的使用配置可以参考文档 [VCI 入门指引](https://www.volcengine.com/docs/6460/110394)。在应用 Pod 中加入以下 Annotation 即可在 VCI 环境中使用 JuiceFS PV：

```yaml {6}
apiVersion: v1
kind: Pod
metadata:
  name: mypod
  annotations:
    vke.volcengine.com/burst-to-vci: "enforce"
spec:
  volumes:
    - name: myjfs
      persistentVolumeClaim:
        claimName: myjfs
  containers:
    - name: myapp
      volumeMounts:
        - mountPath: /app
          name: myjfs
      ...
```

## 阿里云 ACS {#acs}

(ACS 特权模式计划进行产品调整，目前不推荐使用此模式，文档仅供正在使用此方式的用户参考) ACS 的使用方式可以参考文档 [ACS 文档](https://help.aliyun.com/zh/cs/use-container-computing-for-the-first-time?spm=5176.28566299.J_JeMPinYpaANjKhI-pJViv.2.159f4fcbBJjme3)。环境搭建好后，无需给 Pod 加任何 Label 或 Annotation，可直接使用：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  volumes:
    - name: myjfs
      persistentVolumeClaim:
        claimName: myjfs
  containers:
    - name: myapp
      volumeMounts:
        - mountPath: /app
          name: myjfs
      ...
```

## 腾讯云 Serverless 集群 {#eks}

腾讯云 Serverless 集群的使用方式可以参考文档 [TKE Serverless 集群文档](https://cloud.tencent.com/document/product/457/39813)。环境搭建好后，无需给 Pod 加任何 Label 或 Annotation，可直接使用：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: mypod
spec:
  volumes:
    - name: myjfs
      persistentVolumeClaim:
        claimName: myjfs
  containers:
    - name: myapp
      volumeMounts:
        - mountPath: /app
          name: myjfs
      ...
```
