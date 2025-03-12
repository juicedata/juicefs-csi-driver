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
import { ProColumns, ProTable } from '@ant-design/pro-components'
import {
  Button,
  Popover,
  TableProps,
  Tooltip,
  type TablePaginationConfig,
} from 'antd'
import ReactDiffViewer from 'react-diff-viewer'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import { useConfigDiff } from '@/hooks/cm-api.ts'
import { DiffIcon } from '@/icons'
import { PodDiffConfig } from '@/types/k8s.ts'

const diffContent = (podDiff: PodDiffConfig) => {
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

const upgradeColumn: ProColumns<PodDiffConfig>[] = [
  {
    title: <FormattedMessage id="diffMountPodName" />,
    key: 'name',
    render: (_, podUpgrade) => (
      <>
        {podUpgrade?.pod.metadata?.namespace || '' ? (
          <Link
            to={`/syspods/${podUpgrade.pod.metadata?.namespace || ''}/${podUpgrade.pod.metadata?.name}/`}
          >
            {podUpgrade.pod.metadata?.name}
          </Link>
        ) : (
          `${podUpgrade.pod.metadata?.name}`
        )}
      </>
    ),
  },
  {
    title: <FormattedMessage id="diff" />,
    key: 'diff',
    render: (_, podDiff) => {
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

const PodToUpgradeTable: React.FC<{
  nodeName?: string
  uniqueId?: string
  setDiffPods: (podDiff: PodDiffConfig[]) => void
}> = (props) => {
  const { nodeName, uniqueId, setDiffPods } = props
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    total: 0,
  })
  const { data: diffPods } = useConfigDiff(
    nodeName || '',
    uniqueId || '',
    pagination.pageSize,
    pagination.current,
  )
  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }
  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: diffPods?.total || 0 }))
  }, [diffPods?.total])

  useEffect(() => {
    setDiffPods(diffPods?.pods || [])
  }, [diffPods, setDiffPods])

  return (
    <>
      <ProTable<PodDiffConfig>
        className="diff-pods-table"
        columns={upgradeColumn}
        dataSource={diffPods?.pods}
        onChange={handleTableChange}
        search={false}
        pagination={diffPods?.total ? pagination : false}
        options={false}
        rowKey={(row) => row.pod.metadata!.uid!}
      />
    </>
  )
}

export default PodToUpgradeTable
