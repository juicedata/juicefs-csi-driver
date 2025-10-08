# Mount Pod Configuration Guide

JuiceFS CSI Driver provides flexible Mount Pod deployment options that can be configured per StorageClass. This allows you to optimize resource usage and performance based on your specific needs.

## Overview

JuiceFS CSI Driver supports three Mount Pod deployment modes:

1. **Per-PVC Mode** (`per-pvc`): Creates a separate Mount Pod for each PVC
2. **Shared Pod Mode** (`shared-pod`): Shares Mount Pods across PVCs using the same StorageClass
3. **DaemonSet Mode** (`daemonset`): Deploys Mount Pods as DaemonSets with node affinity support

## Configuration Methods

### Global Defaults (Environment Variables)

Set default behavior for all StorageClasses via environment variables in the CSI Driver:

```yaml
env:
  # Enable mount sharing (defaults to shared-pod mode)
  - name: STORAGE_CLASS_SHARE_MOUNT
    value: "true"
```

When `STORAGE_CLASS_SHARE_MOUNT` is enabled:
- Default mode is `shared-pod` 
- DaemonSet mode can be configured per StorageClass via ConfigMap
- Per-PVC mode can still be explicitly configured via ConfigMap

### Per-StorageClass Configuration (ConfigMap)

Override global defaults and configure individual StorageClasses using a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: juicefs-mount-config
  namespace: kube-system
data:
  # Default configuration for all StorageClasses
  default: |
    mode: shared-pod
  
  # Configuration for specific StorageClass
  my-storage-class: |
    mode: daemonset
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: juicefs/mount-node
            operator: In
            values:
            - "true"
```

## Automatic Fallback Behavior

When using DaemonSet mode, the CSI Driver automatically falls back to shared Pod mode if:

1. The DaemonSet cannot schedule a Pod on the node due to nodeAffinity restrictions
2. The node has taints that prevent the DaemonSet Pod from being scheduled
3. The DaemonSet Pod fails to become ready within the timeout period

This ensures that workloads can still mount volumes even if the DaemonSet configuration prevents Pods from running on certain nodes.

## Mount Modes Explained

### Per-PVC Mode

Each PVC gets its own dedicated Mount Pod.

**Advantages:**

- Complete isolation between PVCs
- Simple troubleshooting
- Independent lifecycle management

**Use Cases:**

- Development environments
- Multi-tenant scenarios requiring strict isolation
- Applications with specific mount configurations

**Configuration:**

```yaml
mode: per-pvc
```

### Shared Pod Mode

Multiple PVCs using the same StorageClass share Mount Pods.

**Advantages:**

- Reduced resource consumption
- Fewer Pods to manage
- Shared cache benefits

**Use Cases:**

- Production environments with many PVCs
- Clusters with resource constraints
- Applications with similar access patterns

**Configuration:**

```yaml
mode: shared-pod
```

### DaemonSet Mode

Mount Pods are deployed as DaemonSets across selected nodes.

**Advantages:**

- Predictable Pod placement
- Node-level resource optimization
- Automatic scaling with node additions
- Centralized node affinity control

**Use Cases:**

- High-performance computing
- Dedicated Mount nodes
- GPU workloads
- Large-scale deployments

**Configuration:**

```yaml
mode: daemonset
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: node-type
        operator: In
        values:
        - compute
```

## Configuration Priority

The system determines mount mode in this order:

1. **ConfigMap with StorageClass name** - Highest priority
2. **ConfigMap default configuration**
3. **Global environment variables** - Lowest priority

## Working with Existing StorageClasses

The ConfigMap approach allows you to change mount behavior **without modifying existing StorageClasses**:

1. Create the ConfigMap with desired configuration
2. New PVCs will use the new mount mode
3. Existing PVCs continue using their current Mount Pods

## Examples

### Example 1: Mixed Mode Deployment

Different StorageClasses use different modes:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: juicefs-mount-config
  namespace: kube-system
data:
  # Default: shared pods for most workloads
  default: |
    mode: shared-pod
  
  # High-performance workloads use DaemonSet
  high-performance-sc: |
    mode: daemonset
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: workload-type
            operator: In
            values:
            - compute
  
  # Development uses per-PVC for isolation
  development-sc: |
    mode: per-pvc
```

### Example 2: Gradual Migration

