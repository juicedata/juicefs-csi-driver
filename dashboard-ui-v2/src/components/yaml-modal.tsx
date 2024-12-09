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
import Editor from '@monaco-editor/react'
import { Button, Modal, Space } from 'antd'

const YamlModal: React.FC<{
  isOpen: boolean
  onClose: () => void
  content: string
  editable?: boolean
  onSave?: (data: string) => void
  saveButtonText?: string
}> = ({ isOpen, onClose, content, editable, onSave, saveButtonText }) => {
  const [data, setData] = useState(content)
  return (
    <Modal
      title="YAML"
      open={isOpen}
      onCancel={onClose}
      footer={
        editable ? (
          <Space>
            <Button type="primary" onClick={() => onSave && onSave(data)}>
              {saveButtonText ?? 'Save'}
            </Button>
          </Space>
        ) : null
      }
    >
      <Editor
        defaultLanguage="yaml"
        options={{
          wordWrap: 'on',
          readOnly: !editable,
          theme: 'vs-light', // TODO dark mode
          scrollBeyondLastLine: false,
        }}
        onChange={(value) => {
          setData(value ?? '')
        }}
        value={content}
      />
    </Modal>
  )
}

export default YamlModal
