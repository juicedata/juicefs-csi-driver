---
slug: /csi-in-nomad
sidebar_label: 在 Nomad 中使用
---

# 在 Nomad 中使用 CSI 驱动

:::note 注意
此特性需使用 0.13.2 及以上版本的 JuiceFS CSI 驱动
:::

## 安装 JuiceFS CSI 驱动

### 前置条件

1. Nomad 版本在 v0.12.0 及以上。
2. 开启 privileged Docker jobs。如果您的 Nomad 客户端配置尚未指定 Docker 插件配置，将以下最小配置文件添加到您的 Nomad 客户端配置并重新启动 Nomad：

    ```hcl
    plugin "docker" {
        config {
            allow_privileged = true
        }
    }
    ```

### 安装 CSI Controller

将以下配置文件保存为文件 `csi-controller.nomad`：

```hcl title="csi-controller.nomad"
job "jfs-controller" {
  datacenters = ["dc1"]
  type = "system"

  group "controller" {
    task "plugin" {
      driver = "docker"

      config {
        image = "juicedata/juicefs-csi-driver:v0.14.1"

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
      env {
        POD_NAME = "csi-controller"
      }
    }
  }
}
```

启动 CSI Controller job：

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

运行命令 `nomad job status jfs-controller` 可检查 CSI Controller 是否正常运行：

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

在上述输出中，`Allocation` 状态为 `running` 即代表 CSI Controller 启动成功。

### 安装 CSI Node

将以下配置文件保存为文件 `csi-node.nomad`：

```hcl title="csi-node.nomad"
job "jfs-node" {
  datacenters = ["dc1"]
  type = "system"

  group "nodes" {
    task "juicefs-plugin" {
      driver = "docker"

      config {
        image = "juicedata/juicefs-csi-driver:v0.14.1"

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
      env {
        POD_NAME = "csi-node"
      }
    }
  }
}
```

启动 CSI Node job：

```shell
$ nomad job run csi-node.nomad
==> 2022-03-14T17:01:15+08:00: Monitoring evaluation "31d7ed49"
    2022-03-14T17:01:15+08:00: Evaluation triggered by job "jfs-node"
    2022-03-14T17:01:15+08:00: Allocation "047a1386" created: node "0673a790", group "nodes"
    2022-03-14T17:01:15+08:00: Evaluation status changed: "pending" -> "complete"
==> 2022-03-14T17:01:15+08:00: Evaluation "31d7ed49" finished with status "complete"
```

运行命令 `nomad job status jfs-node` 可检查 CSI Node 是否正常运行：

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

在上述输出中，`Allocation` 状态为 `running` 即代表 CSI Node 启动成功。

### 创建 volume

#### 社区版

将以下配置文件保存为文件 `volume.hcl`：

```hcl title="volume.hcl"
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

其中：

- `name`：JuiceFS 文件系统名称
- `metaurl`：元数据引擎的访问 URL（比如 Redis）。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/databases_for_metadata)。
- `storage`：对象存储类型，比如 `s3`、`gs`、`oss`。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/how_to_setup_object_storage)。
- `bucket`：Bucket URL。更多信息参考[这篇文档](https://juicefs.com/docs/zh/community/how_to_setup_object_storage)。
- `access-key`：对象存储的 access key。
- `secret-key`：对象存储的 secret key。

创建 volume：

```shell
$ nomad volume create volume.hcl
Created external volume juicefs-volume with ID juicefs-volume
```

#### 云服务版

将以下配置文件保存为文件 `volume.hcl`：

```hcl title="volume.hcl"
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
}
```

其中：

- `name`：JuiceFS 文件系统名称
- `token`：JuiceFS 管理 token。更多信息参考[这篇文档](https://juicefs.com/docs/zh/cloud/metadata#令牌管理)

创建 volume：

```shell
$ nomad volume create volume.hcl
Created external volume juicefs-volume with ID juicefs-volume
```

### 在应用中使用

Volume 创建好之后，就可以在应用中使用，具体可以参考[官方文档](https://www.nomadproject.io/docs/job-specification/volume)。如：

```hcl title="job.nomad"
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

运行 job：

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

Job 运行成功后，可以检查 JuiceFS 是否挂载成功：

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
