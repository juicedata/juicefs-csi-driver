/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

export default {
  name: 'Name',
  namespace: 'Namespace',
  capacity: 'Capacity',
  accessMode: 'AccessMode',
  status: 'Status',
  createAt: 'CreateTime',
  pvcTablePageName: 'PersistentVolumeClaims',
  pvcTableName: 'PVC List',
  inquiryForm: 'Inquiry Form',
  reclaimPolicy: 'ReclaimPolicy',
  pvTablePageName: 'PersistentVolumes',
  pvTableName: 'PV List',
  appPodTablePageName: 'Application Pod',
  appPodTableName: 'Application Pod List',
  appPodTableTip: 'This table only shows pods that use JuiceFS CSI',
  node: 'Node',
  systemPodTablePageName: 'System Pod',
  sysPodTableName: 'System Pod List',
  allowVolumeExpansion: 'AllowVolumeExpansion',
  true: 'True',
  false: 'False',
  scTablePageName: 'StorageClass',
  scTableName: 'StorageClass List',
  appPodTable: 'Application Pod',
  sysPodTable: 'System Pod',
  refresh: ' refresh',
  downloadLog: ' download',
  podNotFound: 'Pod not found',
  basic: 'Basic Information',
  clickToView: 'Click to view',
  containerList: 'Container List',
  containerName: 'Container Name',
  restartCount: 'Restart Count',
  startAt: 'Start Time',
  log: 'Log',
  viewLog: 'Log View',
  event: 'Event',
  pvNotFound: 'PV not found',
  mountOptions: 'Mount Options',
  pvcNotFound: 'PVC not found',
  scNotFound: 'StorageClass not found',
  pvcUnboundErrMsg:
    'PVC which it uses was not successfully bound, please click "PVC" to view details.',
  unScheduledMsg:
    'The Pod was not scheduled successfully. Please click Pod details to view the specific reasons for the scheduling failure.',
  nodeErrMsg: 'The node is abnormal, please check the node status.',
  containerErrMsg:
    'Some container is abnormal. Please click Pod details to view the container status and logs.',
  mountPodTerminatingMsg:
    'Mount Pod is in terminating and has finalizer, please check the CSI Node logs',
  mountPodStickTerminatingMsg:
    'Mount Pod is stuck in Terminating, please check whether there is an undisconnected FUSE request on the node',
  mountContainUidMsg:
    'Mount Pod still contain its uid，please check logs of CSI Node',
  podFinalizerMsg:
    "There are still finalizers in the Pod. Please check the finalizer's status.",
  csiNodeNullMsg:
    'CSI Node in the node did not start, please check: 1. If it is in sidecar mode, please check whether the namespace has set the required label or check the CSI Controller log to confirm why the sidecar is not injected; 2. If it is in Mount Pod mode, please check Whether CSI Node DaemonSet has been scheduled to this node.',
  csiNodeErrMsg:
    'CSI Node in the node is not ready, please click "CSI Node" on the right to view its status and logs.',
  mountPodNullMsg:
    'Mount Pod did not start, please click "CSI Node" on the right to check its log.',
  mountPodErrMsg:
    'Mount Pod is not ready. Please click "Mount Pods" on the right to check its status and logs.',
  podErrMsg:
    'The pod is abnormal, please click details to view its event or log.',
  pvNotCreatedMsg:
    'The corresponding PV is not automatically created. Please click "System Pod" to view the log of CSI Controller.',
  pvcSelectorErrMsg: 'PVC selector is not set.',
  pvOfPVCNotFoundErrMsg:
    'No matching PV was found. Please click "PV" to check whether it has been created.',
  pvcOfPVNotFoundErrMsg:
    'No matching PVC was found. Please click "PVC" to check whether it has been created.',
  waitingPVCDeleteMsg:
    'Waiting for matching PVC to be deleted. Please click "PVC" for detail.',
  waitingVolumeRecycleMsg: 'Waiting for volumes to be recycled.',
  volumeRecycleFailedMsg: 'the volumes were failed to be recycled.',
  volumeAttributes: 'Volume Attributes',
  parameters: 'Parameters',
  type: 'Type',
  reason: 'Reason',
  from: 'From',
  message: 'Message',
  action: 'Action',
  docs: 'Documents',
  save: 'Save',
  config: 'Configs',
  batchUpgrade: 'Batch Upgrade',
  upgrading: 'Upgrading',
  start: 'Start',
  recreate: 'Recreate',
  upgrade: 'Upgrade',
  reset: 'Reset',
  image: 'Image',
  resources: 'Resources',
  setting: 'Setting',
  tool: 'Tools',
}
