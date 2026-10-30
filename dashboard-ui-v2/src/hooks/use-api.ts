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

import { useRef } from 'react'
import { useAsync } from '@react-hookz/web'
import {
  Event,
  Node,
  PersistentVolume,
  PersistentVolumeClaim,
} from 'kubernetes-types/core/v1'
import useWebSocketImport from 'react-use-websocket'
import type { Options } from 'react-use-websocket'
import useSWR from 'swr'

import { AppPagingListArgs, SysPagingListArgs } from '@/types'
import { Pod } from '@/types/k8s'
import { apiFetchBlob, getBasePath } from '@/utils'

const useWebSocket =
  typeof useWebSocketImport === 'function'
    ? useWebSocketImport
    : (
        useWebSocketImport as unknown as {
          default: typeof useWebSocketImport
        }
      ).default

export function useAppPods(args: AppPagingListArgs) {
  const sortBy = Object.keys(args.sort || {}).find(
    (key) => args.sort?.[key],
  ) || 'time'
  const order = args.sort?.[sortBy] || 'descend'
  const namespace = args.namespace || ''
  const name = args.name || ''
  const pv = args.pv || ''
  const csiNode = args.csiNode || ''
  const mountPod = args.mountPod || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1
  const continueToken = args.continue || ''

  return useSWR<{
    pods: Pod[]
    total: number
    continue?: string
  }>(
    `/api/v1/pods?sortBy=${sortBy}&order=${order}&namespace=${namespace}&name=${name}&pv=${pv}&mountpod=${mountPod}&csinode=${csiNode}&pageSize=${pageSize}&current=${current}&continue=${continueToken}`,
  )
}

export function useSysAppPods(args: SysPagingListArgs) {
  const sortBy = Object.keys(args.sort || {}).find(
    (key) => args.sort?.[key],
  ) || 'time'
  const order = args.sort?.[sortBy] || 'ascend'
  const namespace = args.namespace || ''
  const name = args.name || ''
  const node = args.node || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1
  const continueToken = args.continue || ''

  return useSWR<{
    pods: Pod[]
    total: number
    continue?: string
  }>(
    `/api/v1/syspods?namespace=${namespace}&name=${name}&node=${node}&sortBy=${sortBy}&order=${order}&pageSize=${pageSize}&current=${current}&continue=${continueToken}`,
  )
}

export function useMountPodImage(
  isMountPod: boolean,
  namespace?: string,
  name?: string,
) {
  return useSWR<string>(
    isMountPod ? `/api/v1/pod/${namespace}/${name}/latestimage` : '',
  )
}

export function useAppPod(namespace?: string, name?: string) {
  return useSWR<Pod>(`/api/v1/pod/${namespace}/${name}/`)
}

