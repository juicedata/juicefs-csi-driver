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

import { Spin } from 'antd'

import { useWorkerCacheBytes } from '@/hooks/cg-api'

const WorkerCacheBytes: React.FC<{
  namespace?: string
  name?: string
  workerName?: string
}> = ({ namespace, name, workerName }) => {
  const { data, isLoading } = useWorkerCacheBytes(
    namespace,
    name,
    workerName,
    5000,
  )
  if (isLoading) {
    return <Spin />
  }

  // bytes to human readable format
  const humanReadable = (bytes: number) => {
    if (bytes == 0) {
      return '-'
    }
    const i = Math.floor(Math.log(bytes) / Math.log(1024))
    return (
      parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) +
      ' ' +
      ['B', 'KB', 'MB', 'GB', 'TB'][i]
    )
  }

  return <>{humanReadable(data?.result ?? 0)}</>
}

export default WorkerCacheBytes
