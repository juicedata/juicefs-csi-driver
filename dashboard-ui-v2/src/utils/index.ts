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
import { StorageClass } from 'kubernetes-types/storage/v1'
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
    case 'CrashLoopBackOff':
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
  if (pv.metadata?.deletionTimestamp) {
    if (pv.status?.phase === 'Bound') {
      return 'waitingPVCDeleteMsg'
    }
    return ''
  }

  if (pv.status?.phase === 'Bound' || pv.status?.phase === 'Pending') {
    return ''
  }

  if (pv.status?.phase === 'Available') {
    return 'pvcOfPVNotFoundErrMsg'
  }

  if (pv.status?.phase === 'Released') {
    return 'waitingVolumeRecycleMsg'
  }

  if (pv.status?.phase === 'Failed') {
    return 'volumeRecycleFailedMsg'
  }
  return ''
}

export const failedReasonOfAppPod = (pod: Pod) => {
  if (pod.metadata?.deletionTimestamp) {
    return failedReasonOfTerminatingAppPod(pod)
  }
  return failedReasonOfRunningAppPod(pod)
}

export const failedReasonOfRunningAppPod = (pod: Pod) => {
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
  if (!csiNode) {
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

export const failedReasonOfTerminatingAppPod = (pod: Pod) => {
  const { mountPods, csiNode, node } = pod
  //  1. node not ready
  if (node === undefined || node.status?.phase === 'Ready') {
    return 'nodeErrMsg'
  }

  // sidecar mode do not need
  if (
    pod.metadata?.labels === undefined ||
    pod.metadata?.labels['done.sidecar.juicefs.com/inject'] !== 'true'
  ) {
    // 2. csi node not ready
    if (!csiNode) {
      return 'csiNodeNullMsg'
    }
    if (!isPodReady(csiNode)) {
      return 'csiNodeErrMsg'
    }

    // 3. mount pod not terminating or contain pod uid
    let reason = ''
    mountPods?.forEach((mountPod) => {
      if (!mountPod.metadata?.deletionTimestamp) {
        if (mountPod.metadata?.finalizers) {
          reason = 'mountPodTerminatingMsg'
        } else {
          reason = 'mountPodStickTerminatingMsg'
        }
      } else {
        for (const anno in mountPod.metadata.annotations) {
          if (anno.includes(pod.metadata?.uid || '')) {
            reason = 'mountContainUidMsg'
          }
        }
      }
    })

    if (reason !== '') {
      return reason
    }
  }

  // 4. container error
  const reason = containerErrMsg(pod)
  if (reason !== '') {
    return reason
  }

  // 5. finalizer not delete
  if (pod.metadata?.finalizers) {
    return 'podFinalizerMsg'
  }
}

export const containerErrMsg = (pod: Pod) => {
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

export const failedReasonOfMountPod = (pod: Pod) => {
  if (pod.metadata?.deletionTimestamp) {
    return failedReasonOfTerminatingMountPod(pod)
  }
  return failedReasonOfRunningMountPod(pod)
}

export const failedReasonOfRunningMountPod = (pod: Pod) => {
  const { csiNode } = pod
  // check if pod is ready
  if (isPodReady(pod)) {
    return ''
  }

  let reason = ''

  // 1. node not ready
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

  // 2. check csi node
  if (!csiNode) {
    return 'csiNodeNullMsg'
  }
  if (!isPodReady(csiNode)) {
    return 'csiNodeErrMsg'
  }

  // 3. check container error
  reason = containerErrMsg(pod)
  if (reason !== '') {
    return reason
  }

  return 'podErrMsg'
}

export const failedReasonOfTerminatingMountPod = (pod: Pod) => {
  const { csiNode, node } = pod
  //  1. node not ready
  if (node === undefined || node.status?.phase === 'Ready') {
    return 'nodeErrMsg'
  }

  // 2. csi node not ready
  if (!csiNode) {
    return 'csiNodeNullMsg'
  }
  if (!isPodReady(csiNode)) {
    return 'csiNodeErrMsg'
  }

  // 3. container error
  const reason = containerErrMsg(pod)
  if (reason !== '') {
    return reason
  }

  // 4. finalizer not delete
  if (pod.metadata?.finalizers) {
    return 'mountPodTerminatingMsg'
  }

  // 5. finalizer deleted
  return 'mountPodStickTerminatingMsg'
}

// podStatus: copy from kubernetes/pkg/printers/internalversion/printers.go, which `kubectl get po` used.
export const podStatus = (pod: RawPod) => {
  let reason = pod.status?.phase
  if (pod.status?.reason) {
    reason = pod.status.reason
  }

  let initializing = false
  if (pod.status?.initContainerStatuses) {
    for (let i = 0; i < (pod.status?.initContainerStatuses?.length || 0); i++) {
      const container = pod.status?.initContainerStatuses[i]
      if (
        container?.state?.terminated &&
        container.state.terminated.exitCode === 0
      ) {
        continue
      }
      if (container.state?.terminated) {
        // initialization is failed
        if (container.state.terminated.reason?.length === 0) {
          if (container.state.terminated.signal !== 0) {
            reason = 'Init:Signal:' + container.state.terminated.signal
          } else {
            reason = 'Init:ExitCode:' + container.state.terminated.exitCode
          }
        } else {
          reason = 'Init:' + container.state.terminated.reason
        }
        initializing = true
        continue
      }
      if (
        container.state?.waiting &&
        (container.state.waiting.reason?.length || 0) > 0 &&
        container.state.waiting.reason !== 'PodInitializing'
      ) {
        reason = 'Init:' + container.state.waiting.reason
        initializing = true
        continue
      }
      reason = 'Init:' + i + '/' + pod.spec?.initContainers?.length
      initializing = true
    }
  }

  if (!initializing) {
    let hasRunning = false
    if (pod.status?.containerStatuses) {
      for (let i = pod.status.containerStatuses.length - 1; i >= 0; i--) {
        const container = pod.status.containerStatuses[i]

        if (container.state?.waiting && container.state.waiting.reason !== '') {
          reason = container.state.waiting.reason
        } else if (
          container.state?.terminated &&
          container.state.terminated.reason !== ''
        ) {
          reason = container.state.terminated.reason
        } else if (
          container.state?.terminated &&
          container.state.terminated.reason === ''
        ) {
          if (container.state.terminated.signal !== 0) {
            reason = 'Signal:' + container.state.terminated.signal
          } else {
            reason = 'ExitCode:' + container.state.terminated.exitCode
          }
        } else if (container.ready && container.state?.running) {
          hasRunning = true
        }
      }

      // change pod status back to "Running" if there is at least one container still reporting as "Running" status
      if (reason == 'Completed' && hasRunning) {
        if (hasPodReadyCondition(pod)) {
          reason = 'Running'
        } else {
          reason = 'NotReady'
        }
      }
    }
  }

  if (pod.metadata?.deletionTimestamp && pod.status?.reason === 'NodeLost') {
    reason = 'Unknown'
  } else if (pod.metadata?.deletionTimestamp) {
    reason = 'Terminating'
  }
  return reason
}

export const hasPodReadyCondition = (pod: RawPod) => {
  let hasReady = false
  pod.status?.conditions?.forEach((condition) => {
    if (condition.type === 'Ready' && condition.status === 'True') {
      hasReady = true
      return
    }
  })
  return hasReady
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

export const scParameter = (sc: StorageClass) => {
  const parameters: string[] = []
  for (const key in sc.parameters) {
    if (Object.prototype.hasOwnProperty.call(sc.parameters, key)) {
      const value = sc.parameters[key]
      parameters.push(`${key}: ${value}`)
    }
  }
  return parameters
}

export function getBasePath() {
  const domain = window.location.pathname.split('/')
  let base = ''
  if (
    !['', 'pods', 'syspods', 'pvcs', 'pvs', 'storageclass', 'config'].includes(
      domain[1],
    )
  ) {
    base = `/${domain[1]}`
  }
  return base
}

export function getHost(): string {
  const protocol = window.location.protocol === 'https:' ? 'https' : 'http'
  const host = import.meta.env.VITE_HOST ?? window.location.host
  return `${protocol}://${host}`
}

export function isMountPod(pod: Pod): boolean {
  return (
    (pod.metadata?.name?.startsWith('juicefs-') &&
      pod.metadata?.labels?.['app.kubernetes.io/name'] === 'juicefs-mount') ||
    false
  )
}
