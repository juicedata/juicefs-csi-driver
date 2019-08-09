# Permissions

JuiceFS is [POSIX](https://en.wikipedia.org/wiki/POSIX)-compilant. There is no
extra effort to manage permissions with Unix-like [UID](https://en.wikipedia.org/wiki/User_identifier)
and [GID](https://en.wikipedia.org/wiki/Group_identifier).

## Resources

Ensure you have already get familiar with [static-provisioning](../static-provisioning/README.md) example.

We shall create a [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/) to demonstrate permission control in JuiceFS volume.

## Apply the configurations

Build the example with [kustomize](https://github.com/kubernetes-sigs/kustomize) and apply with `kubectl`

```s
kustomize build | kubectl apply -f -
```

or apply with kubectl >= 1.14

```s
kubectl apply -k .
```

## Check how permission works in JuiceFS volume

The `owner` container is run as user `1000` and group `3000`. Check the file it created owned by 1000:3000 under permission `-rw-r--r--` since
umask is `0022`

```sh
>> kubectl exec -it app-perms-7c6c95b68-76g8g -c owner -- id
uid=1000 gid=3000 groups=3000
>> kubectl exec -it app-perms-7c6c95b68-76g8g -c owner -- umask
0022
>> kubectl exec -it app-perms-7c6c95b68-76g8g -c owner -- ls -l /data
total 707088
-rw-r--r--   1 1000 3000      3780 Aug  9 11:23 out-app-perms-7c6c95b68-76g8g.txtkubectl get pods
```

The `group` container is run as user `2000` and group `3000`. Check the file is readable by other user in the group.

```sh
>> kubectl exec -it app-perms-7c6c95b68-76g8g -c group -- id
uid=2000 gid=3000 groups=3000
>> kubectl logs app-perms-7c6c95b68-76g8g group
Fri Aug 9 10:08:32 UTC 2019
Fri Aug 9 10:08:37 UTC 2019
...
```

The `other` container is run as user `3000` and group `4000`. Check the file is not writable for users not in the group.

```sh
>> kubectl exec -it app-perms-7c6c95b68-76g8g -c other -- id
uid=3000 gid=4000 groups=4000
>> kubectl logs app-perms-7c6c95b68-76g8g -c other
/bin/sh: /data/out-app-perms-7c6c95b68-76g8g.txt: Permission denied
...
```
