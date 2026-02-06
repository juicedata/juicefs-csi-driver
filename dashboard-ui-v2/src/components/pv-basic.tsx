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
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { Badge, Button, List, Tooltip } from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import YamlModal from './yaml-modal'
import { YamlIcon } from '@/icons'
import { accessModeMap, PV } from '@/types/k8s'
import { getPVStatusBadge } from '@/utils'

const PVBasic: React.FC<{
  pv: PV
}> = ({ pv }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const showModal = () => {
    setIsModalOpen(true)
  }

  const handleCancel = () => {
    setIsModalOpen(false)
  }

  const [volumeAttributes, setVolumeAttributes] = useState<string[]>([])
  const [labels, setLabels] = useState<string[]>([])
  const [annotations, setAnnotations] = useState<string[]>([])

  useEffect(() => {
    if (pv.spec?.csi?.volumeAttributes) {
      setVolumeAttributes(
        Object.keys(pv.spec?.csi?.volumeAttributes ?? {}).map(
          (key) => `${key}: ${pv.spec?.csi?.volumeAttributes?.[key]}`,
        ),
      )
    }
  }, [pv.spec?.csi?.volumeAttributes])

  useEffect(() => {
    if (pv.metadata?.labels) {
      setLabels(
        Object.keys(pv.metadata.labels).map(
          (key) => `${key}: ${pv.metadata?.labels?.[key]}`,
        ),
      )
    } else {
      setLabels([])
    }
  }, [pv.metadata?.labels])

  useEffect(() => {
    if (pv.metadata?.annotations) {
      setAnnotations(
        Object.keys(pv.metadata.annotations).map(
          (key) => `${key}: ${pv.metadata?.annotations?.[key]}`,
        ),
      )
    } else {
      setAnnotations([])
    }
  }, [pv.metadata?.annotations])

  return (
    <>
      <ProCard
        title={<FormattedMessage id="basic" />}
        extra={
          <>
            <Tooltip title={<FormattedMessage id="showYaml" />}>
              <Button
                className="action-button"
                onClick={showModal}
                icon={<YamlIcon />}
              />
              <YamlModal
                isOpen={isModalOpen}
                onClose={handleCancel}
                content={YAML.stringify(pv)}
              />
            </Tooltip>
          </>
        }
      >
        <ProDescriptions
          column={2}
          dataSource={pv}
          columns={[
            {
              title: 'UID',
              dataIndex: ['metadata', 'uid'],
              render: (_, record) => (
                <code>{record.metadata?.uid}</code>
              ),
            },
            {
              title: 'PVC',
              key: 'pvc',
              render: (_, record) => {
                if (!record.spec?.claimRef) {
                  return '-'
                }
                return (
                  <Link
                    to={`/pvcs/${record.spec?.claimRef?.namespace}/${record.spec?.claimRef?.name}`}
                  >
                    {record.spec?.claimRef?.namespace}/
                    {record.spec.claimRef?.name}
                  </Link>
                )
              },
            },
            {
              title: <FormattedMessage id="capacity" />,
              dataIndex: ['spec', 'capacity', 'storage'],
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
              title: <FormattedMessage id="reclaimPolicy" />,
              dataIndex: ['spec', 'persistentVolumeReclaimPolicy'],
            },
            {
              title: 'StorageClass',
              key: 'storageClass',
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
              title: 'volumeHandle',
              dataIndex: ['spec', 'csi', 'volumeHandle'],
            },
            {
              title: <FormattedMessage id="status" />,
              dataIndex: 'status',
              valueType: 'select',
              render: (_, pv) => {
                return (
                  <Badge color={getPVStatusBadge(pv)} text={pv.status?.phase} />
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
      {labels.length > 0 && (
        <ProCard title={<FormattedMessage id="labels" />}>
          <List
            dataSource={labels}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>
      )}
      {annotations.length > 0 && (
        <ProCard title={<FormattedMessage id="annotations" />}>
          <List
            dataSource={annotations}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>
      )}
      <ProCard title={<FormattedMessage id="volumeAttributes" />}>
        <List
          dataSource={volumeAttributes}
          split={false}
          size="small"
          renderItem={(item) => (
            <List.Item>
              <code>{item}</code>
            </List.Item>
          )}
        />
      </ProCard>
      <ProCard title={<FormattedMessage id="mountOptions" />}>
        <List
          dataSource={pv.spec?.mountOptions}
          split={false}
          size="small"
          renderItem={(item) => (
            <List.Item>
              <code>{item}</code>
            </List.Item>
          )}
        />
      </ProCard>
    </>
  )
}

export default PVBasic
