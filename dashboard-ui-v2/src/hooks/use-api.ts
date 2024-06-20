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

import { Event } from 'kubernetes-types/core/v1'
import useWebSocket, { Options } from 'react-use-websocket'
import useSWR from 'swr'

import { AppPagingListArgs, SysPagingListArgs } from '@/types'
import { Pod } from '@/types/k8s'
import { getBasePath } from '@/utils'

export function useAppPods(args: AppPagingListArgs) {
  const order = args.sort?.['time'] || 'descend'
  const namespace = args.namespace || ''
  const name = args.name || ''
  const pv = args.pv || ''
  const csiNode = args.csiNode || ''
  const mountPod = args.mountPod || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1

  return useSWR<{
    pods: Pod[]
    total: number
  }>(
    `/api/v1/pods?order=${order}&namespace=${namespace}&name=${name}&pv=${pv}&mountpod=${mountPod}&csinode=${csiNode}&pageSize=${pageSize}&current=${current}`,
  )
}

export function useSysAppPods(args: SysPagingListArgs) {
  const order = args.sort?.['time'] || 'ascend'
  const namespace = args.namespace || ''
  const name = args.name || ''
  const node = args.node || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1

  return useSWR<{
    pods: Pod[]
    total: number
  }>(
    `/api/v1/syspods?namespace=${namespace}&name=${name}&node=${node}&order=${order}&pageSize=${pageSize}&current=${current}`,
  )
}

export function useAppPod(namespace?: string, name?: string) {
  return useSWR<Pod>(`/api/v1/pod/${namespace}/${name}/`)
}

export function usePodEvents(namespace?: string, name?: string) {
  return useSWR<Event[]>(`/api/v1/pod/${namespace}/${name}/events`)
}

export function usePods(
  namespace?: string,
  name?: string,
  type: 'mountpods' | 'apppods' = 'apppods',
) {
  return useSWR<Pod[]>(`/api/v1/pod/${namespace}/${name}/${type}`)
}

export function useWebsocket(
  uri?: string,
  opts?: Options,
  shouldConnect: boolean = false,
) {
  return useWebSocket(
    `ws://${import.meta.env.VITE_HOST ?? window.location.host}${getBasePath()}${uri}`,
    opts,
    shouldConnect,
  )
}
