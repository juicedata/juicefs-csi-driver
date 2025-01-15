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
import { AlertTwoTone } from '@ant-design/icons'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import {
  Badge,
  Button,
  Tooltip,
  type TablePaginationConfig,
  type TableProps,
} from 'antd'
import { SortOrder } from 'antd/es/table/interface'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { usePVs } from '@/hooks/pv-api.ts'
import { PV } from '@/types/k8s'
import { failedReasonOfPV, getPVStatusBadge } from '@/utils'

const accessModeMap: { [key: string]: string } = {
  ReadWriteOnce: 'RWO',
  ReadWriteMany: 'RWX',
  ReadOnlyMany: 'ROX',
  ReadWriteOncePod: 'RWOP',
}

const columns: ProColumns<PV>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    render: (_, pv) => {
      const pvFailReason = failedReasonOfPV(pv) || ''
      if (pvFailReason === '') {
        return (
          <Link to={`/pvs/${pv.metadata?.name}/`}>{pv.metadata?.name}</Link>
        )
      }
      const failReason = <FormattedMessage id={pvFailReason} />
      return (
        <div>
          <Link to={`/pvs/${pv.metadata?.name}/`}>{pv.metadata?.name}</Link>
          <Tooltip title={failReason}>
            <AlertTwoTone twoToneColor="#cf1322" />
          </Tooltip>
        </div>
      )
    },
  },
  {
    title: 'PVC',
    dataIndex: ['spec', 'claimRef', 'name'],
    render: (_, pv) => {
      if (!pv.spec?.claimRef) {
        return <span>-</span>
      } else {
        return (
          <Link
            to={`/pvcs/${pv.spec.claimRef.namespace}/${pv.spec.claimRef.name}`}
          >
            {pv.spec.claimRef.namespace}/{pv.spec.claimRef.name}
          </Link>
        )
      }
    },
  },
  {
    title: <FormattedMessage id="capacity" />,
    key: 'storage',
    search: false,
    dataIndex: ['spec', 'capacity', 'storage'],
  },
  {
    title: <FormattedMessage id="accessMode" />,
    key: 'accessModes',
    search: false,
    render: (_, pv) => {
      let accessModes: string
      if (pv.spec?.accessModes) {
        accessModes = pv.spec.accessModes
          .map((accessMode) => accessModeMap[accessMode] || 'Unknown')
          .join(',')
        return <div>{accessModes}</div>
      }
    },
  },
  {
    title: <FormattedMessage id="reclaimPolicy" />,
    key: 'persistentVolumeReclaimPolicy',
    search: false,
    dataIndex: ['spec', 'persistentVolumeReclaimPolicy'],
  },
  {
    title: 'StorageClass',
    dataIndex: ['spec', 'storageClassName'],
    render: (_, pv) => {
      if (pv.spec?.storageClassName) {
        return (
          <Link to={`/storageclass/${pv.spec?.storageClassName}/`}>
            {pv.spec?.storageClassName}
          </Link>
        )
      }
      return '-'
    },
  },
  {
    title: <FormattedMessage id="status" />,
    dataIndex: ['status', 'phase'],
    disable: true,
    search: false,
    filters: true,
    onFilter: true,
    render: (_, pv) => {
      return <Badge color={getPVStatusBadge(pv)} text={pv.status?.phase} />
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

const PVList: React.FC<unknown> = () => {
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    total: 0,
  })
  const [filter, setFilter] = useState<{
    name?: string
    pvc?: string
    sc?: string
    continue?: string
  }>()
  const [sorter, setSorter] = useState<Record<string, SortOrder>>({
    time: 'ascend',
  })

  const { data, isLoading } = usePVs({
    current: pagination.current,
    pageSize: pagination.pageSize,
    sort: sorter,
    ...filter,
  })

  const handleTableChange: TableProps['onChange'] = (pagination, _, sorter) => {
    setPagination(pagination)
    if (sorter instanceof Array) {
      setSorter({ time: sorter[0].order || 'ascend' })
    } else {
      setSorter({ time: sorter.order || 'ascend' })
    }
  }

  const [continueToken, setContinueToken] = useState<string | undefined>()
  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])

  useEffect(() => {
    setContinueToken(data?.continue)
  }, [data?.continue])

  return (
    <PageContainer
      header={{
        title: <FormattedMessage id="pvTablePageName" />,
      }}
    >
      <ProTable<PV>
        headerTitle={<FormattedMessage id="pvTableName" />}
        rowKey={(record) => record.metadata?.uid || ''}
        loading={isLoading}
        dataSource={data?.pvs}
        columns={columns}
        onChange={handleTableChange}
        search={{
          optionRender: false,
          labelWidth: 120,
          collapsed: false,
        }}
        form={{
          onValuesChange: (_, values) => {
            if (values) {
              setFilter((prev) => ({
                ...prev,
                ...values,
                ...values.metadata,
                pvc: values.spec?.claimRef.name,
                sc: values.spec?.storageClassName,
              }))
            }
          },
        }}
        pagination={data?.total ? pagination : false}
      />
      {continueToken && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'flex-end',
            marginTop: 16,
          }}
        >
          <Button
            onClick={() =>
              setFilter({
                ...filter,
                continue: continueToken,
              })
            }
          >
            Next
          </Button>
        </div>
      )}
    </PageContainer>
  )
}

export default PVList
