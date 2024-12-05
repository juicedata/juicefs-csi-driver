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
import { BatchConfig, MountPodUpgrade, PodDiffConfig } from '@/types/k8s.ts'
import { useEffect, useState } from 'react'
import { Collapse, Table, TableProps } from 'antd'

const PodDiff: React.FC<{
  batchConfig?: BatchConfig,
  diffPods?: [PodDiffConfig],
  diffStatus: Map<string, string>
}> = (props) => {
  const { diffPods, batchConfig, diffStatus } = props
  const [podMap, setPodMap] = useState<Map<string, Pod>>()
  const [activeStage, setActiveStage] = useState(0)

  useEffect(() => {
    batchConfig?.batches?.forEach((batch, i) => {
      batch?.map((podUpgrade) => {
        const status = diffStatus.get(podUpgrade.name)
        if (status === 'running') {
          setActiveStage(i)
        }
      })
    })
  }, [diffStatus, batchConfig])

  useEffect(() => {
    const newMap = new Map()
    diffPods?.forEach((poddiff) => {
      const podName = poddiff.pod?.metadata?.name || ''
      newMap.set(podName, poddiff.pod)
    })
    setPodMap(newMap)
  }, [diffPods])

  interface UpgradeType {
    key: string
    name: string
    status: string
  }

  const podUpgradeData = (podUpgrades: MountPodUpgrade[]) => {
    return podUpgrades.map((podUpgrade) => {
      return {
        key: podUpgrade.name,
        name: podUpgrade.name,
        status: diffStatus.get(podUpgrade.name) || '',
      }
    })
  }

  const upgradeColumn: TableProps<UpgradeType>['columns'] = [
    {
      title: <FormattedMessage id="podName" />,
      key: 'name',
      render: (podUpgrade) => <>{
        (podMap?.get(podUpgrade.name)?.metadata?.namespace || '') ?
          <Link
            to={`/syspods/${podMap?.get(podUpgrade.name)?.metadata?.namespace || ''}/${podUpgrade.name}/`}>
            {podUpgrade.name}
          </Link> : `${podUpgrade.name}`
      }</>,
    },
    {
      title: <FormattedMessage id="upgradeStatus" />,
      key: 'status',
      render: (podUpgrade) => <Badge
        status={getUpgradeStatusBadge(diffStatus.get(podUpgrade.name) || '')}
        text={`${diffStatus.get(podUpgrade.name) || 'pending'}`}
      />,
    },
  ]

  const stageItems = batchConfig?.batches?.map((podUpgrades, i) => ({
    key: i.toString(),
    label: (<> <FormattedMessage id="batch" /> {i + 1} </>),
    children: <ProCard colSpan={6}>
      <Table<UpgradeType> pagination={false} columns={upgradeColumn} dataSource={podUpgradeData(podUpgrades) || []} />
    </ProCard>,
  })) || []

  return (
    <ProCard
      title={<FormattedMessage id="diffPods" />}
      key="diffPods"
      style={{ marginBlockStart: 4 }}
      gutter={4}
      wrap
    >
      <Collapse items={stageItems} defaultActiveKey={[activeStage]} />
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
