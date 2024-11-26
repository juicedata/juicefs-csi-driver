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

import { useAsync } from '@react-hookz/web'
import { Event } from 'kubernetes-types/core/v1'
import useWebSocket, { Options } from 'react-use-websocket'
import useSWR from 'swr'

import { AppPagingListArgs, SysPagingListArgs } from '@/types'
import { Pod, PodToUpgrade } from '@/types/k8s'
import { Node } from 'kubernetes-types/core/v1'
import { getBasePath, getHost } from '@/utils'
import { Job } from 'kubernetes-types/batch/v1'

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

export function useMountPodImage(isMountPod: boolean, namespace?: string, name?: string) {
  return useSWR<string>(isMountPod ? `/api/v1/pod/${namespace}/${name}/latestimage` : '')
}

export function useAppPod(namespace?: string, name?: string) {
  return useSWR<Pod>(`/api/v1/pod/${namespace}/${name}/`)
}

export function useEvents(
  source: 'pod' | 'pv' | 'pvc' = 'pod',
  namespace?: string,
  name?: string,
) {
  return useSWR<Event[]>(
    source === 'pv'
      ? `/api/v1/${source}/${name}/events`
      : `/api/v1/${source}/${namespace}/${name}/events`,
  )
}

export function usePods(
  namespace?: string,
  name?: string,
  source: 'pod' | 'pv' | 'pvc' = 'pod',
  type: 'mountpods' | 'apppods' | 'csi-nodes' = 'apppods',
) {
  return useSWR<Pod[]>(
    source === 'pv'
      ? `/api/v1/${source}/${name}/${type}`
      : `/api/v1/${source}/${namespace}/${name}/${type}`,
  )
}

export function useWebsocket(
  uri?: string,
  opts?: Options,
  shouldConnect: boolean = false,
) {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'

  return useWebSocket(
    `${protocol}://${import.meta.env.VITE_HOST ?? window.location.host}${getBasePath()}${uri}`,
    {
      ...opts,
      heartbeat: {
        message: '{"type":"ping","data":"ping"}',
        interval: 3000,
      },
      onMessage: (msg) => {
        if (msg.data === 'pong') return
        opts?.onMessage?.(msg)
      },
    },
    shouldConnect,
  )
}

export function useDownloadPodLogs(
  namespace?: string,
  name?: string,
  container?: string,
) {
  return useAsync(async () => {
    await fetch(
      `${getHost()}/api/v1/pod/${namespace}/${name}/logs/${container}?download=true`,
    )
      .then((res) => res.blob())
      .then((blob) => {
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `${namespace}-${name}-${container}.log`
        a.click()
        window.URL.revokeObjectURL(url)
      })
  })
}

export function useDownloadPodDebugFiles(namespace?: string, name?: string) {
  return useAsync(async () => {
    await fetch(
      `${getHost()}/api/v1/pod/${namespace}/${name}/downloadDebugFile`,
    )
      .then((res) => res.blob())
      .then((blob) => {
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.download = `${namespace}-${name}-debug.zip`
        a.href = url
        a.click()
        window.URL.revokeObjectURL(url)
      })
  })
}

export function usePodsToUpgrade(recreate: boolean, nodeName?: string, uniqueId?: string) {
  const node = nodeName === 'All Nodes' ? '' : nodeName
  const recreateFlag = recreate ? 'true' : 'false'
  const uniqueIdStr = uniqueId !== undefined ? uniqueId : ''
  return useSWR<PodToUpgrade[]>(
    `/api/v1/batch/pods?nodeName=${node}&recreate=${recreateFlag}&uniqueIds=${uniqueIdStr}`,
  )
}

export function useNodes() {
  return useSWR<Node[]>('/api/v1/nodes')
}

export function useUpgradePods() {
  return useAsync(async ({ nodeName, recreate, worker, ignoreError, uniqueId }) => {
    const response = await fetch(`${getHost()}/api/v1/batch/upgrade`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        nodeName: nodeName === 'All Nodes' ? '' : nodeName,
        recreate: recreate,
        worker: worker,
        ignoreError: ignoreError,
        uniqueIds: [uniqueId],
      }),
    })
    const result: {
      jobName: string,
    } = await response.json()
    return result
  })
}

export function useUpgradeStatus() {
  return useSWR<Job>(`/api/v1/batch/job`)
}
