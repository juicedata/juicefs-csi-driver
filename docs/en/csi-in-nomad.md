---
sidebar_label: CSI in Nomad
---

# How to Use JuiceFS CSI Driver in Nomad

## Install JuiceFS CSI Driver

### Prerequisites

1. Nomad v0.12.0 or greater.
2. Enable privileged Docker jobs. If your Nomad client configuration does not already specify a Docker plugin configuration, this minimal one will allow privileged containers. Add it to your Nomad client configuration and restart Nomad.

    ```
    plugin "docker" {
        config {
            allow_privileged = true
        }
    }
    ```

### Install CSI Controller

Save the following configuration as a file `csi-controller.nomad`:

```
job "jfs-controller" {
  datacenters = ["dc1"]
  type = "system"

  group "controller" {
    task "plugin" {
      driver = "docker"

      config {
        image = "juicedata/juicefs-csi-driver:nightly"

        args = [
          "--endpoint=unix://csi/csi.sock",
          "--logtostderr",
          "--nodeid=test",
          "--v=5",
          "--by-process=true"
        ]

        privileged = true
      }

      csi_plugin {
        id        = "juicefs0"
        type      = "controller"
        mount_dir = "/csi"  
      }
      resources {
        cpu    = 100
        memory = 512
      }
    }
  }
}
```

Run CSI Controller job：

```shell
$ nomad job run csi-controller.nomad
==> 2022-03-14T17:00:20+08:00: Monitoring evaluation "2287baf7"
    2022-03-14T17:00:20+08:00: Evaluation triggered by job "jfs-controller"
    2022-03-14T17:00:20+08:00: Allocation "00806191" created: node "0673a790", group "controller"
==> 2022-03-14T17:00:21+08:00: Monitoring evaluation "2287baf7"
    2022-03-14T17:00:21+08:00: Allocation "00806191" status changed: "pending" -> "running" (Tasks are running)
    2022-03-14T17:00:21+08:00: Evaluation status changed: "pending" -> "complete"
==> 2022-03-14T17:00:21+08:00: Evaluation "2287baf7" finished with status "complete"
```

Run command `nomad job status jfs-controller` to check if CSI Controller runs successfully:

```shell
$ nomad job status jfs-controller
ID            = jfs-controller
Name          = jfs-controller
Submit Date   = 2022-03-14T17:00:20+08:00
Type          = system
Priority      = 50
Datacenters   = dc1
Namespace     = default
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
controller  0       0         1        0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created     Modified
00806191  0673a790  controller  0        run      running  31m47s ago  31m42s ago
```

In the above output, if the `Allocation` status is `running`, it means CSI Controller runs successfully.

### Install CSI Node

Save the following configuration as a file `csi-node.nomad`.

```
job "jfs-node" {
  datacenters = ["dc1"]
  type = "system"

  group "nodes" {
    task "juicefs-plugin" {
      driver = "docker"

      config {
        image = "juicedata/juicefs-csi-driver:nightly"

        args = [
          "--endpoint=unix://csi/csi.sock",
          "--logtostderr",
          "--v=5",
          "--nodeid=test",
          "--by-process=true",
        ]

        privileged = true
      }

      csi_plugin {
        id        = "juicefs0"
        type      = "node"
        mount_dir = "/csi"
      }
      resources {
        cpu    = 1000
        memory = 1024
      }
    }
  }
}
```

Run CSI Node job:

```shell
$ nomad job run csi-node.nomad
==> 2022-03-14T17:01:15+08:00: Monitoring evaluation "31d7ed49"
    2022-03-14T17:01:15+08:00: Evaluation triggered by job "jfs-node"
    2022-03-14T17:01:15+08:00: Allocation "047a1386" created: node "0673a790", group "nodes"
    2022-03-14T17:01:15+08:00: Evaluation status changed: "pending" -> "complete"
==> 2022-03-14T17:01:15+08:00: Evaluation "31d7ed49" finished with status "complete"
```

Run command `nomad job status jfs-node` to check if CSI Node runs successfully:

```shell
$ nomad job status jfs-node
ID            = jfs-node
Name          = jfs-node
Submit Date   = 2022-03-14T17:01:15+08:00
Type          = system
Priority      = 50
Datacenters   = dc1
Namespace     = default
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
nodes       0       0         1        0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created     Modified
047a1386  0673a790  nodes       0        run      running  28m41s ago  28m35s ago
```

In the above output, if the `Allocation` status is `running`, it means CSI Node runs successfully.

### Create Volume

#### Community edition

Save the following configuration as a file `volume.hcl`.

