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
import { SortOrder } from 'antd/es/table/interface'

export type Params = {
  resources:
    | 'pods'
    | 'mountpods'
    | 'syspods'
    | 'pvs'
    | 'pvcs'
    | 'storageclass'
    | 'config'
}

export type DetailParams = {
  namespace: string
  name: string
} & Params

export interface AppPagingListArgs {
  pageSize?: number
  current?: number
  namespace?: string
  name?: string
  pv?: string
  mountPod?: string
  csiNode?: string
  sort?: Record<string, SortOrder>
  filter?: Record<string, (string | number)[] | null>
}

export interface SysPagingListArgs {
  pageSize?: number
  current?: number
  namespace?: string
  name?: string
  node?: string
  sort?: Record<string, SortOrder>
  filter?: Record<string, (string | number)[] | null>
}

export interface SCPagingListArgs {
  pageSize?: number
  current?: number
  name?: string
  sort?: Record<string, SortOrder>
}

export interface PVPagingListArgs {
  pageSize?: number
  current?: number
  name?: string
  pvc?: string
  sc?: string
  sort?: Record<string, SortOrder>
  filter?: Record<string, (string | number)[] | null>
}

export interface PVCPagingListArgs {
  pageSize?: number
  current?: number
  namespace?: string
  name?: string
  pv?: string
  sc?: string
  sort?: Record<string, SortOrder>
  filter?: Record<string, (string | number)[] | null>
}
