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
import { Button, message, Popconfirm, Space, Tooltip } from 'antd'
import { omit } from 'lodash'
import { FormattedMessage } from 'react-intl'
import { useNavigate } from 'react-router-dom'
import YAML from 'yaml'

import { YamlModal } from '@/components'
import CgWorkersTable from '@/components/cg-workers-table'
import {
  useCacheGroup,
  useDeleteCacheGroup,
  useUpdateCacheGroup,
} from '@/hooks/cg-api'
import { YamlIcon } from '@/icons'

const CgDetail: React.FC<{
  name?: string
  namespace?: string
}> = memo((props) => {
  const { name, namespace } = props
  const [isModalOpen, setIsModalOpen] = useState(false)
  const redirect = useNavigate()

  const { data, isLoading, mutate } = useCacheGroup(namespace, name)
  const [, updateCg] = useUpdateCacheGroup()
  const [, deleteCg] = useDeleteCacheGroup()

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
                editable
                onSave={async (data) => {
                  const resp = await updateCg.execute({
                    body: YAML.parse(data),
                  })
                  if (resp.status !== 200) {
                    message.error('error: ' + (await resp.json()).error)
                    return
                  }
                  message.success('success')
                  handleCancel()
                  mutate()
                }}
              />
            </Tooltip>
            <Popconfirm
              title="Delete this CacheGroup"
              description={
                <FormattedMessage id="deleteCacheGroupDescription" />
              }
              onConfirm={async () => {
                await deleteCg.execute({
                  body: data,
                })
                message.success('success')
                redirect('/cachegroups')
              }}
              okText="Yes"
              cancelText="No"
            >
              <Button type="primary" danger>
                Delete
              </Button>
            </Popconfirm>
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
              title: <FormattedMessage id="expectWorker" />,
              dataIndex: ['status', 'expectWorker'],
            },
            {
              title: <FormattedMessage id="readyWorker" />,
              dataIndex: ['status', 'readyWorker'],
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
