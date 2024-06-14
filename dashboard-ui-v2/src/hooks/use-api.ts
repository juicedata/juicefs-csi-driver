import useSWR from 'swr'

import { AppPagingListArgs, SysPagingListArgs } from '@/types'
import { Pod } from '@/types/k8s'

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
