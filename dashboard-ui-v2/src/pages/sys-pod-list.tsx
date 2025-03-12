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

import React, { useEffect, useState } from 'react'
import { AlertTwoTone } from '@ant-design/icons'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import {
  Button,
  Tooltip,
  type TablePaginationConfig,
  type TableProps,
} from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { useSysAppPods } from '@/hooks/use-api'
import { Pod } from '@/types/k8s'
import {
  failedReasonOfMountPod,
  getNodeStatusBadge,
  getPodStatusBadge,
  podStatus,
} from '@/utils'

const columns: ProColumns<Pod>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    render: (_, pod) => {
      const podFailReason = failedReasonOfMountPod(pod) || ''
      if (podFailReason === '') {
        return (
          <Link
            to={`/syspods/${pod.metadata?.namespace}/${pod.metadata?.name}`}
          >
            {pod.metadata?.name}
          </Link>
        )
      }
      const failReason = <FormattedMessage id={podFailReason} />
      return (
        <div>
          <Link
            to={`/syspods/${pod.metadata?.namespace}/${pod.metadata?.name}`}
          >
            {pod.metadata?.name}
          </Link>
          <Tooltip title={failReason}>
            <AlertTwoTone twoToneColor="#cf1322" />
          </Tooltip>
        </div>
      )
    },
  },
  {
    title: <FormattedMessage id="namespace" />,
    dataIndex: ['metadata', 'namespace'],
  },
  {
    title: <FormattedMessage id="status" />,
    key: 'status',
    hideInSearch: true,
    render: (_, pod) => {
      const finalStatus = podStatus(pod)
      return (
        <Badge
          color={getPodStatusBadge(finalStatus || '')}
          text={finalStatus}
        />
      )
    },
  },
  {
    title: <FormattedMessage id="createAt" />,
    dataIndex: ['metadata', 'creationTimestamp'],
    hideInSearch: true,
    render: (_, row) =>
      new Date(row.metadata?.creationTimestamp as string).toLocaleString(),
  },
  {
    title: <FormattedMessage id="node" />,
    key: 'node',
    dataIndex: ['spec', 'nodeName'],
    valueType: 'text',
    render: (_, pod) => {
      if (!pod.node) {
        return '-'
      }
      return (
        <Badge color={getNodeStatusBadge(pod.node)} text={pod.spec?.nodeName} />
      )
    },
  },
]

const SysPodList: React.FC = () => {
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    total: 0,
  })

  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }

  const [filter, setFilter] = useState<{
    name?: string
    namespace?: string
    node?: string
    continue?: string
  }>()

  const { data, isLoading } = useSysAppPods({
    current: pagination.current,
    pageSize: pagination.pageSize,
    ...filter,
  })

  const [continueToken, setContinueToken] = useState<string | undefined>()

  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])
  useEffect(() => {
    setContinueToken(data?.continue)
  }, [data?.continue])

  return (
    <PageContainer
      header={{
        title: <FormattedMessage id="systemPodTablePageName" />,
      }}
    >
      <ProTable<Pod>
        headerTitle={<FormattedMessage id="sysPodTableName" />}
        columns={columns}
        loading={isLoading}
        dataSource={data?.pods}
        pagination={data?.total ? pagination : false}
        onChange={handleTableChange}
        rowKey={(row) => row.metadata!.uid!}
        search={{
          optionRender: false,
          collapsed: false,
        }}
        form={{
          onValuesChange: (_, values) => {
            if (values) {
              setFilter((prev) => ({
                ...prev,
                ...values,
                ...values.metadata,
              }))
            }
          },
        }}
      />
      {continueToken && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            marginTop: 16,
          }}
        >
          <Button
            onClick={() =>
              setFilter({
                ...filter,
                continue: continueToken,
              })
            }
          >
            Next
          </Button>
        </div>
      )}
    </PageContainer>
  )
}

export default SysPodList
