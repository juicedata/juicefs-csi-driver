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
import { ProCard } from '@ant-design/pro-components'
import { Table } from 'antd'
import { FormattedMessage } from 'react-intl'

import { useEvents } from '@/hooks/use-api'

const EventTable: React.FC<{
  source: 'pod' | 'pv' | 'pvc'
  name: string
  namespace?: string
}> = ({ source, namespace, name }) => {
  const { data } = useEvents(source, namespace, name)

  useEffect(() => {
    if (data) {
      data.sort((a, b) => {
        const aTime = new Date(a.firstTimestamp || a.eventTime || 0).getTime()
        const bTime = new Date(b.firstTimestamp || b.eventTime || 0).getTime()
        return bTime - aTime
      })
      data.forEach((event) => {
        event.firstTimestamp = event.firstTimestamp || event.eventTime
        event.reportingComponent =
          event.reportingComponent || event.source?.component
      })
    }
  }, [data])

  return (
    <ProCard title={<FormattedMessage id="event" />}>
      <Table
        columns={[
          {
            title: <FormattedMessage id="type" />,
            dataIndex: 'type',
          },
          {
            title: <FormattedMessage id="reason" />,
            dataIndex: 'reason',
          },
          {
            title: <FormattedMessage id="createAt" />,
            dataIndex: 'firstTimestamp',
            render: (t) => new Date(t).toLocaleString(),
          },
          {
            title: <FormattedMessage id="from" />,
            dataIndex: 'reportingComponent',
          },
          {
            title: <FormattedMessage id="message" />,
            dataIndex: 'message',
          },
        ]}
        dataSource={data}
        size="small"
        pagination={false}
        rowKey={(c) => c.metadata?.uid || ''}
      />
    </ProCard>
  )
}

export default EventTable
