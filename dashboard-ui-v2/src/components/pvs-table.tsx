/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import React from 'react'
import { ProCard } from '@ant-design/pro-components'
import { Badge, Table } from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import { usePVOfSC } from '@/hooks/pv-api.ts'
import { getPVStatusBadge } from '@/utils'

const PVsTable: React.FC<{
  sc: string
}> = ({ sc }) => {
  const { data } = usePVOfSC(sc)
  if (!data || data.length === 0) {
    return null
  }
  return (
    <ProCard title={'PV'}>
      <Table
        columns={[
          {
            title: <FormattedMessage id="name" />,
            key: 'name',
            render: (pv) => (
              <Link to={`/pv/${pv.metadata.name}/`}>
                {pv.metadata.name}
              </Link>
            ),
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            dataIndex: ['status', 'phase'],
            render: (_, pv) => {
              return <Badge color={getPVStatusBadge(pv)} text={pv.status?.phase} />
            },
          },
          {
            title: <FormattedMessage id="createAt" />,
            dataIndex: ['metadata', 'creationTimestamp'],
            key: 'time',
          },
        ]}
        dataSource={data}
        rowKey={(c) => c.metadata?.uid || ''}
        pagination={false}
      />
    </ProCard>
  )
}

export default PVsTable
