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


import { Pod } from 'kubernetes-types/core/v1'
import { ProCard } from '@ant-design/pro-components'
import { Badge } from 'antd/lib'
import { Link } from 'react-router-dom'
import { FormattedMessage } from 'react-intl'
import { BatchConfig, MountPodUpgrade } from '@/types/k8s.ts'
import { useEffect, useState } from 'react'
import { Collapse } from 'antd'

const PodDiff: React.FC<{
  batchConfig?: BatchConfig,
  diffPods?: [Pod],
  diffStatus: Map<string, string>
}> = (props) => {
  const { diffPods, batchConfig, diffStatus } = props
  const [podMap, setPodMap] = useState<Map<string, Pod>>()

  useEffect(() => {
    const newMap = new Map()
    diffPods?.forEach((pod) => {
      const podName = pod?.metadata?.name || ''
      newMap.set(podName, pod)
    })
    setPodMap(newMap)
  }, [diffPods])

  const stageItems = batchConfig?.batches?.map((podUpgrades, i) => ({
    key: i.toString(),
    label: (<> <FormattedMessage id="stage" /> {i + 1} </>),
    children: <ProCard colSpan={6}>
      {podUpgrades.map(getUpgradeCard)}
    </ProCard>,
  })) || []

  function getUpgradeCard(podUpgrade: MountPodUpgrade) {
    const name = podUpgrade.name
    const namespace = podMap?.get(podUpgrade.name)?.metadata?.namespace || ''
    const uid = podMap?.get(podUpgrade.name)?.metadata?.uid || name
    return <ProCard key={uid} colSpan={6}>
      <Badge
        status={getUpgradeStatusBadge(diffStatus.get(name) || '')}
        text={namespace ?
          <Link
            to={`/syspods/${namespace}/${name}/`}>
            {name}
          </Link> : `${name}`
        }
      />
    </ProCard>
  }

  return (
    <ProCard
      title={<FormattedMessage id="diffPods" />}
      key="diffPods"
      style={{ marginBlockStart: 4 }}
      gutter={4}
      wrap
    >
      <Collapse items={stageItems} defaultActiveKey={['0']} />
    </ProCard>
  )
}

export default PodDiff

const getUpgradeStatusBadge = (finalStatus: string) => {
  switch (finalStatus) {
    case 'pending':
      return 'default'
    case 'running':
    case 'start':
      return 'processing'
    case 'success':
      return 'success'
    case 'fail':
      return 'error'
    default:
      return 'default'
  }
}

