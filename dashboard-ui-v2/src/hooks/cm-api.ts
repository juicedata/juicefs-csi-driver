import { useAsync } from '@react-hookz/web'
import { ConfigMap } from 'kubernetes-types/core/v1'
import useSWR from 'swr'

import { getHost } from '@/utils'

export function useConfig() {
  return useSWR<ConfigMap>(`/api/v1/config`)
}

export function useUpdateConfig() {
  return useAsync(async (config: ConfigMap) => {
    await fetch(`${getHost()}/api/v1/config`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(config),
    })
  })
}
