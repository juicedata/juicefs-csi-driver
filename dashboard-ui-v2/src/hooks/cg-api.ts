import { useAsync } from '@react-hookz/web'
import { Pod as NativePod } from 'kubernetes-types/core/v1'
import useSWR from 'swr'

import { CgWorkerPagingListArgs } from '@/types'
import { CacheGroup, Warmup } from '@/types/k8s'
import { apiFetch } from '@/utils'

export function useCacheGroups() {
  return useSWR<CacheGroup[]>(`/api/v1/cachegroups`)
}

export function warmupCacheGroup(warmup: Warmup, groups: CacheGroup[]) {
  if (warmup.spec.secretRef || !warmup.spec.cacheGroupName) return undefined
  const candidates = groups.filter(
    (group) => group.metadata?.namespace === warmup.metadata.namespace,
  )
  return (
    candidates.find((group) => group.metadata?.name === warmup.spec.cacheGroupName) ||
    candidates.find((group) => group.status?.cacheGroup === warmup.spec.cacheGroupName)
  )
}

export function useCacheGroup(namespace?: string, name?: string, refresh = true) {
  return useSWR<CacheGroup>(`/api/v1/cachegroup/${namespace}/${name}/`, {
    refreshInterval: refresh ? 10000 : 0,
  })
}

export function useCacheGroupStats(group?: CacheGroup) {
  const status = group?.status
  const hasStats = !!(
    status?.stats?.length || status?.cacheSize || status?.cacheCapacity
  )
  const namespace = group?.metadata?.namespace
  const name = group?.metadata?.name
  const { data } = useSWR<
    Pick<NonNullable<CacheGroup['status']>, 'stats' | 'cacheSize' | 'cacheCapacity'>
  >(
    !hasStats && namespace && name
      ? `/api/v1/cachegroup/${namespace}/${name}/stats`
      : null,
    { refreshInterval: 30000, dedupingInterval: 30000 },
  )
  return hasStats ? status : data
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

  return useSWR<{ items: NativePod[]; total: number; statuses: string[] }>(
    `/api/v1/cachegroup/${namespace}/${name}/workers/?pageSize=${pagination?.pageSize}&current=${pagination?.current}&name=${pagination?.name ?? ''}&node=${pagination?.node ?? ''}&sortBy=${sortBy}&order=${order}${pagination?.unready ? '&unready=1' : ''}${pagination?.status?.map((status) => `&status=${encodeURIComponent(status)}`).join('') ?? ''}`,
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
