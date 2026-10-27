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

import React, { useEffect, useMemo, useState } from 'react'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import {
  ModalForm,
  ProCard,
  ProForm,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components'
import {
  Badge,
  Button,
  Form,
  Input,
  message,
  Popconfirm,
  Select,
  Space,
  Switch,
  TablePaginationConfig,
  TableProps,
  Tooltip,
} from 'antd'
import { SortOrder } from 'antd/es/table/interface'
import { Pod as NativePod } from 'kubernetes-types/core/v1'
import { FormattedMessage, useIntl } from 'react-intl'
import { Link } from 'react-router-dom'

import WorkerCacheBytes from './cache-bytes'
import WarmupModal from './warmup-modal'
import {
  useAddWorker,
  useCacheGroupWorkers,
  useRemoveWorker,
} from '@/hooks/cg-api'
import { useNodes } from '@/hooks/use-api'
import { CacheGroup } from '@/types/k8s'
import { getPodStatusBadge, podStatus } from '@/utils'

// Returns the tag of a Docker image reference; '' means no explicit tag,
// undefined means no image. Registry ports and digests are not treated as tags.
const imageTag = (image?: string): string | undefined => {
  if (!image) return undefined
  const name = image.split('@', 1)[0]
  const lastPart = name.slice(name.lastIndexOf('/') + 1)
  const colon = lastPart.lastIndexOf(':')
  if (colon === -1) return ''
  return lastPart.slice(colon + 1)
}

const workerRowKey = (worker: NativePod) =>
  worker.metadata?.uid || worker.metadata?.name || ''

const getSortBy = (sortKey: unknown) => {
  const key = Array.isArray(sortKey) ? sortKey.join('.') : String(sortKey)
  switch (key) {
    case 'name':
    case 'metadata.name':
    case 'metadata,name':
      return 'name'
    case 'node':
    case 'spec.nodeName':
    case 'spec,nodeName':
      return 'node'
    default:
      return 'time'
  }
}

type WorkerMutationResult = {
  succeeded: string[]
  failed: Array<{ nodeName: string; reason: string }>
}

const CgWorkersTable: React.FC<{
  name?: string
  namespace?: string
  nodeSelector?: Record<string, string>
  stats?: NonNullable<CacheGroup['status']>['stats']
  autoRefresh?: boolean
}> = ({ namespace, name, nodeSelector, stats, autoRefresh }) => {
  const intl = useIntl()
  const [refreshInterval, setRefreshInterval] = useState(0)
  const { data: nodes, mutate: mutateNodes } = useNodes()
  const [, removeWorker] = useRemoveWorker(namespace, name)
  const [, addWorker] = useAddWorker(namespace, name)
  const [form] = Form.useForm<{ nodeNames: string[] }>()
  const selectedAddNodeNames = Form.useWatch('nodeNames', form) ?? []
  const [searchForm] = Form.useForm<{
    name?: string
    node?: string
    unready?: boolean
  }>()
  const searchName = Form.useWatch('name', searchForm)
  const searchNode = Form.useWatch('node', searchForm)
  const searchUnready = Form.useWatch('unready', searchForm)

  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    total: 0,
  })
  const [filter, setFilter] = useState<{
    name?: string
    node?: string
    unready?: boolean
  }>()
  const [sorter, setSorter] = useState<Record<string, SortOrder>>({
    time: 'ascend',
  })
  const [selectedWorkers, setSelectedWorkers] = useState<Map<string, string>>(
    new Map(),
  )
  const selectedRowKeys = [...selectedWorkers.keys()]
  const selectedWorkerNodeNames = [...new Set(selectedWorkers.values())]
  const workerStats = useMemo(
    () => new Map(stats?.map((stat) => [stat.name, stat])),
    [stats],
  )

  const {
    data,
    isLoading,
    mutate: mutateWorkers,
  } = useCacheGroupWorkers(namespace, name, refreshInterval, {
    current: pagination.current,
    pageSize: pagination.pageSize,
    sort: sorter,
    ...filter,
  })

  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])

  useEffect(() => {
    if (autoRefresh) {
      setRefreshInterval(1000)
    }
  }, [autoRefresh])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setPagination((prev) => ({ ...prev, current: 1 }))
      setFilter((prev) => ({
        ...prev,
        name: searchName?.trim() || undefined,
      }))
    }, searchName?.trim() ? 300 : 0)
    return () => window.clearTimeout(timer)
  }, [searchName])

  useEffect(() => {
    setPagination((prev) => ({ ...prev, current: 1 }))
    setFilter((prev) => ({
      ...prev,
      node: searchNode || undefined,
      unready: searchUnready || undefined,
    }))
  }, [searchNode, searchUnready])

  const nodeOptions = useMemo(
    () =>
      [...(nodes ?? [])]
        .sort((left, right) =>
          (left.metadata?.name ?? '').localeCompare(
            right.metadata?.name ?? '',
          ),
        )
        .map((node) => {
          const nodeName = node.metadata?.name ?? ''
          const labels = node.metadata?.labels ?? {}
          const disabled =
            nodeSelector !== undefined &&
            Object.entries(nodeSelector).every(
              ([key, value]) => labels[key] === value,
            )
          return { label: nodeName, value: nodeName, disabled }
        }),
    [nodeSelector, nodes],
  )
  const searchNodeOptions = useMemo(
    () => nodeOptions.map(({ label, value }) => ({ label, value })),
    [nodeOptions],
  )

  const refreshData = async () => {
    await Promise.allSettled([mutateWorkers(), mutateNodes()])
  }

  const showResult = (result: WorkerMutationResult) => {
    if (result.failed.length === 0) {
      message.success(
        intl.formatMessage(
          { id: 'workerBatchSuccess' },
          { count: result.succeeded.length },
        ),
      )
    } else if (result.succeeded.length > 0) {
      message.warning(
        intl.formatMessage(
          { id: 'workerBatchPartial' },
          {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
          },
        ),
      )
    } else {
      message.error(
        intl.formatMessage(
          { id: 'workerBatchFailed' },
          { count: result.failed.length },
        ),
      )
    }
  }

  const toolbarSearch = (
    <Form form={searchForm} layout="inline">
      <Form.Item name="name" style={{ marginBottom: 0 }}>
        <Input
          allowClear
          placeholder={intl.formatMessage({ id: 'name' })}
          style={{ width: 140 }}
        />
      </Form.Item>
      <Form.Item name="node" style={{ marginBottom: 0 }}>
        <Select
          allowClear
          showSearch
          aria-label={intl.formatMessage({ id: 'node' })}
          placeholder={intl.formatMessage({ id: 'node' })}
          options={searchNodeOptions}
          filterOption={(input, option) =>
            String(option?.label ?? '')
              .toLowerCase()
              .includes(input.toLowerCase())
          }
          style={{ width: 160 }}
        />
      </Form.Item>
      <Form.Item style={{ marginBottom: 0 }}>
        <Space size={8}>
          <FormattedMessage id="notReady" />
          <Form.Item name="unready" valuePropName="checked" noStyle>
            <Switch aria-label={intl.formatMessage({ id: 'notReady' })} />
          </Form.Item>
        </Space>
      </Form.Item>
    </Form>
  )

  const handleTableChange: TableProps<NativePod>['onChange'] = (
    nextPagination,
    _,
    nextSorter,
  ) => {
    setPagination(nextPagination)
    const currentSorter = Array.isArray(nextSorter)
      ? nextSorter[0]
      : nextSorter
    if (!currentSorter.order) {
      setSorter({ time: 'ascend' })
      return
    }
    const sortBy = getSortBy(currentSorter.columnKey || currentSorter.field)
    setSorter({ [sortBy]: currentSorter.order })
  }

  return (
    <ProCard title="workers">
      <ProTable<NativePod>
        loading={isLoading}
        search={false}
        toolbar={{
          settings: undefined,
          search: toolbarSearch,
          actions: [
            <ModalForm<{ nodeNames: string[] }>
              key="add-worker"
              title={intl.formatMessage({ id: 'addWorkerTitle' })}
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
                afterClose: () => form.resetFields(),
              }}
              submitter={{
                submitButtonProps: {
                  disabled: selectedAddNodeNames.length === 0,
                },
              }}
              onFinish={async ({ nodeNames }) => {
                const result = (await addWorker.execute({
                  nodeNames,
                })) as WorkerMutationResult
                showResult(result)
                if (result.succeeded.length > 0) {
                  setRefreshInterval(1000)
                }
                if (result.failed.length === 0) {
                  form.resetFields()
                  await refreshData()
                  return true
                }
                form.setFieldsValue({
                  nodeNames: result.failed.map(({ nodeName }) => nodeName),
                })
                await refreshData()
                return false
              }}
            >
              <ProForm.Group>
                <ProFormSelect
                  name="nodeNames"
                  width="md"
                  mode="multiple"
                  showSearch
                  placeholder={intl.formatMessage({
                    id: 'selectNodesPlaceholder',
                  })}
                  options={nodeOptions}
                  fieldProps={{
                    filterOption: (input, option) =>
                      String(option?.label ?? '')
                        .toLowerCase()
                        .includes(input.toLowerCase()),
                  }}
                  rules={[{ required: true }]}
                />
              </ProForm.Group>
            </ModalForm>,
            selectedRowKeys.length > 0 && (
              <Popconfirm
                key="remove-selected-workers"
                title={intl.formatMessage({
                  id: 'removeSelectedWorkersConfirm',
                })}
                onConfirm={async () => {
                  const result = (await removeWorker.execute({
                    nodeNames: selectedWorkerNodeNames,
                  })) as WorkerMutationResult
                  const succeededNodeNames = new Set(result.succeeded)
                  setSelectedWorkers(
                    (workers) =>
                      new Map(
                        [...workers].filter(
                          ([, nodeName]) => !succeededNodeNames.has(nodeName),
                        ),
                      ),
                  )
                  showResult(result)
                  if (result.succeeded.length > 0) {
                    setRefreshInterval(1000)
                  }
                  await refreshData()
                }}
                okText={intl.formatMessage({ id: 'yes' })}
                cancelText={intl.formatMessage({ id: 'no' })}
              >
                <Button danger>
                  <FormattedMessage
                    id="removeSelectedWorkers"
                    values={{ count: selectedRowKeys.length }}
                  />
                </Button>
              </Popconfirm>
            ),
            <WarmupModal
              key="warmup"
              name={data?.items?.[0]?.metadata?.name ?? ''}
              namespace={data?.items?.[0]?.metadata?.namespace ?? ''}
              container={data?.items?.[0]?.status?.containerStatuses?.[0]}
            >
              {({ onClick }) => (
                <Button onClick={onClick} disabled={!data?.items?.length}>
                  <FormattedMessage id="warmup" />
                </Button>
              )}
            </WarmupModal>,
          ],
        }}
        columns={[
          {
            title: <FormattedMessage id="name" />,
            key: 'name',
            sorter: 'name',
            hideInSearch: true,
            dataIndex: ['metadata', 'name'],
            render: (_, pod) => (
              <Link
                to={`/syspods/${pod.metadata?.namespace}/${pod.metadata?.name}`}
              >
                {pod.metadata?.name}
              </Link>
            ),
          },
          {
            title: <FormattedMessage id="node" />,
            key: 'node',
            sorter: 'node',
            hideInSearch: true,
            dataIndex: ['spec', 'nodeName'],
          },
          {
            title: <FormattedMessage id="version" />,
            key: 'version',
            hideInSearch: true,
            render: (_, pod) => {
              const image = pod.spec?.containers?.[0]?.image
              const tag = imageTag(image)
              if (tag === undefined) return '-'
              return <Tooltip title={image}>{tag || 'latest'}</Tooltip>
            },
          },
          {
            title: <FormattedMessage id="cacheBytes" />,
            hideInSearch: true,
            dataIndex: ['metadata', 'uid'],
            render: (_, row) => (
              <span style={{ whiteSpace: 'nowrap' }}>
                <WorkerCacheBytes
                  bytes={workerStats.get(row.metadata?.name ?? '')?.cacheBytes}
                />{' / '}
                <WorkerCacheBytes
                  bytes={workerStats.get(row.metadata?.name ?? '')?.capacityBytes}
                />
              </span>
            ),
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            hideInSearch: true,
            render: (_, pod) => {
              if (
                pod.metadata?.annotations?.['juicefs.io/waiting-delete-worker']
              ) {
                return (
                  <Tooltip title={<FormattedMessage id="dataMigrationDescription" />}>
                    <span tabIndex={0} style={{ cursor: 'help' }}>
                      <Badge
                        color="yellow"
                        text={<FormattedMessage id="dataMigration" />}
                      />
                    </span>
                  </Tooltip>
                )
              }
              if (pod.metadata?.annotations?.['juicefs.io/backup-worker']) {
                return (
                  <Tooltip title={<FormattedMessage id="joiningDescription" />}>
                    <span tabIndex={0} style={{ cursor: 'help' }}>
                      <Badge
                        color="yellow"
                        text={<FormattedMessage id="joining" />}
                      />
                    </span>
                  </Tooltip>
                )
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
            key: 'time',
            sorter: 'time',
            dataIndex: ['metadata', 'creationTimestamp'],
            hideInSearch: true,
            render: (_, row) =>
              new Date(row.metadata?.creationTimestamp as string).toLocaleString(),
          },
          {
            title: <FormattedMessage id="action" />,
            key: 'action',
            hideInSearch: true,
            render: (_, record) => {
              const nodeName = record.spec?.nodeName ?? ''
              return (
                <Space>
                  <Tooltip
                    title={intl.formatMessage({ id: 'removeWorker' })}
                  >
                    <Popconfirm
                      title={intl.formatMessage({ id: 'removeWorkerConfirm' })}
                      description={
                        <FormattedMessage id="removeWorkerDescription" />
                      }
                      onConfirm={async () => {
                        const result = (await removeWorker.execute({
                          nodeNames: [nodeName],
                        })) as WorkerMutationResult
                        if (result.succeeded.length > 0) {
                          setSelectedWorkers(
                            (workers) =>
                              new Map(
                                [...workers].filter(
                                  ([, selectedNodeName]) =>
                                    selectedNodeName !== nodeName,
                                ),
                              ),
                          )
                          setRefreshInterval(1000)
                        }
                        showResult(result)
                        await refreshData()
                      }}
                      okText={intl.formatMessage({ id: 'yes' })}
                      cancelText={intl.formatMessage({ id: 'no' })}
                    >
                      <Button
                        className="action-button"
                        icon={<DeleteOutlined />}
                      />
                    </Popconfirm>
                  </Tooltip>
                </Space>
              )
            },
          },
        ]}
        dataSource={data?.items ?? []}
        rowKey={workerRowKey}
        rowSelection={{
          selectedRowKeys,
          preserveSelectedRowKeys: true,
          onChange: (keys, workers) => {
            const selectedKeys = new Set(keys.map(String))
            setSelectedWorkers((previous) => {
              const next = new Map(
                [...previous].filter(([key]) => selectedKeys.has(key)),
              )
              workers.forEach((worker) =>
                next.set(
                  workerRowKey(worker),
                  worker.spec?.nodeName ?? '',
                ),
              )
              return next
            })
          },
        }}
        pagination={pagination}
        onChange={handleTableChange}
      />
    </ProCard>
  )
}

export default CgWorkersTable
