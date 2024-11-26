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
  name: '名称',
  namespace: '命名空间',
  capacity: '容量',
  accessMode: '访问模式',
  status: '状态',
  createAt: '创建时间',
  pvcTablePageName: 'PVC 管理',
  pvcTableName: 'PVC 列表',
  inquiryForm: '查询表格',
  reclaimPolicy: '回收策略',
  pvTablePageName: 'PV 管理',
  pvTableName: 'PV 列表',
  appPodTablePageName: '应用 Pod 管理',
  appPodTableName: 'Pod 列表',
  appPodTableTip: '此列表只显示使用了 JuiceFS CSI 的 Pod',
  node: '节点',
  systemPodTablePageName: '系统 Pod 管理',
  sysPodTableName: '系统 Pod 列表',
  allowVolumeExpansion: '允许扩容',
  true: '是',
  false: '否',
  scTablePageName: 'StorageClass 管理',
  scTableName: 'StorageClass 列表',
  appPodTable: '应用 Pod',
  sysPodTable: '系统 Pod',
  refresh: '刷新',
  downloadLog: '下载完整日志',
  podNotFound: 'Pod 不存在',
  basic: '基础信息',
  clickToViewDetail: '点击查看详情',
  containerList: '容器列表',
  containerName: '容器名',
  restartCount: '重启次数',
  startAt: '启动时间',
  log: '日志',
  viewLog: '查看日志',
  event: '事件',
  pvNotFound: 'PV 不存在',
  mountOptions: '挂载参数',
  pvcNotFound: 'PVC 不存在',
  scNotFound: 'StorageClass 不存在',
  pvcUnboundErrMsg: '其使用的 PVC 未成功绑定，请点击「PVC」查看详情。',
  unScheduledMsg: '未调度成功，请点击 Pod 详情查看调度失败的具体原因。',
  nodeErrMsg: '所在节点异常，请检查节点状态。',
  containerErrMsg: '有容器启动异常，请点击 Pod 详情查看容器状态及日志。',
  mountPodTerminatingMsg:
    'Mount Pod 还在 Terminating 状态且存在 finalizer，请检查 CSI Node 日志',
  mountPodStickTerminatingMsg:
    'Mount Pod 卡在 Terminating 状态，请检查节点上是否存在未断开的 FUSE 请求',
  mountContainUidMsg: 'Mount Pod 还记录其 uid，请检查 CSI Node 日志',
  podFinalizerMsg: 'Pod 还存在 finalizer 未处理完，请查看 finalizer 状态',
  csiNodeNullMsg:
    '所在节点 CSI Node 未启动，请检查：1. 若是 sidecar 模式，请查看其所在 namespace 是否打上需要的 label 或查看 CSI Controller 日志以确认为何 sidecar 未注入；2. 若是 Mount Pod 模式，请检查 CSI Node DaemonSet 是否未调度到该节点上。',
  csiNodeErrMsg:
    '所在节点 CSI Node 未启动成功，请点击右方「CSI Node」查看其状态及日志。',
  mountPodNullMsg: 'Mount Pod 未启动，请点击右方「CSI Node」检查其日志。',
  mountPodErrMsg:
    'Mount Pod 未启动成功，请点击右方「Mount Pods」检查其状态及日志。',
  podErrMsg: 'pod 异常，请点击详情查看其 event 或日志。',
  pvNotCreatedMsg:
    '对应的 PV 未自动创建，请点击「系统 Pod」查看 CSI Controller 日志。',
  pvcSelectorErrMsg: '未设置 PVC 的 selector。',
  pvOfPVCNotFoundErrMsg:
    '未找到符合 PVC 条件的 PV，请点击「PV」查看其是否被创建。',
  pvcOfPVNotFoundErrMsg:
    '未找到符合 PV 条件的 PVC，请点击「PVC」查看其是否被创建。',
  waitingPVCDeleteMsg: '对应的 PVC 未被删除，请点击「PVC」查看详情。',
  waitingVolumeRecycleMsg: '等待管理员回收 volume',
  volumeRecycleFailedMsg: 'volume 回收失败',
  volumeAttributes: '卷属性',
  parameters: '参数',
  type: '类型',
  reason: '原因',
  from: '事件源',
  message: '信息',
  docs: '文档',
  save: '保存',
  config: '配置',
  batchUpgrade: '批量升级',
  upgrading: '升级中',
  recreate: '是否重建',
  start: '开始',
  upgrade: '升级',
  parallelNum: '并发数',
  ignoreError: '是否跳过失败任务',
  reset: '重置',
  apply: '立即生效',
  diffPods: '配置需要变更的 Mount Pod',
  noDiff: '暂无配置需要变更的 Mount Pod',
  selectPVC: '请选择 PVC',
  action: '操作',
  image: '镜像',
  resources: '资源',
  setting: '设置',
  tool: '工具',
}
