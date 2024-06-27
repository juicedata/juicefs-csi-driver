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

import { ProCard } from '@ant-design/pro-components'
import { Button, Space, Table, Tag } from 'antd'
import { Container, ContainerStatus } from 'kubernetes-types/core/v1'
import { FormattedMessage } from 'react-intl'
import { useParams } from 'react-router-dom'

import LogModal from './log-modal'
import XTermModal from './xterm-modal'
import { DetailParams } from '@/types'

const Containers: React.FC<{
  containers: Array<Container>
  containerStatuses?: Array<ContainerStatus>
}> = (props) => {
  const { containerStatuses } = props

  const { namespace, name } = useParams<DetailParams>()

  return (
    <ProCard title={<FormattedMessage id="containerList" />}>
      <Table
        columns={[
          {
            title: <FormattedMessage id="containerName" />,
            dataIndex: 'name',
          },
          {
            title: <FormattedMessage id="restartCount" />,
            dataIndex: 'restartCount',
          },
          {
            title: <FormattedMessage id="status" />,
            dataIndex: 'ready',
            render: (_, cn) => {
              const color = cn.ready ? 'green' : 'red'
              const text = cn.ready ? 'Ready' : 'NotReady'
              return <Tag color={color}>{text}</Tag>
            },
          },
          {
            title: <FormattedMessage id="startAt" />,
            dataIndex: ['state', 'running', 'startedAt'],
            key: 'startAt',
            render: (startAt) => new Date(startAt).toLocaleString(),
          },
          {
            title: <FormattedMessage id="log" />,
            key: 'action',
            render: (record, c) => (
              <Space>
                <LogModal
                  namespace={namespace!}
                  name={name!}
                  container={record.name}
                  hasPrevious={c.restartCount > 0}
                >
                  {({ onClick }) => (
                    <Button type="primary" onClick={onClick}>
                      Log
                    </Button>
                  )}
                </LogModal>
                <XTermModal
                  namespace={namespace!}
                  name={name!}
                  container={record.name}
                >
                  {({ onClick }) => (
                    <Button type="primary" onClick={onClick}>
                      Exec
                    </Button>
                  )}
                </XTermModal>
              </Space>
            ),
          },
        ]}
        dataSource={containerStatuses}
        rowKey={(c) => c.name}
        pagination={false}
      />
    </ProCard>
  )
}

export default Containers
