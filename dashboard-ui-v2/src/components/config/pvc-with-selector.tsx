/*
 * Copyright 2025 Juicedata Inc
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
import { ProCard, ProColumns, ProTable } from '@ant-design/pro-components'
import { TablePaginationConfig, TableProps } from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { pvcSelector } from '@/types/config.ts'
import { PVCWithPod } from '@/types/k8s.ts'

const columns: ProColumns<PVCWithPod>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['PVC', 'metadata', 'name'],
    render: (_, pvc) => {
      return (
        <Link
          to={`/pvcs/${pvc.PVC.metadata?.namespace}/${pvc.PVC.metadata?.name}`}
        >
          {pvc.PVC.metadata?.namespace}/{pvc.PVC.metadata?.name}
        </Link>
      )
    },
  },
  {
    title: 'Mount Pods',
    key: 'mountPods',
    render: (_, pvc) => {
      if (!pvc.MountPods || pvc.MountPods.length === 0) {
        return <span>-</span>
      } else if (pvc.MountPods.length === 1) {
        const mountPod = pvc.MountPods[0]
        if (mountPod === undefined) {
          return
        }
        return (
          <Link
            to={`/syspods/${mountPod?.metadata?.namespace}/${mountPod?.metadata?.name}/`}
          >
            {mountPod?.metadata?.name}
          </Link>
        )
      } else {
        return (
          <div>
            {pvc.MountPods.map((mountPod) => (
              <div key={mountPod.metadata?.uid}>
                <Link
                  to={`/syspods/${mountPod.metadata?.namespace}/${mountPod.metadata?.name}/`}
                >
                  {mountPod?.metadata?.name}
                </Link>
                <br />
              </div>
            ))}
          </div>
        )
      }
    },
  },
]

const PVCWithSelector: React.FC<{
  pvcSelector?: pvcSelector
  pvcs?: PVCWithPod[]
}> = (props) => {
  const { pvcSelector, pvcs } = props
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 10,
    total: 0,
  })

  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }
  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: pvcs?.length || 0 }))
  }, [pvcs])

  return (
    <>
      {pvcSelector ? (
        pvcs &&
        pvcs.length !== 0 && (
          <ProCard title={<FormattedMessage id="pvcMatched" />}>
            <ProTable<PVCWithPod>
              name={'PVC'}
              dataSource={pvcs}
              columns={columns}
              search={false}
              options={false}
              onChange={handleTableChange}
              pagination={pagination}
              rowKey={(row) => row.PVC.metadata!.uid!}
            />
          </ProCard>
        )
      ) : (
        <ProCard title={<FormattedMessage id="pvcMatched" />}>
          <span style={{ fontSize: '14px', fontWeight: 'normal' }}>
            <FormattedMessage id="allPVC" />
          </span>
        </ProCard>
      )}
    </>
  )
}

export default PVCWithSelector
