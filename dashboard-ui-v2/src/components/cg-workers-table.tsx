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
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import {
  ModalForm,
  ProCard,
  ProForm,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components'
import {
  Button,
  Form,
  message,
  Popconfirm,
  Space,
  TablePaginationConfig,
  TableProps,
  Tooltip,
} from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import WorkerCacheBytes from './cache-bytes'
import WarmupModal from './warmup-modal'
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
  autoRefresh?: boolean
}> = ({ namespace, name, autoRefresh }) => {
  const [refreshInterval, setRefreshInterval] = React.useState<number>(0)
  const { data: nodes } = useNodes()
  const [, removeWorker] = useRemoveWorker(namespace, name)
  const [, addWorker] = useAddWorker(namespace, name)
  const [form] = Form.useForm<{ nodeName: string }>()

  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    total: 0,
  })
  const [filter, setFilter] = useState<{
    name?: string
    node?: string
  }>()

  const { data } = useCacheGroupWorkers(namespace, name, refreshInterval, {
    ...pagination,
    ...filter,
  })
  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])

  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }

  const [existNodes, setExistNodes] = React.useState<string[]>([])
  useEffect(() => {
    if (data?.items) {
      const nodes = data.items.map((v) => v.spec!.nodeName!)
      setExistNodes(nodes)
    }
  }, [data])

  useEffect(() => {
    autoRefresh && setRefreshInterval(1000)
  }, [autoRefresh])

  return (
    <ProCard title="workers">
      <ProTable
        toolbar={{
          settings: undefined,
          actions: [
            <ModalForm
              title="Select Node to add worker"
              trigger={
                <Button type="primary">
                  <PlusOutlined />
                  <FormattedMessage id="addWorker" />
                </Button>
              }
              form={form}
              autoFocusFirstInput
              modalProps={{
                destroyOnClose: true,
                className: 'add-worker-modal',
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
                    return {
                      label: v.metadata!.name,
                      value: v.metadata!.name,
                    }
                  })}
                />
              </ProForm.Group>
            </ModalForm>,
            data && data.items.length > 0 && (
              <WarmupModal
                name={data.items[0].metadata!.name!}
                namespace={data.items[0].metadata!.namespace!}
                container={data.items[0].status!.containerStatuses![0]}
              >
                {({ onClick }) => (
                  <Button onClick={onClick}>
                    <FormattedMessage id="warmup" />
                  </Button>
                )}
              </WarmupModal>
            ),
          ],
        }}
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
            title: <FormattedMessage id="cacheBytes" />,
            dataIndex: ['metadata', 'uid'],
            render: (_, row) => (
              <WorkerCacheBytes
                name={name}
                namespace={namespace}
                workerName={row.metadata?.name}
              />
            ),
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            render: (_, pod) => {
              if (
                pod.metadata.annotations?.['juicefs.io/waiting-delete-worker']
              ) {
                return <FormattedMessage id="dataMigration" />
              }

              if (pod.metadata.annotations?.['juicefs.io/backup-worker']) {
                return <FormattedMessage id="warmingUp" />
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
                <Tooltip title="remove worker">
                  <Popconfirm
                    title="Remove this worker?"
                    description={
                      <FormattedMessage id="removeWorkerDescription" />
                    }
                    onConfirm={async () => {
                      await removeWorker.execute({
                        nodeName: record.spec?.nodeName ?? '',
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
        dataSource={data?.items ?? []}
        rowKey={(c) => c.metadata?.uid || ''}
        pagination={pagination}
        onChange={handleTableChange}
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
    </ProCard>
  )
}

export default CgWorkersTable
