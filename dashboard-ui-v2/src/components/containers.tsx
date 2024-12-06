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
import { Button, Space, Table, Tag, Tooltip } from 'antd'
import { ContainerStatus } from 'kubernetes-types/core/v1'
import { FormattedMessage } from 'react-intl'
import { useParams } from 'react-router-dom'

import { DebugModal } from '.'
import LogModal from './log-modal'
import WarmupModal from './warmup-modal'
import XTermModal from './xterm-modal'
import UpgradeModal from '@/components/upgrade-modal.tsx'
import {
  AccessLogIcon,
  DebugIcon,
  LogIcon,
  TerminalIcon,
  UpgradeIcon,
  WarmupIcon,
} from '@/icons'
import { DetailParams } from '@/types'
import { Pod } from '@/types/k8s'
import { isMountPod, supportBinarySmoothUpgrade, supportDebug } from '@/utils'

const Containers: React.FC<{
  pod: Pod
  containerStatuses?: Array<ContainerStatus>
}> = (props) => {
  const { pod, containerStatuses } = props

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
            title: <FormattedMessage id="image" />,
            dataIndex: 'image',
            render: (image) => {
              return <div style={{ maxWidth: '400px' }}>{image}</div>
            },
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
            title: <FormattedMessage id="action" />,
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
                    <Tooltip title="Log" zIndex={0}>
                      <Button
                        className="action-button"
                        onClick={onClick}
                        icon={<LogIcon />}
                      />
                    </Tooltip>
                  )}
                </LogModal>
                <XTermModal
                  namespace={namespace!}
                  name={name!}
                  container={record.name}
                >
                  {({ onClick }) => (
                    <Tooltip title="Exec in container" zIndex={0}>
                      <Button
                        className="action-button"
                        icon={<TerminalIcon />}
                        onClick={onClick}
                      />
                    </Tooltip>
                  )}
                </XTermModal>
                {isMountPod(pod) ? (
                  <>
                    <LogModal
                      namespace={namespace!}
                      name={name!}
                      container={record.name}
                      hasPrevious={false}
                      type="accesslog"
                    >
                      {({ onClick }) => (
                        <Tooltip title="Access Log" zIndex={0}>
                          <Button
                            className="action-button"
                            onClick={onClick}
                            icon={<AccessLogIcon />}
                          />
                        </Tooltip>
                      )}
                    </LogModal>
                    {supportDebug(c.image) ? (
                      <DebugModal
                        namespace={namespace!}
                        name={name!}
                        container={record.name}
                      >
                        {({ onClick }) => (
                          <Tooltip title="Debug" zIndex={0}>
                            <Button
                              className="action-button"
                              onClick={onClick}
                              icon={<DebugIcon />}
                            />
                          </Tooltip>
                        )}
                      </DebugModal>
                    ) : null}

                    <WarmupModal
                      namespace={namespace!}
                      name={name!}
                      container={c}
                    >
                      {({ onClick }) => (
                        <Tooltip title="Warmup" zIndex={0}>
                          <Button
                            className="action-button"
                            onClick={onClick}
                            icon={<WarmupIcon />}
                          />
                        </Tooltip>
                      )}
                    </WarmupModal>

                    {supportBinarySmoothUpgrade(c.image) ? (
                      <UpgradeModal
                        namespace={namespace!}
                        name={name!}
                        recreate={false}
                      >
                        {({ onClick }) => (
                          <Tooltip title="Binary Upgrade" zIndex={0}>
                            <Button
                              className="action-button"
                              onClick={onClick}
                              icon={<UpgradeIcon />}
                            />
                          </Tooltip>
                        )}
                      </UpgradeModal>
                    ) : null}
                  </>
                ) : null}
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