export function useEvents(
  source: 'pod' | 'pv' | 'pvc' | 'cachegroup' = 'pod',
  namespace?: string,
  name?: string,
) {
  return useSWR<Event[]>(
    source === 'pv'
      ? `/api/v1/${source}/${name}/events`
      : `/api/v1/${source}/${namespace}/${name}/events`,
    { refreshInterval: source === 'cachegroup' ? 10000 : 0 },
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

export function usePVsOfPod(namespace?: string, name?: string) {
  return useSWR<PersistentVolume[]>(`/api/v1/pod/${namespace}/${name}/pvs`)
}

export function usePVCsOfPod(namespace?: string, name?: string) {
  return useSWR<PersistentVolumeClaim[]>(
    `/api/v1/pod/${namespace}/${name}/pvcs`,
  )
}

// Pod output arrives as binary frames so that the bytes stay intact: an
// interactive terminal needs that for ZMODEM file transfer (rz/sz), whose
// payload is not valid UTF-8 text. Views that print the output as text use
// `useWebsocket`, which decodes the frames for them; a view that needs the raw
// bytes, such as the terminal that drives ZMODEM, uses `useBinaryWebsocket`.
export type TextMessageEvent = { data: string }
export type TextWebsocketOptions = Omit<Options, 'onMessage'> & {
  onMessage?: (event: TextMessageEvent) => void
}

function useDecodedWebsocket(
  uri: string | undefined,
  opts: Options | undefined,
  shouldConnect: boolean,
  decodeBinary: boolean,
) {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  // A streaming decoder keeps a UTF-8 sequence that is split across two frames
  // intact, and a fresh one per connection drops any leftover from the old one.
  const decoderRef = useRef<TextDecoder | null>(null)
  // Frames that arrive as a Blob can only be read asynchronously, so their
  // decoding goes through this chain to keep the order of the frames.
  const decodeQueueRef = useRef<Promise<void>>(Promise.resolve())

  return useWebSocket(
    `${protocol}://${import.meta.env.VITE_HOST ?? window.location.host}${getBasePath()}${uri}`,
    {
      ...opts,
      onOpen: (event) => {
        const socket = event.currentTarget as WebSocket | null
        // react-use-websocket cannot set binaryType, and the default `blob`
        // would force an asynchronous read for every frame.
        if (socket) socket.binaryType = 'arraybuffer'
        decoderRef.current = new TextDecoder()
        decodeQueueRef.current = Promise.resolve()
        opts?.onOpen?.(event)
      },
      heartbeat: {
        message: '{"type":"ping","data":"ping"}',
        interval: 3000,
      },
      onMessage: (msg) => {
        if (typeof msg.data === 'string') {
          if (msg.data === 'pong') return
          opts?.onMessage?.(msg)
          return
        }
        if (!decodeBinary) {
          opts?.onMessage?.(msg)
          return
        }
        if (msg.data instanceof ArrayBuffer) {
          decoderRef.current ??= new TextDecoder()
          const decoded = decoderRef.current.decode(msg.data, {
            stream: true,
          })
          opts?.onMessage?.({ data: decoded } as unknown as MessageEvent)
          return
        }
        if (msg.data instanceof Blob) {
          // Setting binaryType should make this unreachable, but if it did not
          // take effect the frame must still be decoded instead of being handed
          // to the consumer as a Blob.
          const blob = msg.data
          decodeQueueRef.current = decodeQueueRef.current.then(async () => {
            try {
              const buffer = await blob.arrayBuffer()
              decoderRef.current ??= new TextDecoder()
              const decoded = decoderRef.current.decode(buffer, {
                stream: true,
              })
              opts?.onMessage?.({ data: decoded } as unknown as MessageEvent)
            } catch (error) {
              console.error('Failed to decode websocket frame', error)
            }
          })
          return
        }
        opts?.onMessage?.(msg)
      },
    },
    shouldConnect,
  )
}

export function useWebsocket(
  uri?: string,
  opts?: TextWebsocketOptions,
  shouldConnect: boolean = false,
) {
  return useDecodedWebsocket(uri, opts, shouldConnect, true)
}

export function useBinaryWebsocket(
  uri?: string,
  opts?: Options,
  shouldConnect: boolean = false,
) {
  return useDecodedWebsocket(uri, opts, shouldConnect, false)
}

export function triggerBlobDownload(blob: Blob, filename: string) {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  window.setTimeout(() => {
    window.URL.revokeObjectURL(url)
  }, 1000)
}

export function useDownloadPodLogs(
  namespace?: string,
  name?: string,
  container?: string,
) {
  return useAsync(async () => {
    const blob = await apiFetchBlob(
      `/api/v1/pod/${namespace}/${name}/logs/${container}?download=true`,
    )
    triggerBlobDownload(blob, `${namespace}-${name}-${container}.log`)
  })
}

export function useDownloadPodDebugInfos(namespace?: string, name?: string) {
  return useAsync(async () => {
    const blob = await apiFetchBlob(
      `/api/v1/pod/${namespace}/${name}/downloadAllDebugInfo`,
    )
    const now = new Date()
    const formattedDate = now.toLocaleDateString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
    const formattedTime = now.toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    })
    triggerBlobDownload(
      blob,
      `${name}-debug-collect-${formattedDate}-${formattedTime}.zip`,
    )
  })
}

export function useDownloadPodDebugFiles(namespace?: string, name?: string) {
  return useAsync(async () => {
    const blob = await apiFetchBlob(
      `/api/v1/pod/${namespace}/${name}/downloadDebugFile`,
    )
    triggerBlobDownload(blob, `${namespace}-${name}-debug.zip`)
  })
}

export function useNodes() {
  return useSWR<Node[]>('/api/v1/nodes')
}
