import React, { useEffect } from 'react'
import { ProCard } from '@ant-design/pro-components'
import { Table } from 'antd'
import { FormattedMessage } from 'react-intl'

import { usePodEvents } from '@/hooks/use-api'

const EventTable: React.FC<{
  namespace: string
  name: string
}> = ({ namespace, name }) => {
  const { data } = usePodEvents(namespace, name)

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
