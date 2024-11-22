import { useAsync } from '@react-hookz/web'
import { Pod as NativePod, Node } from 'kubernetes-types/core/v1'
import useSWR from 'swr'

import { CacheGroup } from '@/types/k8s'
import { getHost } from '@/utils'

export function useCacheGroups() {
  return useSWR<CacheGroup[]>(`/api/v1/cachegroups`)
}

export function useCacheGroup(namespace?: string, name?: string) {
  return useSWR<CacheGroup>(`/api/v1/cachegroup/${namespace}/${name}/`)
}

export function useCacheGroupWorkers(
  namespace?: string,
  name?: string,
  refreshInterval = 0,
) {
  return useSWR<NativePod[]>(
    `/api/v1/cachegroup/${namespace}/${name}/workers`,
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
