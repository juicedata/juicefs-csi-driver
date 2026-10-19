import { useAsync } from '@react-hookz/web'
import { Pod as NativePod } from 'kubernetes-types/core/v1'
import useSWR from 'swr'

import { CgWorkerPagingListArgs } from '@/types'
import { CacheGroup } from '@/types/k8s'
import { apiFetch } from '@/utils'

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
  const sortBy = Object.keys(pagination?.sort || {}).find(
    (key) => pagination?.sort?.[key],
  ) || 'time'
  const order = pagination?.sort?.[sortBy] || 'ascend'

  return useSWR<{ items: NativePod[]; total: number }>(
    `/api/v1/cachegroup/${namespace}/${name}/workers/?pageSize=${pagination?.pageSize}&current=${pagination?.current}&name=${pagination?.name ?? ''}&node=${pagination?.node ?? ''}&sortBy=${sortBy}&order=${order}${pagination?.unready ? '&unready=1' : ''}`,
    null,
    {
      refreshInterval,
    },
  )
}

type WorkerMutationResult = {
  succeeded: string[]
  failed: Array<{ nodeName: string; reason: string }>
}

async function mutateWorkers(
  namespace: string | undefined,
  name: string | undefined,
  nodeNames: string[],
  method: 'POST' | 'DELETE',
): Promise<WorkerMutationResult> {
  const results = await Promise.allSettled(
    nodeNames.map((nodeName) =>
      apiFetch(`/api/v1/cachegroup/${namespace}/${name}/workers/`, {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ nodeName }),
      }),
    ),
  )
  const succeeded: string[] = []
  const failed: WorkerMutationResult['failed'] = []
  results.forEach((result, index) => {
    const nodeName = nodeNames[index]
    if (result.status === 'fulfilled') {
      succeeded.push(nodeName)
    } else {
      failed.push({
        nodeName,
        reason:
          result.reason instanceof Error
            ? result.reason.message
            : String(result.reason),
      })
    }
  })
  return { succeeded, failed }
}

export function useRemoveWorker(namespace?: string, name?: string) {
  return useAsync(async ({ nodeNames }: { nodeNames: string[] }) =>
    mutateWorkers(namespace, name, nodeNames, 'DELETE'),
  )
}

export function useAddWorker(namespace?: string, name?: string) {
  return useAsync(async ({ nodeNames }: { nodeNames: string[] }) =>
    mutateWorkers(namespace, name, nodeNames, 'POST'),
  )
}

export function useCreateCacheGroup() {
  return useAsync(async ({ body }: { body: CacheGroup }) => {
    return await apiFetch(
      `/api/v1/cachegroup/${body.metadata?.namespace}/${body.metadata?.name}/`,
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
    return await apiFetch(
      `/api/v1/cachegroup/${body.metadata?.namespace}/${body.metadata?.name}/`,
      {
        method: 'PUT',
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
    return await apiFetch(
      `/api/v1/cachegroup/${body.metadata?.namespace}/${body.metadata?.name}/`,
      {
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      },
    )
  })
}
