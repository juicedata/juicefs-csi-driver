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

import React from 'react'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { useCacheGroups } from '@/hooks/cg-api'
import { CacheGroup } from '@/types/k8s'

const columns: ProColumns<CacheGroup>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    render: (_, cg) => (
      <Link to={`/cachegroups/${cg.metadata?.namespace}/${cg.metadata?.name}`}>
        {cg.metadata?.name}
      </Link>
    ),
  },
  {
    title: <FormattedMessage id="namespace" />,
    dataIndex: ['metadata', 'namespace'],
  },
  {
    title: <FormattedMessage id="phase" />,
    dataIndex: ['status', 'phase'],
  },
  {
    title: <FormattedMessage id="ready" />,
    dataIndex: ['status', 'readyStr'],
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

const CgList: React.FC<unknown> = () => {
  const { data, isLoading } = useCacheGroups()
  return (
    <PageContainer
      header={{
        title: 'Cache Groups',
      }}
    >
      <ProTable<CacheGroup>
        rowKey={(record) => record.metadata?.uid || ''}
        loading={isLoading}
        dataSource={data}
        columns={columns}
      />
    </PageContainer>
  )
}

export default CgList
