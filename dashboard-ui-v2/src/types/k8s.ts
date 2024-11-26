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
  Pod as NativePod,
  Node,
  PersistentVolume,
  PersistentVolumeClaim,
} from 'kubernetes-types/core/v1'

export type Pod = {
  mountPods?: NativePod[]
  node: Node
  pvcs: PersistentVolumeClaim[]
  pvs: PersistentVolume[]
  csiNode: NativePod
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
  node: string,
  pods: NativePod[],
}
