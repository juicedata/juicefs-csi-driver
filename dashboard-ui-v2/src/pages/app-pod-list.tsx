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
  ConfigProvider,
  Tooltip,
  type TablePaginationConfig,
  type TableProps,
} from 'antd'
import { Badge } from 'antd/lib'
import dayjs from 'dayjs'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { useAppPods } from '@/hooks/use-api'
import { Pod, PodStatusEnum } from '@/types/k8s'
import {
  failedReasonOfAppPod,
  getPodStatusBadge,
  getPVStatusBadge,
  podStatus,
} from '@/utils'

const columns: ProColumns<Pod>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    render: (_, pod) => {
      const podFailReason = failedReasonOfAppPod(pod) || ''
      if (podFailReason === '') {
        return (
          <Link to={`/pods/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
            {pod.metadata?.name}
          </Link>
        )
      }
      const failReason = <FormattedMessage id={podFailReason} />
      return (
        <div>
          <Link to={`/pods/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
            {pod.metadata?.namespace} / {pod.metadata?.name}
          </Link>
          <Tooltip title={failReason}>
            <AlertTwoTone twoToneColor="#cf1322" />
          </Tooltip>
        </div>
      )
    },
  },
  {
    title: 'PV',
    dataIndex: 'pvs',
    render: (_, pod) => {
      if (!pod.pvs || pod.pvs.length === 0) {
        return <span>-</span>
      } else if (pod.pvs.length === 1) {
        const pv = pod.pvs[0]
        return (
          <Badge
            color={getPVStatusBadge(pv)}
            text={
              <Link to={`/pvs/${pv.metadata?.name}`}>{pv.metadata?.name}</Link>
            }
          />
        )
      } else {
        return (
          <div>
            {pod.pvs.map((key) => (
              <div key={key.metadata?.uid}>
                <Badge
                  color={getPVStatusBadge(key)}
                  text={
                    <Link to={`/pvs/${key.metadata?.name}`}>
                      {key.metadata?.name}
                    </Link>
                  }
                />
                <br />
              </div>
            ))}
          </div>
        )
      }
    },
  },
  {
    title: 'Mount Pods',
    dataIndex: ['metadata', 'mountPods'],
    render: (_, pod) => {
      if (!pod.mountPods || pod.mountPods.length === 0) {
        return <span>-</span>
      } else if (pod.mountPods.length === 1) {
        const mountPod = pod.mountPods[0]
        if (mountPod === undefined) {
          return
        }
        return (
          <Badge
            color={getPodStatusBadge(podStatus(mountPod) || '')}
            text={
              <Link
                to={`/pods/${mountPod?.metadata?.namespace}/${mountPod?.metadata?.name}/`}
              >
                {mountPod?.metadata?.name}
              </Link>
            }
          />
        )
      } else {
        return (
          <div>
            {pod.mountPods.map((mountPod) => (
              <div key={mountPod.metadata?.uid}>
                <Badge
                  color={getPodStatusBadge(podStatus(mountPod) || '')}
                  text={
                    <Link
                      to={`/pods/${mountPod.metadata?.namespace}/${mountPod.metadata?.name}/`}
                    >
                      {mountPod?.metadata?.name}
                    </Link>
                  }
                />
                <br />
              </div>
            ))}
          </div>
        )
      }
    },
  },
  {
    title: <FormattedMessage id="status" />,
    dataIndex: ['status', 'phase'],
    valueEnum: PodStatusEnum,
  },
  {
    title: 'CSI Node',
    key: 'csiNode',
    render: (_, pod) => {
      if (!pod.csiNode) {
        return '-'
      }
      return (
        <Badge
          color={getPodStatusBadge(podStatus(pod.csiNode) || '')}
          text={
            <Link
              to={`/pods/${pod.csiNode.metadata?.namespace}/${pod.csiNode.metadata?.name}/`}
            >
              {pod.csiNode.metadata?.name}
            </Link>
          }
        />
      )
    },
  },
  {
    title: '创建时间',
    dataIndex: ['metadata', 'creationTimestamp'],
    render: (_, row) =>
      dayjs(row.metadata?.creationTimestamp).format('YYYY-MM-DD HH:mm:ss'),
  },
]

const PodList: React.FC = () => {
  const [pagination, setPagination] = useState<TablePaginationConfig>({
    current: 1,
    pageSize: 20,
    total: 0,
  })

  const handleTableChange: TableProps['onChange'] = (pagination) => {
    setPagination(pagination)
  }

  const { data, isLoading } = useAppPods({
    current: pagination.current,
    pageSize: pagination.pageSize,
  })

  useEffect(() => {
    setPagination((prev) => ({ ...prev, total: data?.total || 0 }))
  }, [data?.total])

  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: '#00b96b',
          borderRadius: 4,
          colorBgContainer: '#ffffff',
        },
      }}
    >
      <PageContainer
        header={{
          title: '应用 Pod',
        }}
      >
        <ProTable
          headerTitle={<FormattedMessage id="appPodTableName" />}
          tooltip={<FormattedMessage id="appPodTableTip" />}
          columns={columns}
          loading={isLoading}
          dataSource={data?.pods}
          pagination={pagination}
          onChange={handleTableChange}
          rowKey={(row) => row.metadata!.uid!}
        />
      </PageContainer>
    </ConfigProvider>
  )
}

export default PodList
