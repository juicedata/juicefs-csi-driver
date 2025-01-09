import { useAsync } from '@react-hookz/web'
import { ConfigMap } from 'kubernetes-types/core/v1'
import useSWR from 'swr'

import { PodDiffConfig } from '@/types/k8s.ts'
import { getHost } from '@/utils'

export function useConfig() {
  return useSWR<ConfigMap>(`/api/v1/config`)
}

export function useUpdateConfig() {
  return useAsync(async (config: ConfigMap) => {
    const response = await fetch(`${getHost()}/api/v1/config`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    })

    if (!response.ok) {
      const errorText = await response.text()
      throw new Error(`${errorText}`)
    }
  })
}

export function useConfigDiff(nodeName: string, uniqueId: string, pageSize?: number, current?: number) {
  const size = pageSize || 20
  const currentPage = current || 1
  const node = nodeName === 'All Nodes' ? '' : nodeName

  return useSWR<{
    total: number
    pods: PodDiffConfig[]
  }>(
    `/api/v1/config/diff?nodeName=${node}&uniqueIds=${uniqueId}&pageSize=${size}&current=${currentPage}`,
  )
}
