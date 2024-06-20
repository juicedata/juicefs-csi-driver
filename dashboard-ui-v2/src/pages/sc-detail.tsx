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

import { PageContainer, ProCard } from '@ant-design/pro-components'
import { ConfigProvider, List } from 'antd'
import { FormattedMessage } from 'react-intl'
import SCBasic from '@/components/sc-basic.tsx'
import { useSC } from '@/hooks/pv-api.ts'
import React from 'react'
import { scParameter } from '@/utils'
import PVsTable from '@/components/pvs-table.tsx'

const SCDetail: React.FC<{
  name?: string
}> = (props) => {
  const { name } = props

  const { data, isLoading } = useSC(name)
  if (name === '' || !data) {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="StorageClassNotFound" />,
        }}
      ></PageContainer>
    )
  }

  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: '#00b96b',
          borderRadius: 4,
          colorBgContainer: '#ffffff',
        },
      }}
    >
      <PageContainer
        fixedHeader
        loading={isLoading}
        header={{
          title: name,
        }}
      >
        <SCBasic sc={data} />
        <ProCard title={<FormattedMessage id="parameters" />}>
          <List
            dataSource={scParameter(data)}
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
            dataSource={data.mountOptions}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>
        <PVsTable sc={data.metadata?.name || ''} />
      </PageContainer>
    </ConfigProvider>
  )
}

export default SCDetail
