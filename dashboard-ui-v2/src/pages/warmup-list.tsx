/*
 * Copyright 2026 Juicedata Inc
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

import { useState } from 'react'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import { Alert, Button } from 'antd'
import { useIntl } from 'react-intl'
import { Link } from 'react-router-dom'
import useSWR from 'swr'

import WarmupDrawer from '@/components/warmup-drawer'
import { useCacheGroups, warmupCacheGroup } from '@/hooks/cg-api'
import { Warmup } from '@/types/k8s'

export default function WarmupList() {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const { data, error, isLoading } = useSWR<Warmup[]>('/api/v1/warmups', {
    refreshInterval: 10000,
  })
  const [open, setOpen] = useState(false)
  const { data: groups } = useCacheGroups()

  const columns: ProColumns<Warmup>[] = [
    {
      title: t('name'),
      dataIndex: ['metadata', 'name'],
      render: (_, row) => (
        <Link to={`/warmups/${row.metadata.namespace}/${row.metadata.name}`}>
          {row.metadata.name}
        </Link>
      ),
    },
    { title: t('namespace'), dataIndex: ['metadata', 'namespace'] },
    {
      title: t('cacheGroup'),
      dataIndex: ['spec', 'cacheGroupName'],
      render: (_, row) => {
        const group = warmupCacheGroup(row, groups || [])
        return group?.metadata?.name ? (
          <Link to={`/cachegroups/${encodeURIComponent(row.metadata.namespace)}/${encodeURIComponent(group.metadata.name)}`}>
            {row.spec.cacheGroupName}
          </Link>
        ) : row.spec.cacheGroupName || '-'
      },
    },
    { title: t('phase'), dataIndex: ['status', 'phase'] },
    { title: t('warmupProgress'), dataIndex: ['status', 'progress'] },
    {
      title: t('warmupScanned'),
      dataIndex: ['status', 'stats', 'scannedData'],
    },
    {
      title: t('warmupCompleted'),
      dataIndex: ['status', 'stats', 'completedData'],
    },
    { title: t('warmupDuration'), dataIndex: ['status', 'duration'] },
    {
      title: t('createAt'),
      render: (_, row) =>
        row.metadata.creationTimestamp
          ? new Date(row.metadata.creationTimestamp).toLocaleString()
          : '-',
    },
  ]

  return (
    <PageContainer header={{ title: t('warmupTasks') }}>
      {error && (
        <Alert
          type="error"
          showIcon
          message={t('warmupListError')}
          description={String(error)}
        />
      )}
      <ProTable<Warmup>
        rowKey={(row) =>
          row.metadata.uid || `${row.metadata.namespace}/${row.metadata.name}`
        }
        loading={isLoading}
        dataSource={data || []}
        columns={columns}
        search={false}
        toolbar={{
          actions: [
            <Button key="create" type="primary" onClick={() => setOpen(true)}>
              {t('warmupCreate')}
            </Button>,
          ],
          settings: undefined,
        }}
      />
      {open && <WarmupDrawer onClose={() => setOpen(false)} />}
    </PageContainer>
  )
}
