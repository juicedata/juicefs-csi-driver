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

import { useState } from 'react'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { Badge, Button, Tooltip } from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import YamlModal from './yaml-modal'
import { YamlIcon } from '@/icons'
import { accessModeMap, PVC } from '@/types/k8s'
import { getPVCStatusBadge } from '@/utils'

const PVCBasic: React.FC<{
  pvc: PVC
}> = ({ pvc }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
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
        <>
          <Tooltip title="Show Yaml">
            <Button
              className="action-button"
              onClick={showModal}
              icon={<YamlIcon />}
            />
            <YamlModal
              isOpen={isModalOpen}
              onClose={handleCancel}
              content={YAML.stringify(pvc)}
            />
          </Tooltip>
        </>
      }
    >
      <ProDescriptions
        column={2}
        dataSource={pvc}
        columns={[
          {
            title: 'PV',
            key: 'pv',
            render: (_, record) => {
              if (!record.spec?.volumeName) {
                return '-'
              }
              return (
                <Link to={`/pvs/${record.spec?.volumeName}`}>
                  {record.spec?.volumeName}
                </Link>
              )
            },
          },
          {
            title: <FormattedMessage id="namespace" />,
            dataIndex: ['metadata', 'namespace'],
          },
          {
            title: <FormattedMessage id="capacity" />,
            dataIndex: ['spec', 'resources', 'requests', 'storage'],
          },
          {
            title: <FormattedMessage id="accessMode" />,
            dataIndex: ['spec', 'accessModes'],
            render: (_, record) =>
              record.spec?.accessModes
                ?.map((mode) => accessModeMap[mode] || 'Unknown')
                .join(','),
          },
          {
            title: 'StorageClass',
            dataIndex: ['spec', 'storageClassName'],
            render: (_, record) => {
              if (!record.spec?.storageClassName) {
                return '-'
              }
              return (
                <Link to={`/storageclass/${record.spec.storageClassName}`}>
                  {record.spec.storageClassName}
                </Link>
              )
            },
          },
          {
            title: <FormattedMessage id="status" />,
            dataIndex: 'status',
            render: (_, pv) => {
              return (
                <Badge color={getPVCStatusBadge(pvc)} text={pv.status?.phase} />
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

export default PVCBasic
