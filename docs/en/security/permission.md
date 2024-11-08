---
title: Manage File Permission
---

JuiceFS is fully POSIX-compatible, simply manage file permissions with Unix-like [UID](https://en.wikipedia.org/wiki/User_identifier)/[GID](https://en.wikipedia.org/wiki/Group_identifier).

## Deploy

For example, with dynamic provisioning, first create a Kubernetes Secret:

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

Create StorageClass and PersistentVolumeClaim (PVC):

```yaml
kubectl apply -f - <<EOF
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

## Set permissions in Pod

```yaml {10,20-21,29,39-40,48,58-59}
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

## Verify file permission in JuiceFS volume

The `owner` container is run as user `1000` and group `3000`. Check the file it created owned by `1000:3000` under permission `-rw-r--r--` since umask is `0022`:

```sh
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c owner -- id
uid=1000 gid=3000 groups=3000
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c owner -- umask
0022
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c owner -- ls -l /data
total 707088
-rw-r--r--   1 1000 3000      3780 Aug  9 11:23 out-juicefs-app-perms-7c6c95b68-76g8g.txt
```

The `group` container is run as user `2000` and group `3000`. Check the file is readable by other user in the group:

```sh
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c group -- id
uid=2000 gid=3000 groups=3000
>> kubectl logs juicefs-app-perms-7c6c95b68-76g8g group
Fri Aug 9 10:08:32 UTC 2019
Fri Aug 9 10:08:37 UTC 2019
...
```

The `other` container is run as user `3000` and group `4000`. Check the file is not writable for users not in the group:

```sh
>> kubectl exec -it juicefs-app-perms-7c6c95b68-76g8g -c other -- id
uid=3000 gid=4000 groups=4000
>> kubectl logs juicefs-app-perms-7c6c95b68-76g8g -c other
/bin/sh: /data/out-juicefs-app-perms-7c6c95b68-76g8g.txt: Permission denied
...
```
