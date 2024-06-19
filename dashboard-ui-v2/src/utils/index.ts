/**
 * Copyright 2024 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  Node,
  PersistentVolume,
  PersistentVolumeClaim,
  Pod as RawPod,
} from 'kubernetes-types/core/v1'
import { ObjectMeta } from 'kubernetes-types/meta/v1'
import { omit } from 'lodash'

import { Pod } from '@/types/k8s'

export interface Source {
  metadata?: ObjectMeta
}

export const getNodeStatusBadge = (node: Node) => {
  const ready = node.status?.conditions?.find((condition) => {
    if (condition.type === 'Ready' && condition.status === 'True') {
      return true
    }
    return false
  })
  return ready ? 'green' : 'red'
}

export const getPodStatusBadge = (finalStatus: string) => {
  switch (finalStatus) {
    case 'Pending':
    case 'ContainerCreating':
    case 'PodInitializing':
      return 'yellow'
    case 'Running':
      return 'green'
    case 'Succeed':
      return 'blue'
    case 'Failed':
    case 'Error':
    case 'ImagePullBackOff':
      return 'red'
    case 'Unknown':
    case 'Terminating':
    default:
      return 'grey'
  }
}

export const getPVStatusBadge = (pv: PersistentVolume) => {
  if (pv.status === undefined || pv.status.phase === undefined) {
    return 'grey'
  }
  switch (pv.status.phase) {
    case 'Bound':
      return 'green'
    case 'Available':
      return 'blue'
    case 'Pending':
      return 'yellow'
    case 'Failed':
      return 'red'
    case 'Released':
    default:
      return 'grey'
  }
}

export const getPVCStatusBadge = (pvc: PersistentVolumeClaim) => {
  if (pvc.status === undefined || pvc.status.phase === undefined) {
    return 'grey'
  }
  switch (pvc.status.phase) {
    case 'Bound':
      return 'green'
    case 'Available':
      return 'blue'
    case 'Pending':
      return 'yellow'
    case 'Failed':
      return 'red'
    case 'Released':
    default:
      return 'grey'
  }
}

export const isPodReady = (pod: RawPod) => {
  let conditionTrue = 0
  pod.status?.conditions?.forEach((condition) => {
    if (
      (condition.type === 'ContainersReady' || condition.type === 'Ready') &&
      condition.status === 'True'
    ) {
      conditionTrue++
    }
  })
  return conditionTrue === 2
}

export const failedReasonOfPVC = (pvc: PersistentVolumeClaim) => {
  if (pvc.status?.phase === 'Bound') {
    return ''
  }
  if (pvc.spec?.storageClassName !== '') {
    return 'pvNotCreatedMsg'
  }
  if (pvc.spec.volumeName) {
    return 'pvOfPVCNotFoundErrMsg'
  }
  if (pvc.spec.selector === undefined) {
    return 'pvcSelectorErrMsg'
  }
  return 'pvOfPVCNotFoundErrMsg'
}

export const failedReasonOfPV = (pv: PersistentVolume) => {
  if (pv.status?.phase === 'Bound') {
    return ''
  }
  return 'pvcOfPVNotFoundErrMsg'
}

export const failedReasonOfAppPod = (pod: Pod) => {
  const { mountPods, pvcs, csiNode } = pod
  // check if pod is ready
  if (isPodReady(pod)) {
    return ''
  }

  let reason = ''
  // 1. PVC pending
  pvcs?.forEach((pvc) => {
    if (pvc.status?.phase !== 'Bound') {
      reason = 'pvcUnboundErrMsg'
    }
  })
  if (reason !== '') {
    return reason
  }

  // 2. not scheduled
  pod.status?.conditions?.forEach((condition) => {
    if (condition.type === 'PodScheduled' && condition.status !== 'True') {
      reason = 'unScheduledMsg'
      return
    }
  })
  if (reason !== '') {
    return reason
  }

  // 3. node not ready
  if (pod.node) {
    pod.node.status?.conditions?.forEach((condition) => {
      if (condition.type === 'Ready' && condition.status !== 'True') {
        reason = 'nodeErrMsg'
      }
    })
  }
  if (reason !== '') {
    return reason
  }

  // sidecar mode
  if (
    pod.metadata?.labels !== undefined &&
    pod.metadata?.labels['done.sidecar.juicefs.com/inject'] === 'true'
  ) {
    let reason = ''
    pod.status?.initContainerStatuses?.forEach((containerStatus) => {
      if (!containerStatus.ready) {
        reason = 'containerErrMsg'
      }
    })
    pod.status?.containerStatuses?.forEach((containerStatus) => {
      if (!containerStatus.ready) {
        reason = 'containerErrMsg'
      }
    })
    return reason
  }

  // mount pod mode
  // 4. check csi node
  if (!csiNode || csiNode === undefined) {
    return 'csiNodeNullMsg'
  }
  if (!isPodReady(csiNode)) {
    return 'csiNodeErrMsg'
  }
  // 5. check mount pod
  if (mountPods?.length === 0) {
    return 'mountPodNullMsg'
  }
  mountPods?.forEach((mountPod) => {
    if (!isPodReady(mountPod)) {
      reason = 'mountPodErrMsg'
      return
    }
  })
  if (reason !== '') {
    return reason
  }

  return 'podErrMsg'
}

export const podStatus = (pod: RawPod) => {
  if (pod.metadata?.deletionTimestamp) {
    return 'Terminating'
  }
  if (!pod.status) {
    return 'Unknown'
  }
  let status: string = ''
  pod.status?.containerStatuses?.forEach((containerStatus) => {
    if (!containerStatus.ready) {
      if (containerStatus.state?.waiting) {
        if (
          containerStatus.state.waiting.reason === 'ContainerCreating' ||
          containerStatus.state.waiting.reason === 'PodInitializing' ||
          containerStatus.state.waiting.reason === 'ImagePullBackOff'
        ) {
          status = containerStatus.state.waiting.reason
          return
        }
        if (containerStatus.state.waiting.message) {
          status = 'Error'
          return
        }
      }
      if (
        containerStatus.state?.terminated &&
        containerStatus.state.terminated.message
      ) {
        status = 'Error'
        return
      }
    }
  })
  if (status !== '') {
    return status
  }
  return pod.status.phase
}

export const omitPod = (pod: Pod) => {
  return omit(pod, [
    'metadata.managedFields',
    'pvs',
    'pvcs',
    'csiNode',
    'mountPods',
    'node',
  ])
}
