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

import { ProForm, ProFormList, ProFormText } from '@ant-design/pro-components'
import { Input, Space } from 'antd'
import { FormattedMessage } from 'react-intl'

import PVCSelectorForm from '@/components/config/pvc-selector-form.tsx'

const MountPodPatchForm: React.FC = () => {
  return (
    <ProForm.Item>
      <Space direction="vertical">
        <PVCSelectorForm />

        <ProForm.Item
          name={'ceMountImage'}
          label={<FormattedMessage id="ceImage" />}
        >
          <Input />
        </ProForm.Item>
        <ProForm.Item
          name={'eeMountImage'}
          label={<FormattedMessage id="eeImage" />}
        >
          <Input />
        </ProForm.Item>

        <ProForm.Group>
          <ProFormList
            name={'labels'}
            label={<FormattedMessage id="labels" />}
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

          <ProFormList
            name={'annotations'}
            label={<FormattedMessage id="annotations" />}
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
        </ProForm.Group>

        <ProForm.Group>
          <ProFormList
            name={'mountOptions'}
            label={<FormattedMessage id="mountOptions" />}
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

          <ProFormList name={'env'} label={<FormattedMessage id="envs" />}>
            <ProForm.Group>
              <ProFormText name={'name'}>
                <Input placeholder="Name" />
              </ProFormText>

              <ProFormText name={'value'}>
                <Input placeholder="Value" />
              </ProFormText>
            </ProForm.Group>
          </ProFormList>
        </ProForm.Group>

        <ProForm.Group>
          <ProForm.Item
            name={['resources', 'requests']}
            label={<FormattedMessage id="resourceRequests" />}
          >
            <ProFormText name={['resources', 'requests', 'cpu']} label={'CPU'}>
              <Input />
            </ProFormText>
            <ProFormText
              name={['resources', 'requests', 'memory']}
              label={'Memory'}
            >
              <Input />
            </ProFormText>
          </ProForm.Item>
          <ProForm.Item
            name={['resources', 'limits']}
            label={<FormattedMessage id="resourceLimits" />}
          >
            <ProFormText name={['resources', 'limits', 'cpu']} label={'CPU'}>
              <Input />
            </ProFormText>
            <ProFormText
              name={['resources', 'limits', 'memory']}
              label={'Memory'}
            >
              <Input />
            </ProFormText>
          </ProForm.Item>
        </ProForm.Group>
      </Space>
    </ProForm.Item>
  )
}

export default MountPodPatchForm
