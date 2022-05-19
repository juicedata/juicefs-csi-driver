# 如何在 JuiceFS 中使用权限管理

JuiceFS 实现了 [POSIX](https://en.wikipedia.org/wiki/POSIX) 。没有
使用类 Unix [UID](https://en.wikipedia.org/wiki/User_identifier) 管理权限的额外工作
和 [GID](https://en.wikipedia.org/wiki/Group_identifier) 。

## 部署

您可以使用 [静态配置](static-provisioning.md) 或 [动态配置](dynamic-provisioning.md) 。我们以动态提供为例：

创建 Secret：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: juicefs-secret
type: Opaque
stringData:
  name: <NAME>
  metaurl: redis://[:<PASSWORD>]@<HOST>:6379[/<DB>]
  storage: s3
  bucket: https://<BUCKET>.s3.<REGION>.amazonaws.com
  access-key: <ACCESS_KEY>
  secret-key: <SECRET_KEY>
```

创建 StorageClass, PersistentVolumeClaim (PVC)：

```yaml
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc
  namespace: default
provisioner: csi.juicefs.com
parameters:
  csi.storage.k8s.io/provisioner-secret-name: juicefs-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  csi.storage.k8s.io/node-publish-secret-name: juicefs-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: juicefs-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Pi
  storageClassName: juicefs-sc
EOF
```

## 在 pod 中设置权限

```yaml
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: juicefs-app
spec:
  template:
    spec:
      containers:
      - name: owner
        image: centos
        command: ["/bin/sh"]
        args: ["-c", "while true; do echo $(date -u) >> /data/out-$(POD).txt; sleep 5; done"]
        env:
        - name: POD
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        securityContext:
          runAsUser: 1000
          runAsGroup: 3000
        resources:
          limits:
            cpu: "20m"
            memory: "55M"
        volumeMounts:
        - name: data
          mountPath: /data
      - name: group
        image: centos
        command: ["/bin/sh"]
        args: ["-c", "tail -f /data/out-$(POD).txt"]
        env:
        - name: POD
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        securityContext:
          runAsUser: 2000
          runAsGroup: 3000
        resources:
          limits:
            cpu: "20m"
            memory: "55M"
        volumeMounts:
        - name: data
          mountPath: /data
      - name: other
        image: centos
        command: ["/bin/sh"]
        args: ["-c", "while true; do echo $(date -u) >> /data/out-$(POD).txt; sleep 5; done"]
        env:
        - name: POD
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        securityContext:
          runAsUser: 3000
          runAsGroup: 4000
        resources:
          limits:
            cpu: "20m"
            memory: "55M"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: juicefs-pvc
EOF
```

## 检查 JuiceFS volume 中的权限如何工作

`owner` 容器以用户 `1000` 和组 `3000` 的身份运行。在权限 `-rw-r--r--` 下检查它创建的由 1000:3000 拥有的文件，因为 umask 是`0022`

```sh
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c owner -- id
uid=1000 gid=3000 groups=3000
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c owner -- umask
0022
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c owner -- ls -l /data
total 707088
-rw-r--r--   1 1000 3000      3780 Aug  9 11:23 out-juicefs-app-perms-7c6c95b68-76g8g.txtkubectl get pods
```

`group` 容器以用户 `2000` 和组 `3000` 运行。检查该文件是否可由组中的其他用户读取。

```sh
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c group -- id
uid=2000 gid=3000 groups=3000
>> kubectl logs juicefs-app-perms-7c6c95b68-76g8g group
Fri Aug 9 10:08:32 UTC 2019
Fri Aug 9 10:08:37 UTC 2019
...
```

`other` 容器以用户 `3000` 和组 `4000` 运行。检查文件对于不在组中的用户不可写。

```sh
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c other -- id
uid=3000 gid=4000 groups=4000
>> kubectl logs juicefs-app-perms-7c6c95b68-76g8g -c other
/bin/sh: /data/out-juicefs-app-perms-7c6c95b68-76g8g.txt: Permission denied
...
```
