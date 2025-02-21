/*
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

import React from 'react'
import {
  ProCard,
  ProDescriptions,
  ProForm,
  ProFormList,
  ProFormText,
} from '@ant-design/pro-components'
import { Input } from 'antd'
import { FormattedMessage } from 'react-intl'

import { mountPodPatch } from '@/types/config.ts'

const PVCSelectorForm: React.FC<{
  patch?: mountPodPatch
}> = (props) => {
  const { patch } = props

  return (
    <ProCard title={<FormattedMessage id="selector" />}>
      <ProDescriptions
        column={2}
        dataSource={patch?.pvcSelector}
        columns={[
          {
            title: <FormattedMessage id="pvcName" />,
            key: 'pvcName',
            render: () => {
              return (
                <ProForm.Item name={['pvcSelector', 'matchName']}>
                  <Input />
                </ProForm.Item>
              )
            },
          },
          {
            title: <FormattedMessage id="scName" />,
            key: 'scName',
            render: () => {
              return (
                <ProForm.Item name={['pvcSelector', 'matchStorageClassName']}>
                  <Input />
                </ProForm.Item>
              )
            },
          },
          {
            title: <FormattedMessage id="pvcLabelMatch" />,
            key: 'pvcLabelMatch',
            render: () => {
              return (
                <ProFormList
                  name={['pvcSelector', 'matchLabels']}
                  creatorButtonProps={{
                    position: 'bottom',
                    creatorButtonText: 'New',
                  }}
                >
                  <ProForm.Group>
                    <ProFormText name={['key']}>
                      <Input placeholder="Key" />
                    </ProFormText>
                    <ProFormText name={['value']}>
                      <Input placeholder="Value" />
                    </ProFormText>
                  </ProForm.Group>
                </ProFormList>
              )
            },
          },
        ]}
      ></ProDescriptions>
    </ProCard>
  )
}

export default PVCSelectorForm
