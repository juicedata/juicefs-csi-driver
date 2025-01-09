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

import React, { useEffect, useState } from 'react'
import { ProCard, ProColumns, ProTable } from '@ant-design/pro-components'
import { Button, Popover, type TablePaginationConfig, TableProps, Tooltip } from 'antd'
import { Badge } from 'antd/lib'
import ReactDiffViewer from 'react-diff-viewer'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import { DiffIcon } from '@/icons'
import {
  BatchConfig,
  MountPatch,
  PodDiffConfig, UpgradeJobWithDiff,
} from '@/types/k8s.ts'
import { getUpgradeStatusBadge } from '@/utils'

interface UpgradeType {
  key: string
  name: string
  status: string
  diff: {
    oldConfig?: MountPatch
    newConfig?: MountPatch
  }
}

const diffContent = (podDiff: {
  oldConfig?: MountPatch
  newConfig?: MountPatch
}) => {
  const oldData = YAML.stringify(podDiff.oldConfig)
  const newData = YAML.stringify(podDiff.newConfig)
  return (
    <ReactDiffViewer
      oldValue={oldData}
      newValue={newData}
      splitView={true}
    ></ReactDiffViewer>
  )
}

const PodUpgradeTable: React.FC<{
  upgradeJob?: UpgradeJobWithDiff
  diffStatus: Map<string, string>
  failReasons: Map<string, string>
}> = (props) => {
  const { upgradeJob, diffStatus, failReasons } = props
  const [podMap, setPodMap] = useState<Map<string, PodDiffConfig>>()
  const [mountPods, setMountPods] = useState<UpgradeType[]>([])
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    total: 0,
  })

  useEffect(() => {
    const newMap = new Map()
    upgradeJob?.diffs?.forEach((poddiff) => {
      const podName = poddiff.pod?.metadata?.name || ''
      newMap.set(podName, poddiff)
    })
    setPodMap(newMap)
    const pods = upgradeJob?.config.batches.map((mp): UpgradeType[] => {
      return mp.map((podUpgrade) => {
        return {
          key: podUpgrade.name,
          name: podUpgrade.name,
          status: diffStatus.get(podUpgrade.name) || '',
          diff: {
            oldConfig: newMap?.get(podUpgrade.name)?.oldConfig,
            newConfig: newMap?.get(podUpgrade.name)?.newConfig,
          },
        }
      })
    })
    const mountPodUpgrades: UpgradeType[] = []
    if (pods) {
      for (let i = 0; i < (pods.length || 0); i++) {
        for (let j = 0; j < (pods[i].length || 0); j++) {
          mountPodUpgrades.push(pods[i][j])
        }
      }
    }
    setMountPods(mountPodUpgrades)
    console.log("mountPods", mountPodUpgrades)
  }, [upgradeJob, diffStatus])

  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }
  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: upgradeJob?.total || 0 }))
  }, [upgradeJob?.total])

  const upgradeColumn: ProColumns<UpgradeType>[] = [
    {
      title: 'Mount Pods',
      key: 'name',
      render: (_, podUpgrade) => (
        <>
          {podMap?.get(podUpgrade.name)?.pod.metadata?.namespace || '' ? (
            <Link
              to={`/syspods/${podMap?.get(podUpgrade.name)?.pod.metadata?.namespace || ''}/${podUpgrade.name}/`}
            >
              {podUpgrade.name}
            </Link>
          ) : (
            `${podUpgrade.name}`
          )}
        </>
      ),
    },
    {
      title: <FormattedMessage id="upgradeStatus" />,
      key: 'status',
      render: (_, podUpgrade) => {
        const podStatus = getPodUpgradeStatus(
          podUpgrade.name,
          diffStatus.get(podUpgrade.name) || 'pending',
          upgradeJob?.config,
        )
        return (
          <>
            {podStatus !== 'fail' ? (
              <Badge
                status={getUpgradeStatusBadge(podStatus)}
                text={podStatus}
              />
            ) : (
              <Tooltip title={failReasons.get(podUpgrade.name) || ''}>
                <Badge
                  status={getUpgradeStatusBadge(podStatus)}
                  text={podStatus}
                />
              </Tooltip>
            )}
          </>
        )
      },
    },
    {
      title: <FormattedMessage id="diff" />,
      key: 'diff',
      render: (_, podDiff) => {
        return (
          <Popover
            content={diffContent(podDiff.diff)}
            title={<FormattedMessage id="diff" />}
            trigger="click"
          >
            {diffStatus.get(podDiff.name) !== 'success' ? (
              <Tooltip title={<FormattedMessage id="clickToViewDetail" />}>
                <Button icon={<DiffIcon />} />
              </Tooltip>
            ) : (
              <Button disabled={true} icon={<DiffIcon />} />
            )}
          </Popover>
        )
      },
    },
  ]

  return (
    <ProCard>
      <ProTable<UpgradeType>
        columns={upgradeColumn}
        dataSource={mountPods}
        onChange={handleTableChange}
        search={false}
        pagination={upgradeJob?.total ? pagination : false}
        options={false}
      />
    </ProCard>
  )
}

export default PodUpgradeTable

const getPodUpgradeStatus = (
  podName: string,
  statusFromLog: string,
  config?: BatchConfig,
): string => {
  if (statusFromLog !== 'pending') {
    return statusFromLog
  }
  let status = statusFromLog
  config?.batches.forEach((batch) => {
    batch.forEach((pod) => {
      if (pod.name === podName && pod.status !== '') {
        status = pod.status
      }
    })
  })
  return status
}
