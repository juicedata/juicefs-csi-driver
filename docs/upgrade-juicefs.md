# Upgrade juicefs binary

In the case we only need to upgrade the JuiceFS binary, please follow these steps:

1. Use this script to replace the `juicefs` binary in `juicefs-csi-node` pod with the new built one: 

```bash
#!/bin/bash
KUBECTL=/path/to/kubectl
JUICEFS_BIN=/path/to/new/juicefs

$KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
    xargs -L 1 -P 10 -I'{}' \
    $KUBECTL -n kube-system cp $JUICEFS_BIN '{}':/tmp/juicefs -c juicefs-plugin

$KUBECTL -n kube-system get pods | grep juicefs-csi-node | awk '{print $1}' | \
    xargs -L 1 -P 10 -I'{}' \
    $KUBECTL -n kube-system exec -i '{}' -c juicefs-plugin -- \
    chmod a+x /tmp/juicefs && mv /tmp/juicefs /bin/juicefs
```
Replace `/path/to/kubectl` and `/path/to/new/juicefs` to your environment path and execute this script.

2. Restart the applications one by one, or kill the existing pods.
