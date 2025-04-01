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
import { Button, Space, Tooltip } from 'antd'
import { Badge } from 'antd/lib'
import { FormattedMessage } from 'react-intl'
import YAML from 'yaml'

import YamlModal from './yaml-modal'
import UpgradeModal from '@/components/upgrade-modal.tsx'
import { useDownloadPodDebugInfos, useMountPodImage } from '@/hooks/use-api.ts'
import { GatherIcon, UpgradeIcon, YamlIcon } from '@/icons'
import { Pod } from '@/types/k8s'
import {
  getPodStatusBadge,
  isMountPod, isSysPod,
  omitPod,
  podStatus,
  supportPodSmoothUpgrade,
} from '@/utils'

const PodBasic: React.FC<{
  pod: Pod
}> = (props) => {
  const { pod } = props

  const [isModalOpen, setIsModalOpen] = useState(false)
  const { data } = useMountPodImage(
    isMountPod(pod),
    pod.metadata?.namespace,
    pod.metadata?.name,
  )
  const [image] = useState(pod.spec?.containers[0].image)
  const [state, actions] = useDownloadPodDebugInfos(pod.metadata?.namespace, pod.metadata?.name)

  const showModal = () => {
    setIsModalOpen(true)
  }

  const handleCancel = () => {
    setIsModalOpen(false)
  }

  return (
    <ProCard
      title={<FormattedMessage id="basic" />}
      extra={
        <Space>
          <Tooltip title={<FormattedMessage id="gatherDiagnosis" />} zIndex={0}>
            <Button
              className="action-button"
              loading={state.status === 'loading'}
              onClick={() => {
                actions.execute()
              }}
              icon={<GatherIcon />}
            >
            </Button>
          </Tooltip>
          {supportPodSmoothUpgrade(image || '') &&
          supportPodSmoothUpgrade(data || '') ? (
            <UpgradeModal
              namespace={pod.metadata?.namespace || ''}
              name={pod.metadata?.name || ''}
              recreate={true}
            >
              {({ onClick }) => (
                <Tooltip title="Upgrade" zIndex={0}>
                  <Button
                    className="action-button"
                    onClick={onClick}
                    icon={<UpgradeIcon />}
                  />
                </Tooltip>
              )}
            </UpgradeModal>
          ) : null}
          <Tooltip
            title={isSysPod(pod) ? <FormattedMessage id="showYaml" /> : <FormattedMessage id="desensitizedYaml" />}>
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
        </Space>
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
                <div>
                  <Badge
                    color={getPodStatusBadge(finalStatus || '')}
                    text={finalStatus}
                  />
                </div>
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
