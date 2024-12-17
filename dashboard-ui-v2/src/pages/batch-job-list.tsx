/*
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

import React, { useState } from 'react'
import { PlusOutlined } from '@ant-design/icons'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import { Button, TablePaginationConfig, TableProps } from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link, useSearchParams } from 'react-router-dom'

import BatchUpgradeModal from '@/components/batch-upgrade-modal.tsx'
import { useUpgradeJobs } from '@/hooks/job-api.ts'
import { UpgradeJob } from '@/types/k8s.ts'
import { getUpgradeStatusBadge } from '@/utils'

const columns: ProColumns<UpgradeJob>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['job', 'metadata', 'name'],
    render: (_, upgradeJob) => {
      return (
        <div>
          <Link
            to={`/jobs/${upgradeJob.job.metadata?.namespace}/${upgradeJob.job.metadata?.name}`}
          >
            {upgradeJob.job.metadata?.name}
          </Link>
        </div>
      )
    },
  },
  {
    title: <FormattedMessage id="status" />,
    key: 'status',
    render: (_, upgradeJob) => {
      const status =
        upgradeJob.config.status === '' ? 'running' : upgradeJob.config.status
      return (
        <Badge status={getUpgradeStatusBadge(status)} text={status}></Badge>
      )
    },
  },
  {
    title: <FormattedMessage id="createAt" />,
    hideInSearch: true,
    dataIndex: ['job', 'metadata', 'creationTimestamp'],
    render: (_, row) =>
      new Date(row.job.metadata?.creationTimestamp as string).toLocaleString(),
  },
]

const UpgradeJobList: React.FC = () => {
  const [searchParams] = useSearchParams()
  const modalOpen = searchParams.get('modalOpen')
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    total: 0,
  })
  const {
    data,
    isLoading,
    mutate: listJobMutate,
  } = useUpgradeJobs({
    current: pagination.current,
    pageSize: pagination.pageSize,
  })

  const [isModalVisible, setIsModalVisible] = useState(
    modalOpen === 'true' || false,
  )

  const showModal = () => {
    setIsModalVisible(true)
  }

  const handleCreate = () => {
    setIsModalVisible(false)
    listJobMutate()
  }

  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }

  return (
    <PageContainer
      header={{
        title: <FormattedMessage id="upgradeJobTablePageName" />,
      }}
    >
      <ProTable
        headerTitle={<FormattedMessage id="upgradeJobTableName" />}
        toolbar={{
          actions: [
            <>
              <Button
                key="button"
                icon={<PlusOutlined />}
                onClick={showModal}
                type="primary"
              >
                <FormattedMessage id="new" />
              </Button>
              <BatchUpgradeModal
                modalOpen={isModalVisible}
                onOk={handleCreate}
                onCancel={() => setIsModalVisible(false)}
              ></BatchUpgradeModal>
            </>,
          ],
          settings: undefined,
        }}
        loading={isLoading}
        columns={columns}
        dataSource={data?.jobs}
        pagination={pagination}
        search={false}
        onChange={handleTableChange}
        rowKey={(row) => row.job.metadata!.uid!}
      />
    </PageContainer>
  )
}

export default UpgradeJobList