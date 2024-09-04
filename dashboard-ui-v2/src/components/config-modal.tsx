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

import { memo, ReactNode, useEffect, useState } from 'react'
import Editor from '@monaco-editor/react'
import { Button, Modal, Space } from 'antd'
import { FormattedMessage } from 'react-intl'
import YAML, { YAMLParseError } from 'yaml'

import { useConfig, useUpdateConfig } from '@/hooks/cm-api'

const ConfigModal: React.FC<{
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)

  const showModal = () => {
    setIsModalOpen(true)
  }
  const handleOk = () => {
    setIsModalOpen(false)
  }
  const handleCancel = () => {
    setIsModalOpen(false)
  }

  const [updated, setUpdated] = useState(false)

  const { data, isLoading, mutate } = useConfig()
  const [state, actions] = useUpdateConfig()
  const [config, setConfig] = useState('')
  useEffect(() => {
    if (data?.data) {
      try {
        setConfig(YAML.stringify(YAML.parse(data?.data?.['config.yaml'])))
      } catch (e) {
        setConfig((e as YAMLParseError).message)
      }
    }
  }, [data])

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={'Config'}
          open={isModalOpen}
          footer={() => (
            <Space>
              <Button
                onClick={() => {
                  window.open(
                    'https://juicefs.com/docs/zh/csi/guide/configurations',
                    '_blank',
                  )
                }}
              >
                <FormattedMessage id="docs" />
              </Button>
              <Button
                loading={isLoading}
                disabled={!updated}
                onClick={() => {
                  mutate()
                  if (data?.data) {
                    setConfig(
                      YAML.stringify(data?.data?.['config.yaml'])
                        .split('\n')
                        .slice(1)
                        .join('\n'),
                    )
                    setUpdated(false)
                  }
                }}
              >
                <FormattedMessage id="reset" />
              </Button>
              <Button
                type="primary"
                disabled={!updated}
                loading={state.status === 'loading'}
                onClick={() => {
                  actions.execute({
                    ...data,
                    data: {
                      'config.yaml': config,
                    },
                  })
                  setUpdated(false)
                }}
              >
                <FormattedMessage id="save" />
              </Button>
            </Space>
          )}
          onOk={handleOk}
          onCancel={handleCancel}
        >
          <Editor
            defaultLanguage="yaml"
            options={{
              wordWrap: 'on',
              theme: 'vs-light', // TODO dark mode
              scrollBeyondLastLine: false,
            }}
            value={config}
            onChange={(v) => {
              if (v) {
                setConfig(v)
                setUpdated(true)
              }
            }}
          />
        </Modal>
      ) : null}
    </>
  )
})

export default ConfigModal
