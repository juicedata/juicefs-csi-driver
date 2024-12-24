---
title: 运行其他 JuiceFS 应用
sidebar_position: 7
---

严格来说，本章内容与 JuiceFS CSI 驱动没有关联，而是通用的 Kubernetes 应用，因此下方介绍的各种部署方式，也完全可以脱离 CSI 驱动、独立使用。比方说：

* 用 `juicefs sync` 在 Kubernetes 中定时同步数据
* 在 Kubernetes 中运行 `juicefs webdav` 或者 `juicefs gateway`（S3 网关）
* &#8203;<Badge type="primary">私有部署</Badge> 在 Kubernetes 中部署分布式缓存组

JuiceFS 客户端还有诸多强大的其他功能，我们无法囊括所有的用例，因此如果你有更多需求希望能获得部署示范，也应当可以参考下方的案例，自行开发属于你的场景的部署配置。如果本章内容帮助你撰写了新的场景的部署配置，欢迎给[文档](https://github.com/juicedata/juicefs-csi-driver/tree/master/docs/zh_cn/guide)贡献内容，将你的示范补充到这里。

## 用 `juicefs sync` 定时同步数据 {#juicefs-sync}

`juicefs sync` 是非常好用的数据迁移方案，你可以用 CronJob 搭建定时同步机制。

下方示范以企业版为例，并且假定了用户需要与 JuiceFS 文件系统进行交互，因此需要在容器中运行 `auth` 命令，如果不需要的话，可以对多余配置进行裁剪。

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  # 名称、命名空间可自定义
  name: juicefs-sync
  namespace: default
spec:
  # 根据实际需求更改
  schedule: "5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: Never
          containers:
          - name: juicefs-sync
            command:
            - sh
            - -c
            - |
              # 下面的 shell 代码仅在私有部署需要，作用是将 envs 这一 JSON 进行展开，将键值设置为环境变量
              for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
                echo "export $keyval"
                eval export $keyval
              done

              # 如果 sync 命令需要直接读取或写入 JuiceFS 文件系统，推荐使用 jfs:// 协议头
              # 此方式需要提前认证、获取配置文件
              /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

              # 具体的 sync 命令需要根据实际需求更改
              # 参考文档：https://juicefs.com/docs/zh/cloud/guide/sync/
              /usr/bin/juicefs sync oss://${ACCESS_KEY}:${SECRET_KEY}@myjfs-bucket.oss-cn-hongkong.aliyuncs.com/chaos-ee-test/juicefs_uuid jfs://$VOL_NAME
            env:
            # 存放文件系统认证信息的 Secret，必须在同一个命名空间下
            # 参考文档：https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
            - name: VOL_NAME
              valueFrom:
                secretKeyRef:
                  key: name
                  name: juicefs-secret
            - name: ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  key: access-key
                  name: juicefs-secret
            - name: SECRET_KEY
              valueFrom:
                secretKeyRef:
                  key: secret-key
                  name: juicefs-secret
            - name: TOKEN
              valueFrom:
                secretKeyRef:
                  key: token
                  name: juicefs-secret
            # 仅在私有部署需要
            - name: ENVS
              valueFrom:
                secretKeyRef:
                  key: envs
                  name: juicefs-secret
            # 使用 Mount Pod 的容器镜像
            # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
            image: juicedata/mount:ee-5.0.14-a38b96d
            # 按照实际情况调整资源请求和约束
            # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
            resources:
              requests:
                memory: 500Mi
```

## 部署分布式缓存集群 {#distributed-cache-cluster}

:::tip
对于大多数场景，通过[「缓存组 Operator」](./cache-group-operator.md)来部署及管理分布式缓存集群更为方便，推荐优先使用这种方式。
:::

为了在 Kubernetes 集群部署一个稳定的缓存集群，可以参考以下示范，在集群内指定的节点挂载 JuiceFS 客户端，形成一个稳定的缓存组。

下方介绍两种部署方式，分别是 StatefulSet 和 DaemonSet。功能上并无区别，但是在升级或修改配置的时候，StatefulSet 默认会按照从低到高位依次重启，这种方式对缓存组消费端的服务冲击更小。而 DaemonSet 方式则会根据其 `updateStrategy` 设置来执行更新，如果规模巨大，需要仔细设置更新策略，避免冲击服务。

除此之外，两种部署方式没有显著不同，根据喜好选择即可。如果需要更加灵活和精准的中心化配置调节，比如覆盖某个节点的权重设置等，可以引入额外的 ConfigMap 和启动脚本来实现。

### DaemonSet 方式

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  # 名称、命名空间可自定义
  name: juicefs-cache-group
  namespace: default
spec:
  selector:
    matchLabels:
      app: juicefs-cache-group
      juicefs-role: cache
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  template:
    metadata:
      labels:
        app: juicefs-cache-group
        juicefs-role: cache
    spec:
      # 使用 hostNetwork，让 Pod 以固定 IP 运行，避免容器重建更换 IP，导致缓存数据失效
      hostNetwork: true
      containers:
      - name: juicefs-cache
        command:
        - sh
        - -c
        - |
          # 下面的 shell 代码仅在私有部署需要，作用是将 envs 这一 JSON 进行展开，将键值设置为环境变量
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # 认证和挂载，所有环境变量均引用包含着文件系统认证信息的 Kubernetes Secret
          # 参考文档：https://juicefs.com/docs/zh/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # 由于在容器中常驻，必须用 --foreground 模式运行，其它挂载选项（特别是 --cache-group）按照实际情况调整
          # 参考文档：https://juicefs.com/docs/zh/cloud/reference/commands_reference#mount
          /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --foreground --cache-dir=/data/jfsCache --cache-size=512000 --cache-group=jfscache
        env:
        # 存放文件系统认证信息的 Secret，必须和该 StatefulSet 在同一个命名空间下
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
        - name: VOL_NAME
          valueFrom:
            secretKeyRef:
              key: name
              name: juicefs-secret
        - name: ACCESS_KEY
          valueFrom:
            secretKeyRef:
              key: access-key
              name: juicefs-secret
        - name: SECRET_KEY
          valueFrom:
            secretKeyRef:
              key: secret-key
              name: juicefs-secret
        - name: TOKEN
          valueFrom:
            secretKeyRef:
              key: token
              name: juicefs-secret
        # 仅在私有部署需要
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        # 使用 Mount Pod 的容器镜像
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.2-69f82b3
        lifecycle:
          # 容器退出时卸载文件系统
          preStop:
            exec:
              command:
              - sh
              - -c
              - umount /mnt/jfs
        # 按照实际情况调整资源请求和约束
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        # 挂载文件系统必须启用的权限
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # 调整缓存目录的路径，如有多个缓存目录需要定义多个 volume
      # 参考文档：https://juicefs.com/docs/zh/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

### StatefulSet 方式

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  # 名称、命名空间可自定义
  name: juicefs-cache-group
  namespace: default
spec:
  # 缓存组客户端数量
  replicas: 1
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: juicefs-cache-group
      juicefs-role: cache
  serviceName: juicefs-cache-group
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: juicefs-cache-group
        juicefs-role: cache
    spec:
      # 一个 Kubernetes 节点上只运行一个缓存组客户端
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: juicefs-role
                operator: In
                values:
                - cache
            topologyKey: kubernetes.io/hostname
      # 使用 hostNetwork，让 Pod 以固定 IP 运行，避免容器重建更换 IP，导致缓存数据失效
      hostNetwork: true
      containers:
      - name: juicefs-cache
        command:
        - sh
        - -c
        - |
          # 下面的 shell 代码仅在私有部署需要，作用是将 envs 这一 JSON 进行展开，将键值设置为环境变量
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # 认证和挂载，所有环境变量均引用包含着文件系统认证信息的 Kubernetes Secret
          # 参考文档：https://juicefs.com/docs/zh/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # 由于在容器中常驻，必须用 --foreground 模式运行，其它挂载选项（特别是 --cache-group）按照实际情况调整
          # 参考文档：https://juicefs.com/docs/zh/cloud/reference/commands_reference#mount
          /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --foreground --cache-dir=/data/jfsCache --cache-size=512000 --cache-group=jfscache
        env:
        # 存放文件系统认证信息的 Secret，必须和该 StatefulSet 在同一个命名空间下
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
        - name: VOL_NAME
          valueFrom:
            secretKeyRef:
              key: name
              name: juicefs-secret
        - name: ACCESS_KEY
          valueFrom:
            secretKeyRef:
              key: access-key
              name: juicefs-secret
        - name: SECRET_KEY
          valueFrom:
            secretKeyRef:
              key: secret-key
              name: juicefs-secret
        - name: TOKEN
          valueFrom:
            secretKeyRef:
              key: token
              name: juicefs-secret
        # 仅在私有部署需要
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        # 使用 Mount Pod 的容器镜像
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.2-69f82b3
        lifecycle:
          # 容器退出时卸载文件系统
          preStop:
            exec:
              command:
              - sh
              - -c
              - umount /mnt/jfs
        # 按照实际情况调整资源请求和约束
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        # 挂载文件系统必须启用的权限
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # 调整缓存目录的路径，如有多个缓存目录需要定义多个 volume
      # 参考文档：https://juicefs.com/docs/zh/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

### StatefulSet 方式（为各节点定制不同配置） {#statefulset-customize-different-configurations-for-each-node}

此处仅是一个范例，如有更多自定义的需求可以自行定制。范例中将相关的配置和脚本均放置于 ConfigMap 中方便管理和调整。

```yaml title="jfs-cache-group-cm.yaml"
apiVersion: v1
kind: ConfigMap
metadata:
  # 名称和命名空间可自定义
  name: juicefs-cache-group-cm
  namespace: default
data:
  cache-group-config.json: |
    {
      "opts": {
        "cache-group": "jfscache",
        "cache-dir": "/jfsCache/disk*",
        "free-space-ratio": 0.01,
        "group-weight-unit": 10240
      },
      "nodes": {
        "default": {
          "size": "auto",
          "weight": "auto"
        },
        "node1": {
          "size": "1024000"
        },
        "node2": {
          "weight": "200"
        }
      }
    }
  run.py: |
    #!/usr/bin/env python3

    import json
    import os
    import subprocess

    # - auth

    # -- load env
    secret_dir = '/etc/jfs/secret'
    with open(os.path.join(secret_dir, 'envs'), 'r') as fd:
        envs = json.load(fd)
        for k in envs:
            os.environ[k] = envs[k]

    # -- load basic opt
    auth_opts = {}
    for opt in ['name', 'access-key', 'secret-key', 'token']:
        with open(os.path.join(secret_dir, opt), 'r') as fd:
            auth_opts[opt] = fd.read()
    fsname = auth_opts['name']
    token = auth_opts['token']
    access_key = auth_opts['access-key']
    secret_key = auth_opts['secret-key']

    # -- run auth
    subprocess.run(f'/usr/bin/juicefs auth --token={token} --access-key={access_key} --secret-key={secret_key} {fsname}', shell=True, check=True)

    # - cache-group mount

    # -- load cache group config
    with open('/etc/jfs/cache-group-config.json', 'r') as fd:
        cache_group_opts = json.load(fd)
    nodes = cache_group_opts['nodes']
    default = nodes['default']

    # -- load hostname
    node_name = os.environ['NODE_NAME']

    # -- load size
    large_size = 1_000_000_000 # 1PB
    override_size = None
    if node_name in nodes and 'size' in nodes[node_name]:
        override_size = nodes[node_name]['size']
    if default['size'] == 'auto':
        cache_size = large_size
    else:
        cache_size = int(default['size'])
    if override_size:
        if override_size == 'auto':
            cache_size = large_size
        else:
            cache_size = int(override_size)

    # -- load weight
    override_weight = None
    if node_name in nodes and 'weight' in nodes[node_name]:
        override_weight = nodes[node_name]['weight']
    cache_weight = "auto"
    if default['weight'] != 'auto':
        cache_weight = int(default['weight'])
    if override_weight and override_weight != 'auto':
        cache_weight = int(override_weight)

    # -- preprocess cache-dir
    cache_dir = cache_group_opts['opts']['cache-dir']
    if cache_dir:
        cache_dir_list = []
        import glob
        entities = glob.glob(cache_dir)
        for entity in entities:
            if os.path.ismount(entity):
                cache_dir_list.append(entity)

    # -- run mount
    cmd = f'/usr/bin/juicefs mount {fsname} /mnt/jfs --foreground'
    opts = cache_group_opts['opts']
    for opt in opts:
        if opt == 'cache-dir':
            # skip cache-dir, we have preprocessed it
            continue
        if cache_weight != 'auto' and opt == 'group-weight-unit':
            # group weight unit has higher priority, we need skip it
            # it's bug. fixed by new version
            continue
        cmd += f' --{opt}={opts[opt]}'
    cmd += f' --cache-size={cache_size}'
    if cache_weight != 'auto':
        cmd += f' --group-weight={cache_weight}'
    if cache_dir_list:
        cmd += f" --cache-dir={':'.join(cache_dir_list)}"
    else:
        # only use mounted dirs
        raise Exception("no mounted dir found")

    print(cmd)
    subprocess.run(cmd, shell=True, check=True)
```

```yaml title="jfs-cache-group-sts.yaml"
apiVersion: apps/v1
kind: StatefulSet
metadata:
  # 名称、命名空间可自定义
  name: juicefs-cache-group
  namespace: default
spec:
  # 缓存组客户端数量
  replicas: 1
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: juicefs-cache-group
      juicefs-role: cache
  serviceName: juicefs-cache-group
  updateStrategy:
    rollingUpdate:
      partition: 0
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: juicefs-cache-group
        juicefs-role: cache
    spec:
      # 一个 Kubernetes 节点上只运行一个缓存组客户端
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: juicefs-role
                operator: In
                values:
                - cache
            topologyKey: kubernetes.io/hostname
      # 使用 hostNetwork，让 Pod 以固定 IP 运行，避免容器重建更换 IP，导致缓存数据失效
      hostNetwork: true
      containers:
      - name: juicefs-cache
        command:
        - python3
        - -u
        - /usr/local/bin/run.py
        env:
        # 传入 Node Name 用于匹配配置中自定义的缓存大小和权重
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        # 使用 Mount Pod 的容器镜像
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.2-69f82b3
        lifecycle:
          # 容器退出时卸载文件系统
          preStop:
            exec:
              command:
              - sh
              - -c
              - umount /mnt/jfs
        # 按照实际情况调整资源请求和约束
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        # 挂载文件系统必须启用的权限
        securityContext:
          privileged: true
        volumeMounts:
        # 自定义启动脚本
        - mountPath: /usr/local/bin/run.py
          subPath: run.py
          name: juicefs-cache-group-cm
          readOnly: true
        # 自定义配置
        - mountPath: /etc/jfs/cache-group-config.json
          subPath: cache-group-config.json
          name: juicefs-cache-group-cm
          readOnly: true
        # 挂载所需的 secret 信息，挂载后由启动脚本直接读取
        - mountPath: /etc/jfs/secret
          name: juicefs-secret
          readOnly: true
        - mountPath: /jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # 以 /jfsCache 下多个 disk* 缓存目录举例
      - name: cache-dir
        hostPath:
          path: /jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

### 使用缓存组

上方示范便是在集群中启动了 JuiceFS 缓存集群，其缓存组名为 `jfscache`，那么为了让应用程序的 JuiceFS 客户端使用该缓存集群，需要让他们一并加入这个缓存组，并额外添加 `--no-sharing` 这个挂载参数，这样一来，应用程序的 JuiceFS 客户端虽然加入了缓存组，但却不参与缓存数据的构建，避免了客户端频繁创建、销毁所导致的缓存数据不稳定。

以动态配置为例，按照下方示范修改挂载参数即可，关于在 `mountOptions` 调整挂载配置，详见[「挂载参数」](../guide/configurations.md#mount-options)。

```yaml {13-14}
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
mountOptions:
  ...
  - cache-group=jfscache
  - no-sharing
```

## 运行 JuiceFS S3 网关 {#juicefs-gateway}

建议用我们的 [Helm Chart](https://github.com/juicedata/charts) 来部署 S3 网关。但出于示范目的，在这里也一并提供 Deployment 的运行示范。注意示范里并不包含 Service 和 Ingress，你需要自行创建他们来对外暴露服务。

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # 名称、命名空间可自定义
  name: juicefs-gateway
  namespace: default
spec:
  # 网关客户端数量
  replicas: 1
  selector:
    matchLabels:
      app: juicefs-gateway
      juicefs-role: gateway
  template:
    metadata:
      labels:
        app: juicefs-gateway
        juicefs-role: gateway
    spec:
      containers:
      - name: juicefs-gateway
        command:
        - sh
        - -c
        - |
          # 下面的 shell 代码仅在私有部署需要，作用是将 envs 这一 JSON 进行展开，将键值设置为环境变量
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # 认证和挂载，所有环境变量均引用包含着文件系统认证信息的 Kubernetes Secret
          # 参考文档：https://juicefs.com/docs/zh/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # 为了方便管理，将 MinIO 的认证信息直接设置为对象存储 AKSK
          export MINIO_ROOT_USER=${ACCESS_KEY}
          export MINIO_ROOT_PASSWORD=${SECRET_KEY}

          # 参考文档：https://juicefs.com/docs/zh/cloud/reference/commands_reference#gateway
          /usr/bin/juicefs gateway $VOL_NAME 0.0.0.0:9000 --cache-dir=/data/jfsCache
        env:
        # 存放文件系统认证信息的 Secret，必须和该 Deployment 在同一个命名空间下
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
        - name: VOL_NAME
          valueFrom:
            secretKeyRef:
              key: name
              name: juicefs-secret
        - name: ACCESS_KEY
          valueFrom:
            secretKeyRef:
              key: access-key
              name: juicefs-secret
        - name: SECRET_KEY
          valueFrom:
            secretKeyRef:
              key: secret-key
              name: juicefs-secret
        - name: TOKEN
          valueFrom:
            secretKeyRef:
              key: token
              name: juicefs-secret
        # 仅在私有部署需要，云服务环境需要删掉下方环境变量
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        # 使用 Mount Pod 的容器镜像
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
        ports:
          - containerPort: 9000
        image: juicedata/mount:ee-5.0.14-a38b96d
        # 按照实际情况调整资源请求和约束
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # 调整缓存目录的路径，如有多个缓存目录需要定义多个 volume
      # 参考文档：https://juicefs.com/docs/zh/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

## 运行 JuiceFS WebDAV {#juicefs-webdav}

对于企业版，建议用 Deployment 运行 WebDAV，的写法参考下方示范。你需要自行撰写 Service 和 Ingress 来对外暴露服务。

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # 名称、命名空间可自定义
  name: juicefs-webdav
  namespace: default
spec:
  # 缓存组客户端数量
  replicas: 1
  selector:
    matchLabels:
      app: juicefs-webdav
      juicefs-role: webdav
  template:
    metadata:
      labels:
        app: juicefs-webdav
        juicefs-role: webdav
    spec:
      containers:
      - name: juicefs-webdav
        command:
        - sh
        - -c
        - |
          # 下面的 shell 代码仅在私有部署需要，作用是将 envs 这一 JSON 进行展开，将键值设置为环境变量
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # 认证和挂载，所有环境变量均引用包含着文件系统认证信息的 Kubernetes Secret
          # 参考文档：https://juicefs.com/docs/zh/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # 设置用户名和密码
          export WEBDAV_USER=root
          export WEBDAV_PASSWORD=1234

          # 参考文档：https://juicefs.com/docs/zh/cloud/reference/commands_reference#webdav
          /usr/bin/juicefs webdav $VOL_NAME 0.0.0.0:9007 --cache-dir=/data/jfsCache
        env:
        # 存放文件系统认证信息的 Secret，必须和该 Deployment 在同一个命名空间下
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
        - name: VOL_NAME
          valueFrom:
            secretKeyRef:
              key: name
              name: juicefs-secret
        - name: ACCESS_KEY
          valueFrom:
            secretKeyRef:
              key: access-key
              name: juicefs-secret
        - name: SECRET_KEY
          valueFrom:
            secretKeyRef:
              key: secret-key
              name: juicefs-secret
        - name: TOKEN
          valueFrom:
            secretKeyRef:
              key: token
              name: juicefs-secret
        # 仅在私有部署需要
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        # 使用 Mount Pod 的容器镜像
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/custom-image
        ports:
          - containerPort: 9007
        image: juicedata/mount:ee-5.0.14-a38b96d
        # 按照实际情况调整资源请求和约束
        # 参考文档：https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # 调整缓存目录的路径，如有多个缓存目录需要定义多个 volume
      # 参考文档：https://juicefs.com/docs/zh/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```
