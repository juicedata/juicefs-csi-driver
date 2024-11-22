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

import React, { useEffect } from 'react'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import {
  ModalForm,
  ProCard,
  ProForm,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components'
import { Button, Form, message, Popconfirm, Space, Tooltip } from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import {
  useAddWorker,
  useCacheGroupWorkers,
  useRemoveWorker,
} from '@/hooks/cg-api'
import { useNodes } from '@/hooks/use-api'
import { getPodStatusBadge, podStatus } from '@/utils'

const CgWorkersTable: React.FC<{
  name?: string
  namespace?: string
}> = ({ namespace, name }) => {
  const [refreshInterval, setRefreshInterval] = React.useState<number>(0)
  const { data } = useCacheGroupWorkers(namespace, name, refreshInterval)
  const [sortedData, setSortedData] = React.useState(data)
  const { data: nodes } = useNodes(true)
  const [, removeWorker] = useRemoveWorker(namespace, name)
  const [, addWorker] = useAddWorker(namespace, name)
  const [form] = Form.useForm<{ nodeName: string }>()

  const [existNodes, setExistNodes] = React.useState<string[]>([])
  useEffect(() => {
    if (data) {
      const nodes = data.map((v) => v.spec!.nodeName!)
      setExistNodes(nodes)
      setSortedData(
        data.sort((a, b) => a.metadata!.name!.localeCompare(b.metadata!.name!)),
      )
    }
  }, [data])

  if (!data || data.length === 0) {
    return null
  }

  return (
    <ProCard title="workers">
      <ProTable
        toolbar={{
          actions: [
            <ModalForm
              trigger={
                <Button type="primary">
                  <PlusOutlined />
                  Add worker
                </Button>
              }
              form={form}
              autoFocusFirstInput
              modalProps={{
                destroyOnClose: true,
                onCancel: () => console.log('run'),
              }}
              submitTimeout={2000}
              onFinish={async (values) => {
                await addWorker.execute({ nodeName: values.nodeName })
                message.success('提交成功')
                setRefreshInterval(1000)
                return true
              }}
            >
              <ProForm.Group>
                <ProFormSelect
                  name="nodeName"
                  width="md"
                  options={nodes?.map((v) => {
                    if (existNodes.includes(v.metadata!.name!)) {
                      return {
                        label: v.metadata!.name,
                        value: v.metadata!.name,
                        disabled: true,
                      }
                    }
                    return { label: v.metadata!.name, value: v.metadata!.name }
                  })}
                />
              </ProForm.Group>
            </ModalForm>,
          ],
        }}
        search={false}
        columns={[
          {
            title: <FormattedMessage id="name" />,
            dataIndex: ['metadata', 'name'],
            render: (_, pod) => {
              return (
                <Link
                  to={`/syspods/${pod.metadata?.namespace}/${pod.metadata?.name}`}
                >
                  {pod.metadata?.name}
                </Link>
              )
            },
          },
          {
            title: <FormattedMessage id="node" />,
            key: 'node',
            dataIndex: ['spec', 'nodeName'],
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            render: (_, pod) => {
              if (
                pod.metadata.annotations?.['juicefs.io/waiting-delete-worker']
              ) {
                return 'Waiting Delete'
              }

              if (pod.metadata.annotations?.['juicefs.io/backup-worker']) {
                return 'Backup Worker'
              }

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
            render: (_, row) =>
              new Date(row.metadata.creationTimestamp).toLocaleString(),
          },
          {
            title: 'Action',
            key: 'action',
            render: (_, record) => (
              <Space>
                <Tooltip title="scale down worker">
                  <Popconfirm
                    title="Remove this worker?"
                    description={
                      <FormattedMessage id="removeWorkerDescription" />
                    }
                    onConfirm={async () => {
                      await removeWorker.execute({
                        nodeName: record.spec!.nodeName!,
                      })
                      message.success('提交成功')
                      setRefreshInterval(1000)
                    }}
                    okText="Yes"
                    cancelText="No"
                  >
                    <Button
                      className="action-button"
                      icon={<DeleteOutlined />}
                    />
                  </Popconfirm>
                </Tooltip>
              </Space>
            ),
          },
        ]}
        dataSource={sortedData}
        rowKey={(c) => c.metadata?.uid || ''}
        pagination={false}
      />
    </ProCard>
  )
}

export default CgWorkersTable
