import { useAsync } from '@react-hookz/web'
import { Pod as NativePod, Node } from 'kubernetes-types/core/v1'
import useSWR from 'swr'

import { CgWorkerPagingListArgs } from '@/types'
import { CacheGroup } from '@/types/k8s'
import { getHost } from '@/utils'

export function useCacheGroups() {
  return useSWR<CacheGroup[]>(`/api/v1/cachegroups`)
}

export function useCacheGroup(namespace?: string, name?: string) {
  return useSWR<CacheGroup>(`/api/v1/cachegroup/${namespace}/${name}/`)
}

export function useWorkerCacheBytes(
  namespace?: string,
  name?: string,
  worker?: string,
  refreshInterval = 0,
) {
  return useSWR<{ result: number }>(
    `/api/v1/cachegroup/${namespace}/${name}/workers/${worker}/cacheBytes`,
    null,
    { refreshInterval },
  )
}

export function useCacheGroupWorkers(
  namespace?: string,
  name?: string,
  refreshInterval = 0,
  pagination?: CgWorkerPagingListArgs,
) {
  return useSWR<{ items: NativePod[]; total: number }>(
    `/api/v1/cachegroup/${namespace}/${name}/workers?pageSize=${pagination?.pageSize}&current=${pagination?.current}&name=${pagination?.name ?? ''}&node=${pagination?.node ?? ''}`,
    null,
    {
      refreshInterval,
    },
  )
}

export function useNodes(namespace?: string, name?: string) {
  return useSWR<Node[]>(`/api/v1/cachegroup/${namespace}/${name}/nodes`)
}

export function useRemoveWorker(namespace?: string, name?: string) {
  return useAsync(async ({ nodeName }) => {
    return await fetch(
      `${getHost()}/api/v1/cachegroup/${namespace}/${name}/removeWorker`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          nodeName: nodeName,
        }),
      },
    )
  })
}

export function useAddWorker(namespace?: string, name?: string) {
  return useAsync(async ({ nodeName }) => {
    return await fetch(
      `${getHost()}/api/v1/cachegroup/${namespace}/${name}/addWorker`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          nodeName: nodeName,
        }),
      },
    )
  })
}

export function useCreateCacheGroup() {
  return useAsync(async ({ body }: { body: CacheGroup }) => {
    return await fetch(
      `${getHost()}/api/v1/cachegroup/${body.metadata?.namespace}/${body.metadata?.name}/create`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      },
    )
  })
}

export function useUpdateCacheGroup() {
  return useAsync(async ({ body }: { body: CacheGroup }) => {
    return await fetch(
      `${getHost()}/api/v1/cachegroup/${body.metadata?.namespace}/${body.metadata?.name}/update`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(body),
      },
    )
  })
}

export function useDeleteCacheGroup() {
  return useAsync(async ({ body }: { body: CacheGroup }) => {
    return await fetch(
      `${getHost()}/api/v1/cachegroup/${body.metadata?.namespace}/${body.metadata?.name}/delete`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
      },
    )
  })
}
