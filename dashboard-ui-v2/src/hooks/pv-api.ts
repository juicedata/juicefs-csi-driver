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

import useSWR from 'swr'

import {SCPagingListArgs} from '@/types'
import {StorageClass} from 'kubernetes-types/storage/v1'

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
