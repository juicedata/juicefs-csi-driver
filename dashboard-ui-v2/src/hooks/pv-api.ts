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

import { StorageClass } from 'kubernetes-types/storage/v1'
import useSWR from 'swr'

import { PVCPagingListArgs, PVPagingListArgs, SCPagingListArgs } from '@/types'
import { PV, PVC } from '@/types/k8s.ts'

export function useSCs(args: SCPagingListArgs) {
  const order = args.sort?.['time'] || 'ascend'
  const name = args.name || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1

  return useSWR<{
    scs: [StorageClass]
    total: number
  }>(
    `/api/v1/storageclasses?name=${name}&order=${order}&pageSize=${pageSize}&current=${current}`,
  )
}

export function usePVs(args: PVPagingListArgs) {
  const order = args.sort['time'] || 'descend'
  const name = args.name || ''
  const pvc = args.pvc || ''
  const sc = args.sc || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1

  return useSWR<{
    pvs: [PV]
    total: number
  }>(
    `/api/v1/pvs?order=${order}&name=${name}&pvc=${pvc}&sc=${sc}&pageSize=${pageSize}&current=${current}`,
  )
}

export function usePVCs(args: PVCPagingListArgs) {
  const order = args.sort['time'] || 'descend'
  const namespace = args.namespace || ''
  const name = args.name || ''
  const pv = args.pv || ''
  const sc = args.sc || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1

  return useSWR<{
    pvcs: [PVC]
    total: number
  }>(
    `/api/v1/pvcs?order=${order}&namespace=${namespace}&name=${name}&pv=${pv}&sc=${sc}&pageSize=${pageSize}&current=${current}`,
  )
}
