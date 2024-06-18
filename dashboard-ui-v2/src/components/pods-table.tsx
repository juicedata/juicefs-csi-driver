import React from 'react'
import { ProCard } from '@ant-design/pro-components'
import { Table, Tag } from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { usePods } from '@/hooks/use-api'
import { getPodStatusBadge, podStatus } from '@/utils'
import { PodStatusEnum } from '@/types/k8s.ts'
import { Badge } from 'antd/lib'

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
