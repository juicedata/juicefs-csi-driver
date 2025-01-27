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

import { useEffect, useState } from 'react'
import { ProCard } from '@ant-design/pro-components'
import Editor from '@monaco-editor/react'
import { Alert } from 'antd'
import { FormattedMessage } from 'react-intl'

const ConfigYamlPage: React.FC<{
  error?: string
  setError: (message: string) => void
  setUpdated: (updated: boolean) => void
  setConfigData: (configData: string) => void
  configData?: string
  edit: boolean
}> = (props) => {
  const { error, setError, setUpdated, setConfigData, configData, edit } = props

  const [data, setData] = useState('')

  useEffect(() => {
    if (configData) {
      setData(configData)
    }
  }, [configData])

  return (
    <ProCard>
      {error && (
        <Alert
          message={<FormattedMessage id="updateConfigError" />}
          description={error}
          type="error"
          showIcon
          style={{ marginTop: '10px' }}
          onClick={() => setError('')}
        />
      )}

      <Editor
        defaultLanguage="yaml"
        height="calc(100vh - 200px)"
        options={{
          wordWrap: 'on',
          theme: 'vs-light', // TODO dark mode
          scrollBeyondLastLine: false,
          readOnly: !edit,
          cursorStyle: edit ? 'line' : 'block',
        }}
        value={data}
        onChange={(v) => {
          if (v) {
            // setConfigData(YAML.stringify(YAML.parse(v)))
            setConfigData(v)
            setUpdated(true)
            setError('')
          }
        }}
      />
    </ProCard>
  )
}

export default ConfigYamlPage
