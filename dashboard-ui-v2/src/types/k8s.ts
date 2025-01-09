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

import { Job } from 'kubernetes-types/batch/v1'
import {
  Container,
  EnvVar,
  Lifecycle,
  Pod as NativePod,
  Node,
  PersistentVolume,
  PersistentVolumeClaim,
  PodSpec,
  Probe,
  ResourceRequirements,
  Volume,
  VolumeDevice,
  VolumeMount,
} from 'kubernetes-types/core/v1'
import { ObjectMeta } from 'kubernetes-types/meta/v1'

export type Pod = {
  mountPods?: NativePod[]
  node?: Node
  pvcs?: PersistentVolumeClaim[]
  pvs?: PersistentVolume[]
  csiNode?: NativePod
} & NativePod

export type PV = PersistentVolume & {
  Pod: {
    namespace: string
    name: string
  }
}

export type PVC = PersistentVolumeClaim & {
  Pod: {
    namespace: string
    name: string
  }
}

export type PVCWithUniqueId = {
  PVC: PersistentVolumeClaim
  PV: PersistentVolume
  UniqueId: string
}

export const accessModeMap: { [key: string]: string } = {
  ReadWriteOnce: 'RWO',
  ReadWriteMany: 'RWX',
  ReadOnlyMany: 'ROX',
  ReadWriteOncePod: 'RWOP',
}

export type PodToUpgrade = {
  node: string
  pods: NativePod[]
}

export type BatchConfig = {
  parallel: number
  ignoreError: boolean
  noRecreate: boolean
  node: string
  uniqueId: string
  batches: MountPodUpgrade[][]
  status: string
}

export type MountPodUpgrade = {
  name: string
  node: string
  csiNodePod: string
  status: string
}

export type PodDiffConfig = {
  pod: Pod
  oldConfig: MountPatch
  newConfig: MountPatch
}

export type MountPatch = {
  ceMountImage: string
  eeMountImage: string
  cacheDirs: MountPatchCacheDir[]
  labels: { string: string }
  annotations: { string: string }
  hostNetwork: boolean
  hostPID: boolean
  livenessProbe: Probe
  readinessProbe: Probe
  startupProbe: Probe
  lifecycle: Lifecycle
  resources: ResourceRequirements
  terminationGracePeriodSeconds: number
  volumes: Volume[]
  volumeDevices: VolumeDevice[]
  volumeMounts: VolumeMount[]
  env: EnvVar[]
  mountOptions: string[]
}

export type MountPatchCacheDir = {
  type: string
  path: string
  name: string
}

export type UpgradeJob = {
  job: Job
  config: BatchConfig
}

export type UpgradeJobWithDiff = {
  job: Job
  total: number
  config: BatchConfig
  diffs: [PodDiffConfig]
}

export type CacheGroupTemplate = Omit<PodSpec, 'metadata' | 'containers'> &
  Omit<Container, 'name'> & {
  opts?: string[]
}

export type CacheGroup = {
  metadata?: ObjectMeta
  spec: {
    updateStrategy: {
      type: 'RollingUpdate' | 'OnDelete'
      rollingUpdate: {
        maxUnavailable: number
      }
    }
    secretRef: {
      name: string
    }
    worker: {
      template: CacheGroupTemplate
      overwrite: (CacheGroupTemplate & { nodes: string[] })[]
    }
  }
  status?: {
    phase: string
    readyWorker?: number
    expectWorker?: number
    readyStr?: string
    cacheGroup?: string
  }
}
