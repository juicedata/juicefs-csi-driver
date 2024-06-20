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
import { ProCard } from '@ant-design/pro-components'
import { Table } from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { usePods } from '@/hooks/use-api'
import { getPodStatusBadge, podStatus } from '@/utils'

const PodsTable: React.FC<{
  title: string
  type: 'mountpods' | 'apppods'
  namespace: string
  name: string
}> = ({ title, type, namespace, name }) => {
  const { data } = usePods(namespace, name, type)
  if (!data || data.length === 0) {
    return null
  }
  return (
    <ProCard title={title}>
      <Table
        columns={[
          {
            title: <FormattedMessage id="name" />,
            dataIndex: ['metadata', 'name'],
            render: (_, pod) => {
              return (
                <Link
                  to={`/pods/${pod.metadata?.namespace}/${pod.metadata?.name}`}
                >
                  {pod.metadata?.namespace}/{pod.metadata?.name}
                </Link>
              )
            },
          },
          {
            title: <FormattedMessage id="namespace" />,
            key: 'namespace',
            dataIndex: ['metadata', 'namespace'],
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
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
            title: <FormattedMessage id="startAt" />,
            dataIndex: ['metadata', 'creationTimestamp'],
            render: (startAt) => new Date(startAt).toLocaleString(),
          },
        ]}
        dataSource={data}
        rowKey={(c) => c.metadata?.uid || ''}
        pagination={false}
      />
    </ProCard>
  )
}

export default PodsTable
