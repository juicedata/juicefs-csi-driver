---
title: Running other JuiceFS applications
sidebar_position: 6
---

Strictly speaking, this chapter isn't directly related to JuiceFS CSI Driver, they are generic Kubernetes applications that can run without our CSI Driver. For example:

* Running `juicefs sync` in a Kubernetes CronJob, to automatically sync data
* Running `juicefs webdav` or `juicefs gateway` (S3 gateway) inside Kubernetes
* &#8203;<Badge type="primary">On-prem</Badge> Deploy dedicated cache cluster within Kubernetes

The JuiceFS Client has so many other capabilities that we simply cannot include everything, if your cases aren't included, you can use all the examples in this chapter as a guideline and compose your own. And if this goes well, a [documentation PR](https://github.com/juicedata/juicefs-csi-driver/tree/master/docs/zh_cn/guide) is welcomed too.

## Running `juicefs sync` as a Kubernetes CronJob {#juicefs-sync}

`juicefs sync` is a great data migration tool, you can use CronJob to set up a automatic sync schedule.

Using our enterprise edition as an example, assuming user need to interact directly with JuiceFS Volume, `auth` command is needed, but you can choose to omit that part if you're dealing with 2 object storage systems.

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  # Customize name and namespace
  name: juicefs-sync
  namespace: default
spec:
  # Customize schedule accordingly
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
              # Below shell code is only needed in on-premise environments, which unpacks JSON and set its key-value pairs as environment variables
              for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
                echo "export $keyval"
                eval export $keyval
              done

          # If sync needs to access JuiceFS Volume directly, using the jfs:// protocol is recommended
              # However, this method requires local client config, which is fetched by the auth command
              /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # Change the command parameters accordingly
              # ref: https://juicefs.com/docs/zh/cloud/guide/sync/
              /usr/bin/juicefs sync oss://${ACCESS_KEY}:${SECRET_KEY}@myjfs-bucket.oss-cn-hongkong.aliyuncs.com/chaos-ee-test/juicefs_uuid jfs://$VOL_NAME
            env:
            # The secret containing volume credentials, must reside in the same namespace
            # ref: https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
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
            # Only needed in on-prem environments
            - name: ENVS
              valueFrom:
                secretKeyRef:
                  key: envs
                  name: juicefs-secret
            # Use mount image
            # ref: https://juicefs.com/docs/zh/csi/guide/custom-image
            image: juicedata/mount:ee-5.0.14-a38b96d
            # Adjust resource definition accordingly
            # ref: https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
            resources:
              requests:
                memory: 500Mi
