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

import React, { useEffect, useState } from 'react'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { Button, Space, Tooltip } from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { useDeleteUpgradeJob, useUpdateUpgradeJob } from '@/hooks/job-api.ts'
import { usePVCWithUniqueId } from '@/hooks/pv-api.ts'
import { DeleteIcon, PauseIcon, ResumeIcon, StopIcon } from '@/icons'
import { UpgradeJob } from '@/types/k8s.ts'
import { getUpgradeStatusBadge } from '@/utils'

const UpgradeBasic: React.FC<{
  upgradeJob: UpgradeJob
  freshJob: () => void
}> = (props) => {
  const { upgradeJob, freshJob } = props
  const { data: pvc } = usePVCWithUniqueId(upgradeJob.config.uniqueId)
  const [, action] = useDeleteUpgradeJob()
  const [, updateAction] = useUpdateUpgradeJob()
  const [status, setStatus] = useState(upgradeJob.config.status || 'running')
  useEffect(() => {
    setStatus(upgradeJob.config.status)
  }, [upgradeJob])

  const upgradeData = {
    upgradeJob,
    pvc: pvc,
  }

  return (
    <ProCard
      title={<FormattedMessage id="basic" />}
      extra={
        <Space>
          {canPause(status) ? (
            <Tooltip title="Pause">
              <Button
                onClick={() => {
                  updateAction
                    .execute(upgradeJob.job.metadata?.name || '', 'pause')
                    .then(freshJob)
                }}
                icon={<PauseIcon />}
              />
            </Tooltip>
          ) : null}
          {canResume(status) ? (
            <Tooltip title="Resume">
              <Button
                onClick={() => {
                  updateAction
                    .execute(upgradeJob.job.metadata?.name || '', 'resume')
                    .then(freshJob)
                }}
                icon={<ResumeIcon />}
              />
            </Tooltip>
          ) : null}
          {canStop(status) ? (
            <Tooltip title="Stop">
              <Button
                onClick={() => {
                  updateAction
                    .execute(upgradeJob.job.metadata?.name || '', 'stop')
                    .then(freshJob)
                }}
                icon={<StopIcon />}
              />
            </Tooltip>
          ) : null}
          <Tooltip title="Delete">
            <Button
              onClick={() => {
                action.execute(upgradeJob.job.metadata?.name || '')
                window.location.href = `/jobs`
              }}
              icon={<DeleteIcon />}
            />
          </Tooltip>
        </Space>
      }
    >
      <ProDescriptions
        column={2}
        dataSource={upgradeData}
        columns={[
          {
            title: <FormattedMessage id="node" />,
            key: 'node',
            render: (_, data) => data.upgradeJob?.config.node || 'All Nodes',
          },
          {
            title: <FormattedMessage id="parallelNum" />,
            key: 'worker',
            render: (_, data) => data.upgradeJob?.config.parallel || '-',
          },
          {
            title: <FormattedMessage id="ignoreError" />,
            key: 'ignoreError',
            dataIndex: 'ignoreError',
            render: (_, record) => {
              if (record.upgradeJob?.config.ignoreError) {
                return (
                  <div>
                    <FormattedMessage id="true" />
                  </div>
                )
              } else {
                return (
                  <div>
                    <FormattedMessage id="false" />
                  </div>
                )
              }
            },
          },
          {
            title: 'PVC',
            render: (_, record) => {
              if (!record.pvc) {
                return '-'
              }
              return (
                <Link
                  to={`/pvcs/${record?.pvc?.metadata?.namespace}/${record.pvc?.metadata?.name}`}
                >
                  {record?.pvc?.metadata?.namespace}/
                  {record.pvc?.metadata?.name}
                </Link>
              )
            },
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            dataIndex: 'status',
            render: (_, data) => {
              const status =
                data.upgradeJob?.config.status === ''
                  ? 'running'
                  : data.upgradeJob?.config.status
              return (
                <Badge
                  status={getUpgradeStatusBadge(status || 'running')}
                  text={status}
                ></Badge>
              )
            },
          },
        ]}
      ></ProDescriptions>
    </ProCard>
  )
}

export default UpgradeBasic

const canPause = (status: string): boolean => {
  return (
    status !== 'stop' &&
    status !== 'pause' &&
    status !== 'fail' &&
    status !== 'success'
  )
}

const canStop = (status: string): boolean => {
  return status !== 'fail' && status !== 'success' && status !== 'stop'
}

const canResume = (status: string): boolean => {
  return status === 'pause'
}
