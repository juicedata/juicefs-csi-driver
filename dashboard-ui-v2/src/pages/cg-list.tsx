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

import React, { useState } from 'react'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import { Button, message } from 'antd'
import { FormattedMessage, useIntl } from 'react-intl'
import { Link } from 'react-router-dom'

import CacheGroupDrawer from '@/components/cache-group-drawer'
import CacheGroupStatus from '@/components/cache-group-status'
import { useCacheGroups, useCreateCacheGroup } from '@/hooks/cg-api'
import { CacheGroup } from '@/types/k8s'

const columns: ProColumns<CacheGroup>[] = [
  {
    title: <><FormattedMessage id="namespace" /> / <FormattedMessage id="name" /></>,
    dataIndex: ['metadata', 'name'],
    render: (_, cg) => (
      <Link to={`/cachegroups/${cg.metadata?.namespace}/${cg.metadata?.name}`}>
        {cg.metadata?.namespace}/{cg.metadata?.name}
      </Link>
    ),
  },
  {
    title: <FormattedMessage id="cacheGroup" />,
    dataIndex: ['status', 'cacheGroup'],
  },
  {
    title: <FormattedMessage id="phase" />,
    dataIndex: ['status', 'phase'],
    render: (_, cg) => <CacheGroupStatus value={cg.status?.phase} />,
  },
  {
    title: <FormattedMessage id="cacheGroupBackUp" />,
    dataIndex: ['status', 'backUpWorker'],
  },
  {
    title: <FormattedMessage id="cacheGroupWaitingDeleted" />,
    dataIndex: ['status', 'waitingDeletedWorker'],
  },
  {
    title: <FormattedMessage id="cacheGroupSize" />,
    dataIndex: ['status', 'cacheSize'],
  },
  {
    title: <FormattedMessage id="cacheGroupCapacity" />,
    dataIndex: ['status', 'cacheCapacity'],
  },
  {
    title: <FormattedMessage id="ready" />,
    dataIndex: ['status', 'readyStr'],
  },
  {
    title: <FormattedMessage id="cacheGroupAge" />,
    render: (_, row) => {
      if (!row.metadata?.creationTimestamp) return <FormattedMessage id="cacheGroupUnknownAge" />
      const seconds = Math.trunc(
        (Date.now() - new Date(row.metadata.creationTimestamp).getTime()) /
          1000,
      )
      if (seconds < -1) return <FormattedMessage id="cacheGroupInvalidAge" />
      if (seconds < 0) return '0s'
      if (seconds < 120) return `${seconds}s`
      const minutes = Math.floor(seconds / 60)
      if (minutes < 10)
        return `${minutes}m${seconds % 60 ? `${seconds % 60}s` : ''}`
      if (minutes < 180) return `${minutes}m`
      const hours = Math.floor(minutes / 60)
      if (hours < 8) return `${hours}h${minutes % 60 ? `${minutes % 60}m` : ''}`
      if (hours < 48) return `${hours}h`
      const days = Math.floor(hours / 24)
      if (hours < 192) return `${days}d${hours % 24 ? `${hours % 24}h` : ''}`
      if (days < 730) return `${days}d`
      const years = Math.floor(days / 365)
      if (years < 8) return `${years}y${days % 365 ? `${days % 365}d` : ''}`
      return `${years}y`
    },
  },
]

const CgList: React.FC<unknown> = () => {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const { data, isLoading, mutate } = useCacheGroups()
  const [, createCg] = useCreateCacheGroup()
  const [open, setOpen] = useState(false)

  return (
    <PageContainer header={{ title: t('cacheGroupsTitle') }}>
      <ProTable<CacheGroup>
        locale={{ emptyText: <FormattedMessage id="noRecords" /> }}
        rowKey={(record) => record.metadata?.uid || ''}
        loading={isLoading}
        dataSource={data}
        columns={columns}
        columnEmptyText=""
        search={false}
        toolbar={{
          actions: [
            <Button key="create" type="primary" onClick={() => setOpen(true)}>
              {t('cacheGroupCreate')}
            </Button>,
          ],
          settings: undefined,
        }}
      />
      {open && (
        <CacheGroupDrawer
          onClose={() => setOpen(false)}
          onSave={async (body) => {
            await createCg.execute({ body })
            await mutate()
            message.success(t('cacheGroupCreated'))
          }}
        />
      )}
    </PageContainer>
  )
}

export default CgList