```
type = "csi"
id = "juicefs-volume"
name = "juicefs-volume"

capability {
access_mode = "multi-node-multi-writer"
attachment_mode = "file-system"
}
plugin_id = "juicefs0"

secrets {
  name="juicefs-volume"
  metaurl="redis://172.16.254.29:6379/0"
  bucket="http://172.16.254.29:9000/minio/test"
  storage="minio"
  access-key="minioadmin"
  secret-key="minioadmin"
}
```

- `name`: The JuiceFS file system name.
- `metaurl`: Connection URL for metadata engine (e.g. Redis). Read [this document](https://juicefs.com/docs/community/databases_for_metadata) for more information.
- `storage`: Object storage type, such as `s3`, `gs`, `oss`. Read [this document](https://juicefs.com/docs/community/how_to_setup_object_storage) for the full supported list.
- `bucket`: Bucket URL. Read [this document](https://juicefs.com/docs/community/how_to_setup_object_storage) to learn how to setup different object storage.
- `access-key`: Access key.
- `secret-key`: Secret key.

Create volume:

```shell
$ nomad volume create volume.hcl
Created external volume juicefs-volume with ID juicefs-volume
```

#### Cloud service edition

Save the following configuration as a file `volume.hcl`.

```
type = "csi"
id = "juicefs-volume"
name = "juicefs-volume"

capability {
access_mode = "multi-node-multi-writer"
attachment_mode = "file-system"
}
plugin_id = "juicefs0"

secrets {
  name="juicefs-volume"
  token="**********"
  access-key="*****"
  secret-key="*****"
}
```

- `name`: JuiceFS file system name
- `token`: JuiceFS managed token. Read [this document](https://juicefs.com/docs/cloud/metadata#token-management) for more details.
- `access-key`: Object storage access key
- `secret-key`: Object storage secret key

Create volume:

```shell
$ nomad volume create volume.hcl
Created external volume juicefs-volume with ID juicefs-volume
```

### Use volume in app

After the volume is created, it can be used in the application. For details, please refer to [official documentation](https://www.nomadproject.io/docs/job-specification/volume). Such as:

```
job "demo" {
  datacenters = ["dc1"]
  group "node" {
    count = 1

    volume "cache-volume" {
      type            = "csi"
      source          = "juicefs-volume"
      attachment_mode = "file-system"
      access_mode     = "multi-node-multi-writer"
    }

    network {
      port "db" {
        to = 8000
      }
    }

    task "nginx" {
      driver = "docker"
      config {
        image = "nginx"
        ports = ["db"]
      }
      resources {
        cpu    = 500
        memory = 256
      }

      volume_mount {
        volume      = "cache-volume"
        destination = "/data/job"
      }
    }
  }
}
```

Run job:

```shell
$ nomad job run job.nomad
==> 2022-03-14T17:11:54+08:00: Monitoring evaluation "e45504d5"
    2022-03-14T17:11:54+08:00: Evaluation triggered by job "demo"
    2022-03-14T17:11:54+08:00: Allocation "1ccca0b4" created: node "0673a790", group "node"
==> 2022-03-14T17:11:55+08:00: Monitoring evaluation "e45504d5"
    2022-03-14T17:11:55+08:00: Evaluation within deployment: "e603f13e"
    2022-03-14T17:11:55+08:00: Evaluation status changed: "pending" -> "complete"
==> 2022-03-14T17:11:55+08:00: Evaluation "e45504d5" finished with status "complete"
==> 2022-03-14T17:11:55+08:00: Monitoring deployment "e603f13e"
  ✓ Deployment "e603f13e" successful

    2022-03-14T17:12:09+08:00
    ID          = e603f13e
    Job ID      = demo
    Job Version = 0
    Status      = successful
    Description = Deployment completed successfully

    Deployed
    Task Group  Desired  Placed  Healthy  Unhealthy  Progress Deadline
    node        1        1       1        0          2022-03-14T17:22:08+08:00
```

After the job runs successfully, you can check whether JuiceFS is mounted successfully:

```shell
$ nomad alloc exec -i -t 1ccca0b4 bash
root@159f51ab7ea5:/# df -h
Filesystem              Size  Used Avail Use% Mounted on
overlay                  40G  8.7G   29G  24% /
tmpfs                    64M     0   64M   0% /dev
tmpfs                   3.8G     0  3.8G   0% /sys/fs/cgroup
shm                      64M     0   64M   0% /dev/shm
/dev/vda1                40G  8.7G   29G  24% /local
tmpfs                   1.0M     0  1.0M   0% /secrets
JuiceFS:juicefs-volume  1.0P  4.0K  1.0P   1% /data/job
tmpfs                   3.8G     0  3.8G   0% /proc/acpi
tmpfs                   3.8G     0  3.8G   0% /proc/scsi
tmpfs                   3.8G     0  3.8G   0% /sys/firmware
root@159f51ab7ea5:/#
```
