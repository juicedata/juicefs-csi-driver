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

import React, { memo, useState } from 'react'
import {
  PageContainer,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components'
import {
  Badge,
  Button,
  Collapse,
  Alert,
  message,
  Popconfirm,
  Space,
  Table,
  Tooltip,
} from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link, useNavigate } from 'react-router-dom'
import useSWR from 'swr'

import { EventTable } from '@/components'
import CacheGroupDrawer from '@/components/cache-group-drawer'
import CacheGroupStatus from '@/components/cache-group-status'
import CgWorkersTable from '@/components/cg-workers-table'
import {
  useCacheGroup,
  useCacheGroups,
  useDeleteCacheGroup,
  useUpdateCacheGroup,
  warmupCacheGroup,
} from '@/hooks/cg-api'
import { Warmup } from '@/types/k8s'

const CgDetail: React.FC<{
  name?: string
  namespace?: string
}> = memo((props) => {
  const { name, namespace } = props
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [autoRefreshWorkers, setAutoRefreshWorkers] = useState(false)
  const redirect = useNavigate()

  const { data, isLoading, mutate } = useCacheGroup(namespace, name, !isModalOpen)
  const { data: groups, error: groupsError } = useCacheGroups()
  const { data: warmups, error: warmupsError, isLoading: warmupsLoading } =
    useSWR<Warmup[]>(namespace && name ? '/api/v1/warmups' : null, {
      refreshInterval: 10000,
    })
  const [, updateCg] = useUpdateCacheGroup()
  const [, deleteCg] = useDeleteCacheGroup()

  if (namespace === '' || name === '' || !data) {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="cacheGroupNotFound" />,
        }}
      ></PageContainer>
    )
  }

  const sampleTimes = (data.status?.stats ?? [])
    .filter(
      (stat) => stat.cacheBytes !== undefined || stat.capacityBytes !== undefined,
    )
    .flatMap((stat) =>
      stat.lastSampleTime ? [new Date(stat.lastSampleTime).getTime()] : [],
    )
  const oldestSampleTime =
    sampleTimes.length > 0
      ? new Date(Math.min(...sampleTimes)).toLocaleString()
      : '-'
  const rebalance = data.status?.rebalance
  const rebalanceMembers = rebalance?.members || []
  const rebalanceStart = rebalance?.startAt
    ? new Date(rebalance.startAt)
    : undefined
  const memberList = (
    <Space direction="vertical" size={4}>
      {rebalanceMembers.map((member) => (
        <Link
          key={member}
          to={`/syspods/${encodeURIComponent(namespace || '')}/${encodeURIComponent(member)}`}
        >
          {member}
        </Link>
      ))}
    </Space>
  )
  const memberLabel = (
    <FormattedMessage
      id="rebalanceMembers"
      values={{ count: rebalanceMembers.length }}
    />
  )

  return (
    <PageContainer
      fixedHeader
      loading={isLoading}
      header={{
        title: name,
        subTitle: namespace,
      }}
    >
      <ProCard
        title={<FormattedMessage id="basic" />}
        extra={
          <Space>
            <Button onClick={() => setIsModalOpen(true)}>
              <FormattedMessage id="edit" />
            </Button>
            {isModalOpen && <CacheGroupDrawer initialGroup={data} onClose={() => setIsModalOpen(false)} onSave={async (body) => {
              await updateCg.execute({ body })
              await mutate()
              message.success(<FormattedMessage id="storageSaved" />)
              setAutoRefreshWorkers(true)
            }} />}
            <Popconfirm
              title={<FormattedMessage id="cacheGroupDeleteTitle" />}
              description={
                <FormattedMessage id="deleteCacheGroupDescription" />
              }
              onConfirm={async () => {
                try {
                  await deleteCg.execute({
                    body: data,
                  })
                } catch (err) {
                  message.error(
                    (JSON.parse((err as Error).message) as { error?: string })
                      .error || String(err),
                  )
                  return
                }
                message.success(<FormattedMessage id="storageDeleted" />)
                redirect('/cachegroups')
              }}
              okText={<FormattedMessage id="yes" />}
              cancelText={<FormattedMessage id="no" />}
            >
              <Button type="primary" danger>
                <FormattedMessage id="warmupDelete" />
              </Button>
            </Popconfirm>
          </Space>
        }
      >
        <ProDescriptions
          column={2}
          dataSource={data}
          columns={[
            {
              title: <FormattedMessage id="namespace" />,
              dataIndex: ['metadata', 'namespace'],
            },
            {
              title: <FormattedMessage id="status" />,
              dataIndex: ['status', 'phase'],
              render: () => <CacheGroupStatus value={data.status?.phase} />,
            },
            {
              title: <FormattedMessage id="expectWorker" />,
              dataIndex: ['status', 'expectWorker'],
            },
            {
              title: <FormattedMessage id="readyWorker" />,
              dataIndex: ['status', 'readyWorker'],
            },
            {
              title: <FormattedMessage id="cacheGroupName" />,
              dataIndex: ['status', 'cacheGroup'],
            },
            {
              title: <FormattedMessage id="cacheBytes" />,
              render: () => (
                <Tooltip
                  title={
                    <FormattedMessage
                      id="cacheTotalSampleTime"
                      values={{ time: oldestSampleTime }}
                    />
                  }
                >
                  <span
                    tabIndex={0}
                    style={{ whiteSpace: 'nowrap', cursor: 'help' }}
                  >
                    {data.status?.cacheSize || '-'} / {data.status?.cacheCapacity || '-'}
                  </span>
                </Tooltip>
              ),
            },
          ]}
        />
      </ProCard>
      {rebalance && (
        <ProCard title={<FormattedMessage id="rebalanceTitle" />}>
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Badge
              status="processing"
              text={<FormattedMessage id="rebalanceInProgress" />}
            />
            <span>
              <FormattedMessage id="rebalanceExpectedStart" />：
              {rebalanceStart && !Number.isNaN(rebalanceStart.getTime())
                ? rebalanceStart.toLocaleString()
                : '-'}
            </span>
            {rebalanceMembers.length > 3 ? (
              <Collapse
                size="small"
                items={[
                  { key: 'members', label: memberLabel, children: memberList },
                ]}
              />
            ) : (
              <Space direction="vertical" size={4}>
                {memberLabel}
                {memberList}
              </Space>
            )}
          </Space>
        </ProCard>
      )}
      <CgWorkersTable
        name={name}
        namespace={namespace}
        nodeSelector={data.spec.worker.template.nodeSelector}
        stats={data.status?.stats}
        autoRefresh={autoRefreshWorkers}
      />
      <ProCard title={<FormattedMessage id="warmupTasks" />}>
        {warmupsError && (
          <Alert type="error" showIcon message={<FormattedMessage id="warmupListError" />} />
        )}
        {groupsError && (
          <Alert type="error" showIcon message={<FormattedMessage id="warmupGroupsError" />} />
        )}
        <Table<Warmup>
          locale={{ emptyText: <FormattedMessage id="noRecords" /> }}
          rowKey={(task) => task.metadata.uid || `${task.metadata.namespace}/${task.metadata.name}`}
          loading={warmupsLoading || (!groups && !groupsError)}
          dataSource={(warmups || []).filter((task) => {
            const group = warmupCacheGroup(task, groups || [])
            return group?.metadata?.namespace === namespace && group?.metadata?.name === name
          })}
          columns={[
            {
              title: <FormattedMessage id="name" />,
              render: (_, task) => (
                <Link to={`/warmups/${encodeURIComponent(task.metadata.namespace)}/${encodeURIComponent(task.metadata.name)}`}>
                  {task.metadata.name}
                </Link>
              ),
            },
            {
              title: <FormattedMessage id="phase" />,
              dataIndex: ['status', 'phase'],
              render: (_, task) => <CacheGroupStatus value={task.status?.phase} />,
            },
            { title: <FormattedMessage id="warmupProgress" />, dataIndex: ['status', 'progress'] },
            {
              title: <FormattedMessage id="createAt" />,
              render: (_, task) => task.metadata.creationTimestamp
                ? new Date(task.metadata.creationTimestamp).toLocaleString()
                : '-',
            },
          ]}
          pagination={{ pageSize: 5, hideOnSinglePage: true }}
        />
      </ProCard>
      <EventTable source="cachegroup" name={name!} namespace={namespace!} />
    </PageContainer>
  )
})

export default CgDetail
