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

import PVCSelectorForm from '@/components/config/pvc-selector-form.tsx'
import { mountPodPatch } from '@/types/config.ts'

const MountPodPatchForm: React.FC<{
  patch?: mountPodPatch
}> = (props) => {
  const { patch } = props

  return (
    <ProForm.Item>
      <PVCSelectorForm patch={patch} />

      <ProCard title={<FormattedMessage id="basicPatch" />}>
        <ProDescriptions
          column={2}
          dataSource={patch}
          columns={[
            {
              title: <FormattedMessage id="ceImage" />,
              key: 'ceMountImage',
              render: () => {
                return (
                  <ProForm.Item
                    name={'ceMountImage'}
                    className={'patch-form-item'}
                  >
                    <Input />
                  </ProForm.Item>
                )
              },
            },
            {
              title: <FormattedMessage id="eeImage" />,
              key: 'eeMountImage',
              render: () => {
                return (
                  <ProForm.Item
                    name={'eeMountImage'}
                    className="patch-form-item"
                  >
                    <Input />
                  </ProForm.Item>
                )
              },
            },
            {
              title: <FormattedMessage id="labels" />,
              key: 'labels',
              render: () => {
                return (
                  <ProFormList
                    name={'labels'}
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
            {
              title: <FormattedMessage id="annotations" />,
              key: 'annotations',
              render: () => {
                return (
                  <ProFormList
                    name={'annotations'}
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
            {
              title: <FormattedMessage id="mountOptions" />,
              key: 'mountOptions',
              render: () => (
                <ProFormList
                  name={'mountOptions'}
                  creatorButtonProps={{
                    position: 'bottom',
                    creatorButtonText: 'New',
                  }}
                >
                  <ProForm.Group>
                    <ProFormText name={['value']}>
                      <Input placeholder="Value" />
                    </ProFormText>
                  </ProForm.Group>
                </ProFormList>
              ),
            },
            {
              title: <FormattedMessage id="envs" />,
              key: 'envs',
              render: () => (
                <ProFormList
                  name={'env'}
                  creatorButtonProps={{
                    position: 'bottom',
                    creatorButtonText: 'New',
                  }}
                >
                  <ProForm.Group>
                    <ProFormText name={'name'}>
                      <Input placeholder="Name" />
                    </ProFormText>

                    <ProFormText name={'value'}>
                      <Input placeholder="Value" />
                    </ProFormText>
                  </ProForm.Group>
                </ProFormList>
              ),
            },
            {
              title: <FormattedMessage id="resourceRequests" />,
              key: 'resourceRequests',
              render: () => (
                <ProForm.Item name={['resources', 'requests']}>
                  <ProFormText
                    name={['resources', 'requests', 'cpu']}
                    label={'CPU'}
                  >
                    <Input />
                  </ProFormText>
                  <ProFormText
                    name={['resources', 'requests', 'memory']}
                    label={'Memory'}
                  >
                    <Input />
                  </ProFormText>
                </ProForm.Item>
              ),
            },
            {
              title: <FormattedMessage id="resourceLimits" />,
              key: 'resourceLimits',
              render: () => (
                <ProForm.Item name={['resources', 'limits']}>
                  <ProFormText
                    name={['resources', 'limits', 'cpu']}
                    label={'CPU'}
                  >
                    <Input />
                  </ProFormText>
                  <ProFormText
                    name={['resources', 'limits', 'memory']}
                    label={'Memory'}
                  >
                    <Input />
                  </ProFormText>
                </ProForm.Item>
              ),
            },
          ]}
        />
      </ProCard>
    </ProForm.Item>
  )
}

export default MountPodPatchForm
