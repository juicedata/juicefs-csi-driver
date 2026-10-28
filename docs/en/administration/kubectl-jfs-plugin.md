---
title: Use the kubectl-jfs Plugin
slug: /kubectl-jfs-plugin
sidebar_position: 9
---

[`kubectl-jfs`](https://github.com/juicedata/kubectl-jfs-plugin) is a kubectl plugin for inspecting, debugging, and upgrading JuiceFS CSI workloads in Kubernetes. After installation, invoke it as `kubectl jfs`.

## Install the plugin

The one-click installation script supports Linux and macOS. It installs the latest version for the current operating system and hardware architecture:

```shell
# The default installation path is /usr/local/bin
curl -sSL https://d.juicefs.com/kubectl-jfs-install | sh -
```

Verify the installation and view the help information:

```shell
kubectl jfs --version
kubectl jfs --help
```

## Connect to a cluster

The plugin uses the current kubeconfig and context by default. Standard kubectl flags such as `--kubeconfig`, `--context`, and `--namespace` are supported. The user running the plugin must have permission to read or modify the corresponding Kubernetes resources.

JuiceFS CSI Driver is installed in `kube-system` by default. If it is installed in another namespace, use `-m` or `--mount-namespace` to specify the CSI Driver namespace:

```shell
kubectl jfs -m juicefs-system mount
```

`--namespace` (or `-n`) specifies the application namespace, while `--mount-namespace` (or `-m`) specifies the namespace of JuiceFS CSI Driver and Mount Pods. Do not confuse the two options.

## Inspect JuiceFS resources

Use the following commands to inspect Mount Pods, application Pods using JuiceFS PVs, JuiceFS PVs, and PVCs:

```shell
# List Mount Pods
kubectl jfs mount

# List application Pods using JuiceFS PVs in the current namespace
kubectl jfs pod

# List application Pods and PVCs in all namespaces
kubectl jfs pod --all-namespaces
kubectl jfs pvc --all-namespaces

# List JuiceFS PVs and PVCs in the current namespace
kubectl jfs pv
kubectl jfs pvc
```

`pod` can be abbreviated as `po`. To inspect one application namespace, use `--namespace <namespace>`.

## Debug Pods, PVCs, and PVs

The `debug` command summarizes the status of application Pods, CSI Nodes, PVCs, PVs, and Mount Pods. It also reports possible failure reasons and suggests the next troubleshooting steps:

```shell
kubectl jfs debug pod <pod-name> --namespace <namespace>
kubectl jfs debug pvc <pvc-name> --namespace <namespace>
kubectl jfs debug pv <pv-name>
```

After locating an unhealthy Mount Pod, inspect its events and logs:

```shell
kubectl describe pod <mount-pod-name> --namespace kube-system
kubectl logs <mount-pod-name> --namespace kube-system --all-containers
```

Adjust the namespace if JuiceFS CSI Driver is not installed in `kube-system`.

## Collect access logs and warm up the cache

Use `accesslog` to continuously print the JuiceFS access log from a Mount Pod:

```shell
kubectl jfs accesslog <mount-pod-name>
```

For details about analyzing access logs, see the JuiceFS [Community Edition](https://juicefs.com/docs/community/fault_diagnosis_and_analysis#access-log) or [Cloud Service](https://juicefs.com/docs/cloud/administration/fault_diagnosis_and_analysis#oplog) documentation.

Use `warmup` to warm up the entire mount:

```shell
kubectl jfs warmup <mount-pod-name>
```

:::warning
The current plugin implementation does not handle the documented `<subpath>` argument correctly. Even if a subpath is specified, the entire mount is warmed up. Evaluate object storage traffic and cache capacity before running this command.
:::

Run `kubectl jfs accesslog --help` or `kubectl jfs warmup --help` for all available options.

## Upgrade one Mount Pod

The `upgrade` command upgrades one Mount Pod. By default, it only upgrades the JuiceFS client binary. With `--recreate`, it recreates the Mount Pod to apply image and other Pod configuration changes:

```shell
# Binary upgrade
kubectl jfs upgrade <mount-pod-name>

# Recreate upgrade
kubectl jfs upgrade <mount-pod-name> --recreate
```

For differences, version requirements, and prerequisites, see [Upgrade JuiceFS Client](./upgrade-juicefs-client.md#smooth-upgrade).

## Batch upgrades

A batch upgrade creates a Kubernetes Job that upgrades Mount Pods or injected JuiceFS Sidecars. The recommended workflow is:

1. Preview the changes with `batch diff`.
2. Create the upgrade Job with `batch upgrade`.
3. Monitor the Job with `batch list` and `batch describe`.
4. Pause, resume, stop, or delete the Job when necessary.

### Preview Mount Pod changes

```shell
# List all Mount Pods that need to be upgraded
kubectl jfs batch diff

# Show the configuration diff for one Mount Pod
kubectl jfs batch diff <mount-pod-name>

# Limit the preview to one node
kubectl jfs batch diff --node <node-name>
```

### Upgrade Mount Pods in batches

```shell
# Upgrade all Mount Pods that need an update
kubectl jfs batch upgrade

# Upgrade Mount Pods on one node
kubectl jfs batch upgrade --node <node-name>

# Upgrade Mount Pods associated with one PVC
kubectl jfs batch upgrade --pvc <pvc-name>

# Run up to five upgrades concurrently
kubectl jfs batch upgrade --worker 5

# Continue when an individual upgrade fails
kubectl jfs batch upgrade --ignore-error

# Skip the confirmation prompt
kubectl jfs batch upgrade --quiet
kubectl jfs batch upgrade -y
```

`batch upgrade` prints the planned changes and asks for confirmation before creating the upgrade Job. Mount Pods are recreated during a batch upgrade. Targets already covered by another running batch upgrade Job are skipped automatically.

| Option | Default | Description |
| --- | --- | --- |
| `--node <name>` | All nodes | Upgrade only targets on the specified node. |
| `--pvc <name>` | All PVCs | Upgrade only Mount Pods associated with the specified PVC. The PVC name must be unique across the cluster. |
| `--worker <number>` | `1` | Maximum number of concurrent upgrades. |
| `--ignore-error` | `false` | Continue after an individual target fails. |
| `-y`, `--quiet` | `false` | Skip the confirmation prompt. |

In production, run `batch diff` first and start with a limited scope and the default concurrency. Expand the scope or increase concurrency only after confirming that the upgrade behaves as expected.

### Configure the batch upgrade Job

The batch upgrade Job runs `juicefs-csi-dashboard upgrade`. By default, the plugin reads the image and ServiceAccount from the `juicefs-csi-dashboard` Deployment in the CSI Driver namespace.

If the Deployment cannot be read, or if you need to override the defaults, set these environment variables:

| Environment variable | Description |
| --- | --- |
| `DASHBOARD_IMAGE` | Container image used by the batch upgrade Job. |
| `JUICEFS_CSI_DASHBOARD_SA` | ServiceAccount used by the batch upgrade Job. It defaults to `juicefs-csi-dashboard-sa` if the value cannot be read from the Dashboard Deployment. |
| `BATCH_UPGRADE_TIMEOUT_SECONDS` | Batch upgrade timeout in seconds. The value must be an integer. |

Set the image and ServiceAccount together to avoid reading either value from the Dashboard Deployment:

```shell
DASHBOARD_IMAGE=juicedata/csi-dashboard:<version> \
JUICEFS_CSI_DASHBOARD_SA=juicefs-csi-dashboard-sa \
  kubectl jfs batch upgrade
```

For example, set a one-hour timeout with:

```shell
BATCH_UPGRADE_TIMEOUT_SECONDS=3600 kubectl jfs batch upgrade
```

If only one of `DASHBOARD_IMAGE` and `JUICEFS_CSI_DASHBOARD_SA` is set, the plugin still reads the missing value from the `juicefs-csi-dashboard` Deployment.

### Batch upgrade injected Sidecars

Upgrade JuiceFS CSI Driver to v0.33.2 or later before using this feature. It only supports upgrading the JuiceFS client version and changing mount arguments, without restarting application Pods. To change mount arguments, both the current and target JuiceFS client versions in the Sidecar must be v5.4.3 or later. Other Sidecar configuration changes still require rebuilding application Pods.

Add the persistent `--sidecar` option after `batch` and before its subcommand to operate on JuiceFS Sidecars injected into application Pods:

```shell
kubectl jfs batch --sidecar <subcommand>
```

A namespace is required for Sidecar batch upgrades:

```shell
# Preview all Sidecars that need an upgrade in a namespace
kubectl jfs batch --sidecar diff --namespace <namespace>

# Show the client version and mount argument differences for one Pod
kubectl jfs batch --sidecar diff <pod-name> --namespace <namespace>

# Create a Sidecar batch upgrade Job
kubectl jfs batch --sidecar upgrade --namespace <namespace>

# Limit the node and concurrency
kubectl jfs batch --sidecar upgrade \
  --namespace <namespace> \
  --node <node-name> \
  --worker 5
```

Sidecar batch upgrades have these constraints:

- `--namespace` is required.
- `--pvc` cannot be used with `--sidecar`.
- `--worker` must be greater than zero.
- Pods that are not Ready or are being deleted are skipped.
- Sidecars already covered by a running batch upgrade Job are skipped.

The plugin derives the target state from the PVC, PV, Secret, and Node configuration. Depending on the detected differences, it updates the JuiceFS client version, mount arguments, or both. During a JuiceFS Community Edition client upgrade, existing mount arguments are preserved.

### Monitor and manage batch upgrade Jobs

```shell
# List Mount Pod or Sidecar batch upgrade Jobs
kubectl jfs batch list
kubectl jfs batch --sidecar list

# Show Job details
kubectl jfs batch describe <job-name>

# Manage a Job
kubectl jfs batch pause <job-name>
kubectl jfs batch resume <job-name>
kubectl jfs batch stop <job-name>
kubectl jfs batch delete <job-name>
```

A running Job must be paused or stopped before it can be deleted. A paused Job can be resumed or deleted.

If JuiceFS CSI Driver is not installed in `kube-system`, use `-m` to specify its namespace for all batch commands:

```shell
kubectl jfs -m juicefs-system batch diff
kubectl jfs -m juicefs-system batch upgrade
kubectl jfs -m juicefs-system batch describe <job-name>

kubectl jfs -m juicefs-system batch --sidecar upgrade \
  --namespace <application-namespace>
```

## Get command help

This page documents common options only. Commands and options may change as the plugin evolves. Refer to the help information from the installed version:

```shell
kubectl jfs --help
kubectl jfs <command> --help
kubectl jfs batch <subcommand> --help
```
