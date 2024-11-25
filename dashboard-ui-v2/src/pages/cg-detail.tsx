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

import { memo, useState } from 'react'
import {
  PageContainer,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components'
import { Button, Space, Tooltip } from 'antd'
import { omit } from 'lodash'
import { FormattedMessage } from 'react-intl'
import YAML from 'yaml'

import { YamlModal } from '@/components'
import CgWorkersTable from '@/components/cg-workers-table'
import { useCacheGroup } from '@/hooks/cg-api'
import { YamlIcon } from '@/icons'

const CgDetail: React.FC<{
  name?: string
  namespace?: string
}> = memo((props) => {
  const { name, namespace } = props
  const [isModalOpen, setIsModalOpen] = useState(false)

  const { data, isLoading } = useCacheGroup(namespace, name)

  const showModal = () => {
    setIsModalOpen(true)
  }

  const handleCancel = () => {
    setIsModalOpen(false)
  }

  if (namespace === '' || name === '' || !data) {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="podNotFound" />,
        }}
      ></PageContainer>
    )
  }

  return (
    <PageContainer
      fixedHeader
      loading={isLoading}
      header={{
        title: name,
        subTitle: namespace,
      }}
    >
      <ProCard
        title={<FormattedMessage id="basic" />}
        extra={
          <Space>
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
                content={YAML.stringify(omit(data, ['metadata.managedFields']))}
              />
            </Tooltip>
          </Space>
        }
      >
        <ProDescriptions
          column={2}
          dataSource={data}
          columns={[
            {
              title: <FormattedMessage id="namespace" />,
              dataIndex: ['metadata', 'namespace'],
            },
            {
              title: <FormattedMessage id="status" />,
              dataIndex: ['status', 'phase'],
            },
            {
              title: <FormattedMessage id="readyWorker" />,
              dataIndex: ['status', 'readyWorker'],
            },
            {
              title: <FormattedMessage id="expectWorker" />,
              dataIndex: ['status', 'expectWorker'],
            },
            {
              title: <FormattedMessage id="cacheGroupName" />,
              dataIndex: ['status', 'cacheGroup'],
            },
          ]}
        />
      </ProCard>
      <CgWorkersTable name={name} namespace={namespace} />
    </PageContainer>
  )
})

export default CgDetail
