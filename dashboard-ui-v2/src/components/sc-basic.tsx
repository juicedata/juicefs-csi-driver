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
import { Button } from 'antd'
import { StorageClass } from 'kubernetes-types/storage/v1'
import { FormattedMessage } from 'react-intl'
import YAML from 'yaml'

import YamlModal from './yaml-modal'

const SCBasic: React.FC<{
  sc: StorageClass
}> = (props) => {
  const { sc } = props

  const [isModalOpen, setIsModalOpen] = useState(false)

  const showModal = () => {
    setIsModalOpen(true)
  }

  const handleCancel = () => {
    setIsModalOpen(false)
  }

  return (
    <ProCard title={<FormattedMessage id="basic" />}>
      <ProDescriptions
        column={2}
        dataSource={sc}
        columns={[
          {
            title: <FormattedMessage id="reclaimPolicy" />,
            key: 'reclaimPolicy',
            dataIndex: 'reclaimPolicy',
          },
          {
            title: <FormattedMessage id="allowVolumeExpansion" />,
            key: 'allowVolumeExpansion',
            render: () => (
              <FormattedMessage
                id={sc.allowVolumeExpansion ? 'true' : 'false'}
              />
            ),
          },
          {
            title: <FormattedMessage id="createAt" />,
            dataIndex: ['metadata', 'creationTimestamp'],
            render: (_, row) =>
              new Date(
                row.metadata?.creationTimestamp as string,
              ).toLocaleString(),
          },
          {
            title: 'Yaml',
            key: 'yaml',
            render: () => (
              <>
                <Button type="primary" onClick={showModal}>
                  Yaml
                </Button>
                <YamlModal
                  isOpen={isModalOpen}
                  onClose={handleCancel}
                  content={YAML.stringify(sc)}
                />
              </>
            ),
          },
        ]}
      />
    </ProCard>
  )
}

export default SCBasic
