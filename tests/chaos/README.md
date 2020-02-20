# chaos

```sh
kustomize build | kubectl apply -f -
stern -n kube-system -l juicefs-csi-driver=master --exclude-container liveness-probe
```

## App pod failure

JuiceFS is mounted for recreated pod and unmounted for failed pod.

```s
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.302442       1 node.go:173] NodeGetCapabilities: called with args
juicefs-csi-controller-0 csi-attacher I0220 12:08:01.311768       1 reflector.go:370] k8s.io/client-go/informers/factory.go:133: Watch close - *v1beta1.VolumeAttachment total 0 items received
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.305342       1 node.go:173] NodeGetCapabilities: called with args
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.541602       1 node.go:75] NodePublishVolume: volume_id is aws-us-east-1
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.541669       1 node.go:86] NodePublishVolume: volume_capability is mount:<fs_type:"juicefs" > access_mode:<mode:MULTI_NODE_MULTI_WRITER >
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.541736       1 node.go:92] NodePublishVolume: creating dir /var/lib/kubelet/pods/e0cb2ab2-52ca-49c7-9166-bc365b87936f/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.541769       1 node.go:121] NodePublishVolume: mounting juicefs with secret [token accesskey name secretkey], options []
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:01.541792       1 juicefs.go:189] AuthFs: cmd "/usr/bin/juicefs", args []string{"auth", "aws-us-east-1", "--accesskey=AKIA4DLXAUBG2HI332KE", "--token=[secret]", "--secretkey=[secret]"}
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:05.418943       1 juicefs.go:111] MountFs: authentication output is ''
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:05.999289       1 juicefs.go:225] Mount: skip mounting for existing mount point "/jfs/aws-us-east-1"
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:05.999349       1 juicefs.go:65] CreateVol: checking "/jfs/aws-us-east-1" exists in &{0xc00009a6e0 aws-us-east-1 /jfs/aws-us-east-1 []}
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:05.999382       1 node.go:132] NodePublishVolume: binding /jfs/aws-us-east-1 at /var/lib/kubelet/pods/e0cb2ab2-52ca-49c7-9166-bc365b87936f/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount with options []
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:05.999457       1 mount_linux.go:146] Mounting cmd (mount) with arguments ([-t none -o bind /jfs/aws-us-east-1 /var/lib/kubelet/pods/e0cb2ab2-52ca-49c7-9166-bc365b87936f/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount])
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:06.009745       1 mount_linux.go:146] Mounting cmd (mount) with arguments ([-t none -o bind,remount /jfs/aws-us-east-1 /var/lib/kubelet/pods/e0cb2ab2-52ca-49c7-9166-bc365b87936f/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount])
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:06.011954       1 node.go:138] NodePublishVolume: mounted aws-us-east-1 at /var/lib/kubelet/pods/e0cb2ab2-52ca-49c7-9166-bc365b87936f/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount with options []
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:06.027795       1 node.go:144] NodeUnpublishVolume: called with args volume_id:"aws-us-east-1" target_path:"/var/lib/kubelet/pods/f281c7ff-0493-4017-bbc7-94c255c66b6e/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount"
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:06.028558       1 node.go:153] NodeUnpublishVolume: unmounting /var/lib/kubelet/pods/f281c7ff-0493-4017-bbc7-94c255c66b6e/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:06.028575       1 mount_linux.go:211] Unmounting /var/lib/kubelet/pods/f281c7ff-0493-4017-bbc7-94c255c66b6e/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount
juicefs-csi-node-bgxlm juicefs-plugin I0220 12:08:06.401950       1 node.go:158] NodeUnpublishVolume: unmounting ref for target /var/lib/kubelet/pods/f281c7ff-0493-4017-bbc7-94c255c66b6e/volumes/kubernetes.io~csi/juicefs-aws-us-east-1-rwx/mount
```
