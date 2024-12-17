/*
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
import useSWR from 'swr'

import { UpgradeJobsPagingListArgs } from '@/types'
import { BatchConfig, UpgradeJob, UpgradeJobWithDiff } from '@/types/k8s.ts'
import { getHost } from '@/utils'

export function useCreateUpgradeJob() {
  return useAsync(async (batchConfig?: BatchConfig, jobName?: string) => {
    const response = await fetch(`${getHost()}/api/v1/batch/upgrade/jobs`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        batchConfig: batchConfig,
        jobName: jobName,
      }),
    })
    const result: {
      jobName: string
    } = await response.json()
    return result
  })
}

export function useUpgradeJob(jobName: string) {
  return useSWR<UpgradeJobWithDiff>(`/api/v1/batch/upgrade/jobs/${jobName}`)
}

export function useUpgradeJobs(args: UpgradeJobsPagingListArgs) {
  const order = args.sort?.['time'] || 'descend'
  const namespace = args.namespace || ''
  const name = args.name || ''
  const pageSize = args.pageSize || 20
  const current = args.current || 1

  return useSWR<{
    jobs: UpgradeJob[]
    total: number
  }>(
    `/api/v1/batch/upgrade/jobs?order=${order}&namespace=${namespace}&name=${name}&pageSize=${pageSize}&current=${current}`,
  )
}

export function useDeleteUpgradeJob() {
  return useAsync(async (jobName: string) => {
    await fetch(`${getHost()}/api/v1/batch/upgrade/jobs/${jobName}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
      },
    })
    return
  })
}

export function useUpdateUpgradeJob() {
  return useAsync(async (jobName: string, action: string) => {
    await fetch(`${getHost()}/api/v1/batch/upgrade/jobs/${jobName}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        action: action,
      }),
    })
    return
  })
}

export function useBatchPlan(
  nodeName: string,
  uniqueId: string,
  worker: number,
  ignoreError: boolean,
  recreate: boolean,
) {
  const node = nodeName === 'All Nodes' ? '' : nodeName
  return useSWR<BatchConfig>(
    `/api/v1/batch/upgrade/plan?nodeName=${node}&uniqueId=${uniqueId}&worker=${worker}&ignoreError=${ignoreError}&recreate=${recreate}`,
  )
}
