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
import { editor } from 'monaco-editor'

import { useDownloadPodLogs, useWebsocket } from '@/hooks/use-api'

const LogModal: React.FC<{
  namespace: string
  name: string
  container: string
  hasPrevious?: boolean
  type?: 'logs' | 'accesslog'
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(
  ({ namespace, name, container, hasPrevious, children, type = 'logs' }) => {
    const [isModalOpen, setIsModalOpen] = useState(false)
    const [data, setData] = useState<string>('')
    const [previous, setPrevious] = useState<boolean>(false)
    const [editor, setEditor] = useState<editor.IStandaloneCodeEditor | null>(
      null,
    )
    const [state, actions] = useDownloadPodLogs(namespace, name, container)

    useWebsocket(
      `/api/v1/ws/pod/${namespace}/${name}/${container}/${type}`,
      {
        queryParams: {
          previous: previous ? 'true' : 'false',
        },
        onMessage: (msg) => {
          setData((prev) => prev + msg.data)
        },
      },
      isModalOpen,
    )

    useEffect(() => {
      if (!editor) return
      const model = editor.getModel()
      if (!model) return
      const visibleLine = editor.getVisibleRanges()[0]
      if (visibleLine.endLineNumber + 5 < model.getLineCount()) {
        return
      }
      editor.revealLine(model.getLineCount())
    }, [data, editor])

    const showModal = () => {
      setIsModalOpen(true)
    }
    const handleOk = () => {
      setIsModalOpen(false)
    }
    const handleCancel = () => {
      setIsModalOpen(false)
    }

    useEffect(() => {
      if (!isModalOpen) {
        setData('')
      }
    }, [isModalOpen])

    return (
      <>
        {children({ onClick: showModal })}
        {isModalOpen ? (
          <Modal
            title={`${type == 'accesslog' ? 'Access Log' : 'Log'}: ${namespace}/${name}/${container}`}
            open={isModalOpen}
            footer={() => (
              <Space>
                <Button onClick={() => setData('')}> Clear </Button>
                {type === 'logs' ? (
                  <>
                    <Button
                      loading={state.status === 'loading'}
                      onClick={() => {
                        actions.execute()
                        console.log('Download full log')
                      }}
                    >
                      Download full log
                    </Button>
                    <Button
                      onClick={() => {
                        setPrevious(!previous)
                        setData('')
                      }}
                      disabled={!hasPrevious}
                    >
                      {previous ? 'Show current log' : 'Show previous log'}
                    </Button>
                  </>
                ) : null}
              </Space>
            )}
            onOk={handleOk}
            onCancel={handleCancel}
          >
            <Editor
              defaultLanguage="yaml"
              onMount={(editor) => {
                setEditor(editor)
              }}
              options={{
                wordWrap: 'on',
                readOnly: true,
                theme: 'vs-light', // TODO dark mode
                scrollBeyondLastLine: false,
              }}
              value={data}
            />
          </Modal>
        ) : null}
      </>
    )
  },
)

export default LogModal