```

## Deploy distributed cache cluster {#distributed-cache-cluster}

Refer to below examples to deploy a stable, dedicated cache cluster within Kubernetes, on selected nodes.

StatefulSet and DaemonSet are both provided, they don't come with any functionality differences, but note that when you adjust config and restart a StatefulSet, pods are restarted one by one in descending order. While daemonset executes the restart according to its own `updateStrategy`. When faced with a large cluster, carefully configure this strategy to avoid service impact.

Apart from that, there's no actual differences between the two.

### DaemonSet

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  # Customize name and namespace
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
      # Using hostNetwork allows pod to run with a static IP, when pod is recreated, IP will not change so that cache data persists
      hostNetwork: true
      containers:
      - name: juicefs-cache
        command:
        - sh
        - -c
        - |
          # Below shell code is only needed in on-premise environments, which unpacks JSON and set its key-value pairs as environment variables
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # Authenticate and mount JuiceFS, all environment variables comes from the volume credentials within the Kubernetes Secret
          # ref: https://juicefs.com/docs/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # Must use --foreground to make JuiceFS Client process run in foreground, adjust other mount options to your need (especially --cache-group)
          # ref: https://juicefs.com/docs/cloud/reference/commands_reference#mount
          /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --foreground --cache-dir=/data/jfsCache --cache-size=512000 --cache-group=jfscache
        env:
        # The Secret that contains volume credentials, must reside in same namespace as this StatefulSet
        # ref: https://juicefs.com/docs/csi/guide/pv#cloud-service
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
        # Only needed in on-prem environments
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        # Use mount image
        # ref: https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.2-69f82b3
        lifecycle:
          # Umount the file system at exit
          preStop:
            exec:
              command:
              - sh
              - -c
              - umount /mnt/jfs
        # Adjust resource accordingly
        # ref: https://juicefs.com/docs/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        # Mounting file system requires system privilege
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # Adjust cache directory, define multiple volumes if need to use multiple cache directories
      # ref: https://juicefs.com/docs/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

### StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  # name and namespace are customizable
  name: juicefs-cache-group
  namespace: kube-system
spec:
  # cache group peer amount
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
      # Run a single cache group peer on each node
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: jfs-role
                operator: In
                values:
                - cache
            topologyKey: kubernetes.io/hostname
      # Using hostNetwork allows pod to run with a static IP, when pod is recreated, IP will not change so that cache data persists
      hostNetwork: true
      containers:
      - name: juicefs-cache
        command:
        - sh
        - -c
        - |
          # Below shell code is only needed in on-premise environments, which unpacks JSON and set its key-value pairs as environment variables
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # Authenticate and mount JuiceFS, all environment variables comes from the volume credentials within the Kubernetes Secret
          # ref: https://juicefs.com/docs/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # Must use --foreground to make JuiceFS Client process run in foreground, adjust other mount options to your need (especially --cache-group)
          # ref: https://juicefs.com/docs/cloud/reference/commands_reference#mount
          /usr/bin/juicefs mount $VOL_NAME /mnt/jfs --foreground --cache-dir=/data/jfsCache --cache-size=512000 --cache-group=jfscache
        env:
        # The Secret that contains volume credentials, must reside in same namespace as this StatefulSet
        # ref: https://juicefs.com/docs/csi/guide/pv#cloud-service
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
        # Only needed in on-prem environments
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        volumeMounts:
        - mountPath: /root/.juicefs
          name: jfs-root-dir
        # Use the mount pod container image
        # ref: https://juicefs.com/docs/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.2-69f82b3
        lifecycle:
          # Unmount file system when exiting
          preStop:
            exec:
              command:
              - sh
              - -c
              - umount /mnt/jfs
        # Adjust resource accordingly
        # ref: https://juicefs.com/docs/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        # Mounting file system requires system privilege
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # Adjust cache directory, define multiple volumes if need to use multiple cache directories
      # ref: https://juicefs.com/docs/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

### 使用缓存组

上方示范便是在集群中启动了 JuiceFS 缓存集群，其缓存组名为 `jfscache`，那么为了让应用程序的 JuiceFS 客户端使用该缓存集群，需要让他们一并加入这个缓存组，并额外添加 `--no-sharing` 这个挂载参数，这样一来，应用程序的 JuiceFS 客户端虽然加入了缓存组，但却不参与缓存数据的构建，避免了客户端频繁创建、销毁所导致的缓存数据不稳定。

以动态配置为例，按照下方示范修改挂载参数即可，关于在 `mountOptions` 调整挂载配置，详见[「挂载参数」](../guide/pv.md#mount-options)。

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

## Running JuiceFS S3 Gateway {#juicefs-gateway}

Running S3 Gateway via our [Helm Chart](https://github.com/juicedata/charts) is recommended. Use below example as reference (Service and Ingress is ommited, you need to create them in your environment as well).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # Customize name and namespace
  name: juicefs-gateway
  namespace: default
spec:
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
          # Below shell code is only needed in on-premise environments, which unpacks JSON and set its key-value pairs as environment variables
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # Authenticate and mount JuiceFS, all environment variables comes from the volume credentials within the Kubernetes Secret
          # ref: https://juicefs.com/docs/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

      # Directly use object storage AKSK as MinIO credentials, for convenience's sake
          export MINIO_ROOT_USER=${ACCESS_KEY}
          export MINIO_ROOT_PASSWORD=${SECRET_KEY}

          # ref: https://juicefs.com/docs/zh/cloud/reference/commands_reference#gateway
          /usr/bin/juicefs gateway $VOL_NAME 0.0.0.0:9000 --cache-dir=/data/jfsCache
        env:
        # The secret containing volume credentials, must reside in the same namespace
        # ref: https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
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
        # Only needed in on-prem environments
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        ports:
          - containerPort: 9000
        # Use mount image
        # ref: https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.14-a38b96d
        # Adjust resource definition accordingly
        # ref: https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # Adjust cache directory, define multiple volumes if need to use multiple cache directories
      # ref: https://juicefs.com/docs/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```

## Running JuiceFS WebDAV {#juicefs-webdav}

Running WebDAV via Kubernetes Deployment is recommended. Use below example as reference (Service and Ingress is ommited, you need to create them in your environment as well).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  # Customize name and namespace
  name: juicefs-webdav
  namespace: default
spec:
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
          # Below shell code is only needed in on-premise environments, which unpacks JSON and set its key-value pairs as environment variables
          for keyval in $(echo $ENVS | sed -e 's/": "/=/g' -e 's/{"//g' -e 's/", "/ /g' -e 's/"}//g' ); do
            echo "export $keyval"
            eval export $keyval
          done

          # Authenticate and mount JuiceFS, all environment variables comes from the volume credentials within the Kubernetes Secret
          # ref: https://juicefs.com/docs/cloud/getting_started#create-file-system
          /usr/bin/juicefs auth --token=${TOKEN} --access-key=${ACCESS_KEY} --secret-key=${SECRET_KEY} ${VOL_NAME}

          # Set username and password
          export WEBDAV_USER=root
          export WEBDAV_PASSWORD=1234

          # ref: https://juicefs.com/docs/zh/cloud/reference/commands_reference#webdav
          /usr/bin/juicefs webdav $VOL_NAME 0.0.0.0:9007 --cache-dir=/data/jfsCache
        env:
        # The secret containing volume credentials, must reside in the same namespace
        # ref: https://juicefs.com/docs/zh/csi/guide/pv#cloud-service
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
        # Only needed in on-prem environments
        - name: ENVS
          valueFrom:
            secretKeyRef:
              key: envs
              name: juicefs-secret
        ports:
          - containerPort: 9007
        # Use mount image
        # ref: https://juicefs.com/docs/zh/csi/guide/custom-image
        image: juicedata/mount:ee-5.0.14-a38b96d
        # Adjust resource definition accordingly
        # ref: https://juicefs.com/docs/zh/csi/guide/resource-optimization#mount-pod-resources
        resources:
          requests:
            memory: 500Mi
        volumeMounts:
        - mountPath: /data/jfsCache
          name: cache-dir
        - mountPath: /root/.juicefs
          name: jfs-root-dir
      volumes:
      # Adjust cache directory, define multiple volumes if need to use multiple cache directories
      # ref: https://juicefs.com/docs/cloud/guide/cache#client-read-cache
      - name: cache-dir
        hostPath:
          path: /data/jfsCache
          type: DirectoryOrCreate
      - name: jfs-root-dir
        emptyDir: {}
```
