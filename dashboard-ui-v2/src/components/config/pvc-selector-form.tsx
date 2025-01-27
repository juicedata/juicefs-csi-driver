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
import { Input } from 'antd'

const PVCSelectorForm = () => {
  return (
    <ProForm.Item name={'pvcSelector'} label="PVCSelector">
      <ProForm.Group>
        <ProForm.Item
          name={['pvcSelector', 'matchStorageClassName']}
          label="Match StorageClass Name"
        >
          <Input />
        </ProForm.Item>

        <ProForm.Item name={['pvcSelector', 'matchName']} label="Match Name">
          <Input />
        </ProForm.Item>
      </ProForm.Group>
      <ProFormList
        name={['pvcSelector', 'matchLabels']}
        label="Match Labels"
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
        name={['pvcSelector', 'matchExpressions']}
        label="Match Expressions"
        creatorButtonProps={{
          position: 'bottom',
          creatorButtonText: 'New',
        }}
      >
        <ProForm.Group>
          <ProFormText name={['key']}>
            <Input placeholder="Key" />
          </ProFormText>
          <ProFormText name={['operator']}>
            <Input placeholder="Operator" />
          </ProFormText>
          <ProFormList name={['values']}>
            <ProFormText name={['value']}>
              <Input placeholder="Value" />
            </ProFormText>
          </ProFormList>
        </ProForm.Group>
      </ProFormList>
    </ProForm.Item>
  )
}

export default PVCSelectorForm
