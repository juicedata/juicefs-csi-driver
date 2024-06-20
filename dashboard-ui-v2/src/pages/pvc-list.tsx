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
import { Tooltip, type TablePaginationConfig, type TableProps } from 'antd'
import { SortOrder } from 'antd/es/table/interface'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { usePVCs } from '@/hooks/pv-api.ts'
import { PVC } from '@/types/k8s'
import { failedReasonOfPVC, getPVCStatusBadge } from '@/utils'

const accessModeMap: { [key: string]: string } = {
  ReadWriteOnce: 'RWO',
  ReadWriteMany: 'RWX',
  ReadOnlyMany: 'ROX',
  ReadWriteOncePod: 'RWOP',
}
const columns: ProColumns<PVC>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    formItemProps: {
      rules: [
        {
          required: true,
          message: '名称为必填项',
        },
      ],
    },
    render: (_, pvc) => {
      const pvcFailReason = failedReasonOfPVC(pvc)
      if (pvcFailReason === '') {
        return (
          <Link to={`/pvcs/${pvc.metadata?.namespace}/${pvc.metadata?.name}`}>
            {pvc.metadata?.name}
          </Link>
        )
      }
      const failReason = <FormattedMessage id={pvcFailReason} />
      return (
        <div>
          <Link to={`/pvcs/${pvc.metadata?.namespace}/${pvc.metadata?.name}`}>
            {pvc.metadata?.name}
          </Link>
          <Tooltip title={failReason}>
            <AlertTwoTone twoToneColor="#cf1322" />
          </Tooltip>
        </div>
      )
    },
  },
  {
    title: <FormattedMessage id="namespace" />,
    dataIndex: ['metadata', 'namespace'],
  },
  {
    title: 'PV',
    dataIndex: ['spec', 'volumeName'],
    render: (_, pvc) => {
      if (!pvc.spec?.volumeName) {
        return <span>-</span>
      } else {
        return (
          <Link to={`/pvs/${pvc.spec.volumeName}`}>{pvc.spec.volumeName}</Link>
        )
      }
    },
  },
  {
    title: <FormattedMessage id="capacity" />,
    key: 'storage',
    search: false,
    dataIndex: ['spec', 'resources', 'requests', 'storage'],
  },
  {
    title: <FormattedMessage id="accessMode" />,
    key: 'accessModes',
    search: false,
    render: (_, pvc) => {
      let accessModes: string
      if (pvc.spec?.accessModes) {
        accessModes = pvc.spec.accessModes
          .map((accessMode) => accessModeMap[accessMode] || 'Unknown')
          .join(',')
        return <div>{accessModes}</div>
      }
    },
  },
  {
    title: 'StorageClass',
    dataIndex: ['spec', 'storageClassName'],
    render: (_, pvc) => {
      if (pvc.spec?.storageClassName) {
        return (
          <Link to={`/storageclass/${pvc.spec?.storageClassName}/`}>
            {pvc.spec?.storageClassName}
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
    render: (_, pvc) => {
      return <Badge color={getPVCStatusBadge(pvc)} text={pvc.status?.phase} />
    },
  },
  {
    title: <FormattedMessage id="createAt" />,
    key: 'time',
    sorter: 'time',
    search: false,
    render: (_, pv) => (
      <span>
        {new Date(pv.metadata?.creationTimestamp || '').toLocaleDateString(
          'en-US',
          {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
          },
        )}
      </span>
    ),
  },
]
const PVCList: React.FC<unknown> = () => {
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    total: 0,
  })
  const [filter, setFilter] = useState<{
    name?: string
    namespace?: string
    pv?: string
    sc?: string
  }>()
  const [sorter, setSorter] = useState<Record<string, SortOrder>>({
    time: 'ascend',
  })

  const handleTableChange: TableProps['onChange'] = (pagination, _, sorter) => {
    setPagination(pagination)
    if (sorter instanceof Array) {
      setSorter({ time: sorter[0].order || 'ascend' })
    } else {
      setSorter({ time: sorter.order || 'ascend' })
    }
  }
  const { data, isLoading } = usePVCs({
    sort: sorter,
    pageSize: pagination.pageSize,
    current: pagination.current,
    ...filter,
  })

  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])

  return (
    <PageContainer
      header={{
        title: <FormattedMessage id="pvcTablePageName" />,
      }}
    >
      <ProTable<PVC>
        headerTitle={<FormattedMessage id="pvcTableName" />}
        rowKey={(record) => record.metadata?.uid || ''}
        loading={isLoading}
        dataSource={data?.pvcs}
        onChange={handleTableChange}
        search={{
          optionRender: false,
          labelWidth: 120,
          collapsed: false,
        }}
        columns={columns}
        form={{
          onValuesChange: (_, values) => {
            if (values) {
              setFilter((prev) => ({
                ...prev,
                ...values,
                ...values.metadata,
                pv: values.spec?.volumeName,
                sc: values.spec?.storageClassName,
              }))
            }
          },
        }}
        pagination={pagination}
      />
    </PageContainer>
  )
}

export default PVCList
