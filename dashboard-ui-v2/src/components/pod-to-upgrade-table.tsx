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
import { Button, Popover, Table, TableProps, Tooltip } from 'antd'
import ReactDiffViewer from 'react-diff-viewer'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import { DiffIcon } from '@/icons'
import {
  BatchConfig,
  MountPatch,
  MountPodUpgrade,
  PodDiffConfig,
} from '@/types/k8s.ts'

const PodToUpgradeTable: React.FC<{
  batchConfig?: BatchConfig
  diffPods?: [PodDiffConfig]
}> = (props) => {
  const { diffPods, batchConfig } = props
  const [podMap, setPodMap] = useState<Map<string, PodDiffConfig>>()

  useEffect(() => {
    const newMap = new Map()
    diffPods?.forEach((poddiff) => {
      const podName = poddiff.pod?.metadata?.name || ''
      newMap.set(podName, poddiff)
    })
    setPodMap(newMap)
  }, [diffPods])

  interface UpgradeType {
    key: string
    name: string
    diff: {
      oldConfig?: MountPatch
      newConfig?: MountPatch
    }
  }

  const diffContent = (podDiff: UpgradeType) => {
    const oldData = YAML.stringify(podDiff.diff.oldConfig)
    const newData = YAML.stringify(podDiff.diff.newConfig)
    return (
      <ReactDiffViewer
        oldValue={oldData}
        newValue={newData}
        splitView={true}
      ></ReactDiffViewer>
    )
  }

  const mountPods = (batchs: MountPodUpgrade[][]): UpgradeType[] => {
    const pods = batchs.map((pods) => {
      return podUpgradeData(pods)
    })
    const mountPodUpgrades: UpgradeType[] = []
    for (let i = 0; i < pods.length; i++) {
      for (let j = 0; j < pods[i].length; j++) {
        mountPodUpgrades.push(pods[i][j])
      }
    }
    return mountPodUpgrades
  }

  const podUpgradeData = (podUpgrades: MountPodUpgrade[]) => {
    return podUpgrades.map((podUpgrade) => {
      return {
        key: podUpgrade.name,
        name: podUpgrade.name,
        diff: {
          oldConfig: podMap?.get(podUpgrade.name)?.oldConfig,
          newConfig: podMap?.get(podUpgrade.name)?.newConfig,
        },
      }
    })
  }

  const upgradeColumn: TableProps<UpgradeType>['columns'] = [
    {
      title: <FormattedMessage id="diffMountPodName" />,
      key: 'name',
      render: (podUpgrade) => (
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
      title: <FormattedMessage id="diff" />,
      key: 'diff',
      render: (podDiff) => {
        return (
          <Popover
            content={diffContent(podDiff)}
            title={<FormattedMessage id="diff" />}
            trigger="click"
          >
            <Tooltip title={<FormattedMessage id="clickToViewDetail" />}>
              <Button icon={<DiffIcon />} />
            </Tooltip>
          </Popover>
        )
      },
    },
  ]

  return (
    <>
      <Table<UpgradeType>
        className="diff-pods-table"
        columns={upgradeColumn}
        dataSource={mountPods(batchConfig?.batches || []) || []}
      />
    </>
  )
}

export default PodToUpgradeTable
