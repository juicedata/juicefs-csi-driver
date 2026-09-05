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
import {
  Button,
  Popover,
  Tooltip,
  type TablePaginationConfig,
  type TableProps,
} from 'antd'
import { Badge } from 'antd/lib'
import ReactDiffViewerModule from 'react-diff-viewer'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import { DiffIcon } from '@/icons'
import {
  PodDiffConfig,
  Setting,
  UpgradeJobWithDiff,
} from '@/types/k8s.ts'
import { getUpgradeStatusBadge } from '@/utils'
import { getUpgradeStatusKey } from '@/utils/upgrade'

const ReactDiffViewer = (
  ReactDiffViewerModule as unknown as { default: typeof ReactDiffViewerModule }
).default

interface UpgradeType {
  key: string
  statusKey: string
  name: string
  status: string
  diff: {
    oldSetting?: Setting
    newSetting?: Setting
  }
}

const diffContent = (podDiff: {
  oldSetting?: Setting
  newSetting?: Setting
}) => {
  const oldData = YAML.stringify(podDiff.oldSetting)
  const newData = YAML.stringify(podDiff.newSetting)
  return (
    <ReactDiffViewer
      oldValue={oldData}
      newValue={newData}
      splitView={true}
    ></ReactDiffViewer>
  )
}

const imageDiffContent = (oldImage?: string, newImage?: string) => {
  return (
    <ReactDiffViewer
      oldValue={oldImage || '-'}
      newValue={newImage || '-'}
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
  const [upgradeType, setUpgradeType] = useState<string>('mountPod')

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
          key: getUpgradeStatusKey(podUpgrade),
          statusKey: getUpgradeStatusKey(podUpgrade),
          name: podUpgrade.name,
          status: diffStatus.get(getUpgradeStatusKey(podUpgrade)) || '',
          diff: {
            oldSetting: newMap?.get(podUpgrade.name)?.oldSetting,
            newSetting: newMap?.get(podUpgrade.name)?.newSetting,
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
    setPagination((prev) => ({
      ...prev,
      total: getUpgradeTableTotal(upgradeJob, mountPodUpgrades.length),
    }))
    // Determine upgrade type
    const type = getUpgradeType(upgradeJob)
    setUpgradeType(type)
  }, [upgradeJob, diffStatus])

  const handleTableChange: TableProps<UpgradeType>['onChange'] = (
    pagination,
  ) => {
    setPagination(pagination)
  }
  const getUpgradeTableTitle = (): string => {
    if (upgradeType === 'sidecar') {
      return 'Application Pods'
    }
    return 'Mount Pods'
  }

  const getDiffTableTitle = (): React.ReactNode => {
    if (upgradeType === 'sidecar') {
      return 'Image'
    }
    return <FormattedMessage id="diff" />
  }

  const upgradeColumn: ProColumns<UpgradeType>[] = [
    {
      title: getUpgradeTableTitle(),
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
          diffStatus.get(podUpgrade.statusKey) || 'pending',
        )
        return (
          <>
            {podStatus !== 'fail' ? (
              <Badge
                status={getUpgradeStatusBadge(podStatus)}
                text={podStatus}
              />
            ) : (
              <Tooltip title={failReasons.get(podUpgrade.statusKey) || ''}>
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
      title: getDiffTableTitle(),
      key: 'diff',
      render: (_, podDiff) => {
        if (upgradeType === 'sidecar') {
          return (
            <Popover
              content={imageDiffContent('-', '-')}
              title="Image"
              trigger="click"
            >
              {diffStatus.get(podDiff.statusKey) !== 'success' ? (
                <Tooltip title={<FormattedMessage id="clickToViewDetail" />}>
                  <Button icon={<DiffIcon />} />
                </Tooltip>
              ) : (
                <Button disabled={true} icon={<DiffIcon />} />
              )}
            </Popover>
          )
        } else {
          // For mount pod, show config diff as before
          return (
            <Popover
              content={diffContent(podDiff.diff)}
              title={<FormattedMessage id="diff" />}
              trigger="click"
            >
              {diffStatus.get(podDiff.statusKey) !== 'success' ? (
                <Tooltip title={<FormattedMessage id="clickToViewDetail" />}>
                  <Button icon={<DiffIcon />} />
                </Tooltip>
              ) : (
                <Button disabled={true} icon={<DiffIcon />} />
              )}
            </Popover>
          )
        }
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
        pagination={pagination.total ? pagination : false}
        options={false}
        rowKey={(row) => row.key}
      />
    </ProCard>
  )
}

export default PodUpgradeTable

const getPodUpgradeStatus = (statusFromLog: string): string => {
  // Status is now only maintained in the log messages, not in the targets
  return statusFromLog
}

const getUpgradeType = (upgradeJob?: UpgradeJobWithDiff): string => {
  if (upgradeJob?.config?.kind === 'sidecar') {
    return 'sidecar'
  }
  const batches = upgradeJob?.config?.batches || []
  for (const batch of batches) {
    for (const target of batch || []) {
      if (target?.containerName) {
        return 'sidecar'
      }
    }
  }
  return 'mountPod'
}

const getUpgradeTableTotal = (
  upgradeJob?: UpgradeJobWithDiff,
  localRows = 0,
): number => {
  const backendTotal = upgradeJob?.total || 0
  return Math.max(backendTotal, localRows)
}
