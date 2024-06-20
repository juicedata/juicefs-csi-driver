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
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import type { TablePaginationConfig, TableProps } from 'antd'
import { SortOrder } from 'antd/es/table/interface'
import { StorageClass } from 'kubernetes-types/storage/v1'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { useSCs } from '@/hooks/pv-api.ts'

const columns: ProColumns<StorageClass>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    render: (_, sc) => (
      <Link to={`/storageclass/${sc.metadata?.name}/`}>
        {' '}
        {sc.metadata?.name}{' '}
      </Link>
    ),
  },
  {
    title: <FormattedMessage id="reclaimPolicy" />,
    key: 'reclaimPolicy',
    search: false,
    dataIndex: ['reclaimPolicy'],
  },
  {
    title: <FormattedMessage id="allowVolumeExpansion" />,
    key: 'allowVolumeExpansion',
    search: false,
    render: (_, sc) => {
      if (sc.allowVolumeExpansion) {
        return (
          <div>
            <FormattedMessage id="true" />
          </div>
        )
      } else {
        return (
          <div>
            <FormattedMessage id="false" />
          </div>
        )
      }
    },
  },
  {
    title: <FormattedMessage id="createAt" />,
    key: 'time',
    sorter: 'time',
    search: false,
    render: (_, row) =>
      new Date(row.metadata?.creationTimestamp as string).toLocaleString(),
  },
]

const ScList: React.FC<unknown> = () => {
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    total: 0,
  })
  const [nameFilter, setNameFilter] = useState<string>('')
  const [sorter, setSorter] = useState<Record<string, SortOrder>>({
    time: 'ascend',
  })

  const { data, isLoading } = useSCs({
    current: pagination.current,
    pageSize: pagination.pageSize,
    name: nameFilter,
    sort: sorter,
  })

  const handleTableChange: TableProps['onChange'] = (pagination, _, sorter) => {
    setPagination(pagination)
    if (sorter instanceof Array) {
      setSorter({ time: sorter[0].order || 'ascend' })
    } else {
      setSorter({ time: sorter.order || 'ascend' })
    }
  }

  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])

  return (
    <PageContainer
      header={{
        title: <FormattedMessage id="scTablePageName" />,
      }}
    >
      <ProTable<StorageClass>
        headerTitle={<FormattedMessage id="scTableName" />}
        rowKey={(record) => record.metadata?.uid || ''}
        loading={isLoading}
        dataSource={data?.scs}
        columns={columns}
        onChange={handleTableChange}
        search={{
          optionRender: false,
          labelWidth: 120,
          collapsed: false,
        }}
        form={{
          onValuesChange: (_, values) => {
            setNameFilter(values.metadata?.name)
          },
        }}
        pagination={pagination}
      />
    </PageContainer>
  )
}

export default ScList