Migrate from per-PVC to shared/DaemonSet mode:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: juicefs-mount-config
  namespace: kube-system
data:
  # Keep existing behavior for old StorageClasses
  default: |
    mode: per-pvc
  
  # New StorageClasses use shared mode
  juicefs-sc-v2: |
    mode: shared-pod
  
  # Critical workloads use DaemonSet
  juicefs-sc-critical: |
    mode: daemonset
```

### Example 3: Node-Specific DaemonSets

Deploy Mount Pods on specific node types:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: juicefs-mount-config
  namespace: kube-system
data:
  # GPU workloads
  gpu-storage: |
    mode: daemonset
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: nvidia.com/gpu
            operator: Exists
  
  # CPU-intensive workloads
  cpu-storage: |
    mode: daemonset
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values:
            - c5.xlarge
            - c5.2xlarge
```

## Monitoring and Management

### List Mount Pods by Type

```bash
# Per-PVC pods
kubectl get pods -n kube-system -l juicefs.com/mount-by=juicefs-csi-driver | grep -v "juicefs-.*-mount-ds"

# Shared pods (look for pods with multiple PVC references)
kubectl get pods -n kube-system -l juicefs.com/mount-by=juicefs-csi-driver -o json | \
  jq '.items[] | select(.metadata.annotations | length > 1) | .metadata.name'

# DaemonSet pods
kubectl get daemonset -n kube-system | grep juicefs
kubectl get pods -n kube-system -l juicefs.com/mount-by=juicefs-csi-driver | grep "juicefs-.*-mount-ds"
```

### Check Configuration

```bash
# View current ConfigMap
kubectl get configmap juicefs-mount-config -n kube-system -o yaml

# Check which mode a StorageClass is using
kubectl get configmap juicefs-mount-config -n kube-system -o jsonpath='{.data.my-storage-class}'
```

## Best Practices

1. **Start with shared-Pod mode** for most workloads
2. **Use DaemonSet mode** for:
   - High-performance requirements
   - Predictable resource allocation
   - Node-specific optimizations
3. **Use per-PVC mode** for:
   - Development/testing
   - Strict isolation requirements
   - Troubleshooting
4. **Test configuration changes** in non-production first
5. **Monitor resource usage** after mode changes
6. **Document your configuration** choices

## Troubleshooting

### Mount Pods not created as expected

1. Check ConfigMap exists and is valid:

```bash
kubectl get configmap juicefs-mount-config -n kube-system
```

2. Verify CSI Driver can read ConfigMap:

```bash
kubectl logs -n kube-system daemonset/juicefs-csi-node | grep "mount-config"
```

3. Check for syntax errors in ConfigMap:

```bash
kubectl get configmap juicefs-mount-config -n kube-system -o yaml | \
  yq eval '.data.default' - | kubectl create --dry-run=client -f -
```

### DaemonSet Pods not scheduled

1. Verify node affinity matches existing nodes:

```bash
kubectl get nodes --show-labels
```

2. Check DaemonSet status:

```bash
kubectl describe daemonset -n kube-system juicefs-<uniqueid>-mount-ds
```

### Switching modes for existing PVCs

Existing PVCs continue using their current Mount Pods. To switch modes:

1. Update ConfigMap with new configuration
2. Delete existing PVCs (ensure data is backed up)
3. Recreate PVCs to use new mode

## Migration Guide

### From Environment Variables to ConfigMap

1. Create ConfigMap with current behavior:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: juicefs-mount-config
  namespace: kube-system
data:
  default: |
    mode: shared-pod  # or per-pvc/daemonset based on current env vars
```

2. Deploy ConfigMap:

```bash
kubectl apply -f juicefs-mount-config.yaml
```

3. New PVCs will use ConfigMap configuration
4. Optionally remove environment variables from CSI Driver

### From Per-PVC to Shared/DaemonSet

1. Update ConfigMap for specific StorageClasses
2. New PVCs use new mode automatically
3. Optionally migrate existing PVCs during maintenance Windows

## Summary

The Mount Pod configuration system provides:

- **Flexibility**: Different modes for different workloads
- **Compatibility**: Works with existing StorageClasses
- **Simplicity**: Centralized configuration via ConfigMap
- **Power**: Fine-grained control with node affinity
- **Safety**: Non-disruptive to existing workloads

Choose the appropriate mode based on your workload requirements, resource constraints, and operational preferences.
