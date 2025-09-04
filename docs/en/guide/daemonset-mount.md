# DaemonSet Mount for StorageClass

This feature allows JuiceFS CSI Driver to deploy Mount Pods as DaemonSets instead of individual Pods when using StorageClass with mount sharing enabled. This provides better resource management and control over which nodes run Mount Pods.

## Overview

When `STORAGE_CLASS_SHARE_MOUNT` is enabled, JuiceFS CSI Driver shares Mount Pods across multiple PVCs that use the same StorageClass. By default, these are created as individual shared Pods. With the DaemonSet option, Mount Pods are deployed as DaemonSets, providing:

- **Better resource control**: DaemonSets ensure one Mount Pod per selected node
- **Node affinity support**: Control which nodes run Mount Pods using nodeAffinity
- **Automatic lifecycle management**: DaemonSets handle Pod creation/deletion automatically
- **Simplified operations**: Easier to manage and monitor Mount Pods
- **Works with existing StorageClasses**: No need to modify or recreate StorageClasses
- **Automatic mode transition**: Seamlessly switches from shared-pod to DaemonSet mode

## Prerequisites

### 1. Enable Mount Sharing

Set the `STORAGE_CLASS_SHARE_MOUNT` environment variable in **BOTH** the CSI Controller and CSI Node components:

```yaml
# For StatefulSet (Controller)
kubectl set env statefulset/juicefs-csi-controller -n kube-system STORAGE_CLASS_SHARE_MOUNT=true

# For DaemonSet (Node)
kubectl set env daemonset/juicefs-csi-node -n kube-system STORAGE_CLASS_SHARE_MOUNT=true
```

Or add to your Helm values:

```yaml
node:
  storageClassShareMount: true
controller:
  storageClassShareMount: true  # Note: This may need to be added to the Helm chart
```

### 2. Grant RBAC Permissions

The CSI Node service account needs permissions to manage DaemonSets. Add these permissions:

```yaml
- apiGroups:
  - apps
  resources:
  - daemonsets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
```

This is included in the latest deployment manifests (`deploy/k8s.yaml`).

## Configuration

### Mount Modes

JuiceFS CSI Driver supports three mount modes:

1. **`pvc`** (or `mountpod`): One mount pod per PVC - each PVC gets its own dedicated mount pod
2. **`shared-pod`**: One shared mount pod per StorageClass per node - multiple PVCs share a mount pod
3. **`daemonset`**: One DaemonSet for the entire StorageClass - ensures one mount pod per selected node

### Configure Mount Mode and Node Affinity

Create a ConfigMap to configure mount mode and optionally node affinity:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: juicefs-mount-config
  namespace: kube-system
data:
  # Default configuration for all StorageClasses
  default: |
    mode: shared-pod  # Options: "pvc", "shared-pod", "daemonset"
    # nodeAffinity is NOT required for pvc or shared-pod modes
  
  # Configuration for specific StorageClass by name
  my-storageclass: |
    mode: daemonset
    nodeAffinity:  # Required for daemonset mode to control which nodes run mount pods
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/workload
            operator: In
            values:
            - "compute"
```

**Important notes:**
- `mode`: Choose from `pvc`, `shared-pod`, or `daemonset`
- `nodeAffinity`: Only required for `daemonset` mode to control which nodes the DaemonSet runs on
  - NOT required for `pvc` or `shared-pod` modes (pods are created on-demand where PVCs are used)

This method works with existing StorageClasses without any modifications.

#### Method 2: StorageClass Parameters (For new StorageClasses)

For new StorageClasses, you can specify `nodeAffinity` directly in the parameters:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: juicefs-sc-daemonset
provisioner: csi.juicefs.com
parameters:
  # ... other parameters ...
  
  # Node affinity configuration for DaemonSet Mount Pods
  nodeAffinity: |
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: juicefs/mount-node
          operator: In
          values:
          - "true"
```

## How It Works

1. When a PVC is created using a StorageClass with DaemonSet mount enabled:
   - The CSI Driver checks if a DaemonSet for this StorageClass already exists
   - If not, it looks for node affinity configuration:
     - First checks the ConfigMap for StorageClass-specific or default configuration
     - Falls back to StorageClass parameters if specified
   - Creates a new DaemonSet with the configured node affinity
   - If DaemonSet exists, it adds a reference to the existing DaemonSet

2. The DaemonSet ensures Mount Pods are running on selected nodes:
   - Pods are automatically created on nodes matching the affinity rules
   - Mount paths are shared across PVCs using the same StorageClass

3. When a PVC is deleted:
   - The reference is removed from the DaemonSet
   - If no references remain, the DaemonSet is deleted

## Priority Order

The system checks for node affinity configuration in this order:

1. **StorageClass parameters** (if `nodeAffinity` is specified)
2. **ConfigMap with StorageClass name** as key
3. **ConfigMap default** configuration
4. **No affinity** (DaemonSet runs on all nodes)

## Use Cases

### Dedicated Mount Nodes

Label specific nodes for running Mount Pods:

```bash
kubectl label nodes node1 node2 node3 juicefs/mount-node=true
```

Then use nodeAffinity in StorageClass to target these nodes.

### High-Performance Nodes

Prefer nodes with better resources for Mount Pods:

```yaml
nodeAffinity: |
  preferredDuringSchedulingIgnoredDuringExecution:
  - weight: 100
    preference:
      matchExpressions:
      - key: node.kubernetes.io/instance-type
        operator: In
        values:
        - m5.xlarge
        - m5.2xlarge
```

### Exclude Control Plane

Prevent Mount Pods from running on control plane nodes:

```yaml
nodeAffinity: |
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: node-role.kubernetes.io/control-plane
        operator: DoesNotExist
```

## Monitoring

You can monitor DaemonSet Mount Pods using standard Kubernetes commands:

```bash
# List all mount DaemonSets
kubectl get daemonset -n kube-system | grep juicefs

# Check DaemonSet status
kubectl describe daemonset juicefs-<uniqueid>-mount-ds -n kube-system

# List pods created by DaemonSet
kubectl get pods -n kube-system -l juicefs.com/mount-by=juicefs-csi-driver
```

## Limitations

- Node affinity is only applied when `STORAGE_CLASS_SHARE_MOUNT` is enabled and DaemonSet mode is configured
- All PVCs using the same StorageClass share the same DaemonSet and node affinity rules
- Changing node affinity requires recreating the DaemonSet (happens automatically when all PVCs are deleted)

## Migration

To migrate from Pod-based mounts to DaemonSet mounts:

1. Enable the feature flags in CSI Driver
2. Create a new StorageClass with desired node affinity
3. Migrate PVCs to the new StorageClass
4. Old Mount Pods will be replaced by DaemonSet Pods

## Troubleshooting

### DaemonSet Pods not created

Check if nodes match the affinity rules:

```bash
kubectl get nodes --show-labels | grep <your-label>
```

### Mount Pods on unexpected nodes

Verify the nodeAffinity configuration:

```bash
kubectl get storageclass <name> -o yaml | grep -A 10 nodeAffinity
```

### References not cleaned up

Check DaemonSet annotations:

```bash
kubectl get daemonset <name> -n kube-system -o jsonpath='{.metadata.annotations}'
```
