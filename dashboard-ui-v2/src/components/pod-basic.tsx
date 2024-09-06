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

import React, { useState } from 'react'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { Button, message, Popconfirm, type PopconfirmProps, Tooltip } from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import YAML from 'yaml'

import YamlModal from './yaml-modal'
import { UpgradeIcon, YamlIcon } from '@/icons'
import { Pod } from '@/types/k8s'
import { getPodStatusBadge, omitPod, podStatus, supportPodSmoothUpgrade } from '@/utils'
import { useMountUpgrade, useMountPodImage } from '@/hooks/use-api.ts'

const PodBasic: React.FC<{
  pod: Pod
}> = (props) => {
  const { pod } = props

  const [isModalOpen, setIsModalOpen] = useState(false)
  const [, actions] = useMountUpgrade()
  const { data } = useMountPodImage(pod.metadata?.namespace, pod.metadata?.name)
  const [image] = useState(pod.spec?.containers[0].image)

  const showModal = () => {
    setIsModalOpen(true)
  }

  const handleCancel = () => {
    setIsModalOpen(false)
  }

  const confirm: PopconfirmProps['onConfirm'] = () => {
    actions.execute(
      pod.metadata?.namespace,
      pod.metadata?.name,
      true,
    )
    message.success('Successfully trigger pod smoothly upgrade')
  }

  return (
    <ProCard
      title={<FormattedMessage id="basic" />}
      extra={
        <>
          {supportPodSmoothUpgrade(image || '') && supportPodSmoothUpgrade(data || '') ? (
            <Popconfirm
              title="Smoothly Upgrade"
              description={`Are you sure to upgrade to ${data}?`}
              onConfirm={confirm}
              okText="Yes"
              cancelText="No"
            >
              <Tooltip title="Smoothly Upgrade" zIndex={0}>
                <Button
                  className="action-button"
                  icon={<UpgradeIcon />}
                />
              </Tooltip>
            </Popconfirm>
          ) : null}
          <Tooltip title="Show Yaml">
            <Button
              className="action-button"
              onClick={showModal}
              icon={<YamlIcon />}
            >
              Yaml
            </Button>
            <YamlModal
              isOpen={isModalOpen}
              onClose={handleCancel}
              content={YAML.stringify(omitPod(pod))}
            />
          </Tooltip>
        </>
      }
    >
      <ProDescriptions
        column={2}
        dataSource={pod}
        columns={[
          {
            title: <FormattedMessage id="namespace" />,
            dataIndex: ['metadata', 'namespace'],
          },
          {
            title: <FormattedMessage id="node" />,
            dataIndex: ['spec', 'nodeName'],
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            render: (_, pod) => {
              const finalStatus = podStatus(pod)
              return (
                <Badge
                  color={getPodStatusBadge(finalStatus || '')}
                  text={finalStatus}
                />
              )
            },
          },
          {
            title: <FormattedMessage id="createAt" />,
            dataIndex: ['metadata', 'creationTimestamp'],
            render: (_, row) =>
              new Date(
                row.metadata?.creationTimestamp as string,
              ).toLocaleString(),
          },
        ]}
      />
    </ProCard>
  )
}

export default PodBasic
