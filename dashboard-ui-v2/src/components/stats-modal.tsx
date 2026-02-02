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
import { Button, Checkbox, Modal, Space } from 'antd'
import { ContainerStatus } from 'kubernetes-types/core/v1'
import { editor } from 'monaco-editor'

import { useWebsocket } from '@/hooks/use-api'
import { isEEImage } from '@/utils'
import { createAnsiStrippedMessageHandlerWithCallback } from '@/utils/ansi'

const EEhelpMessage = `Real-time statistics of the target mount point (Enterprise Edition)

Schema sections (t: time, u: usage, f: fuse, m: meta, c: blockcache, r: remotecache, o: object, a: readahead):
  - Time (t): Timestamp
  - Usage (u): CPU, Memory, Uptime
  - Fuse (f): FUSE operations statistics
  - Meta (m): Metadata operations statistics
  - BlockCache (c): Block cache statistics
  - RemoteCache (r): Remote cache statistics
  - Object (o): Object storage operations statistics
  - Readahead (a): Readahead statistics

Select the sections you want to see and click Start to view real-time stats.

---
`

const CEhelpMessage = `Real-time statistics of the target mount point (Community Edition)

Schema sections (t: time, u: usage, f: fuse, m: meta, c: blockcache, o: object, g: go):
  - Time (t): Timestamp
  - Usage (u): CPU, Memory, Uptime
  - Fuse (f): FUSE operations statistics
  - Meta (m): Metadata operations statistics
  - BlockCache (c): Block cache statistics
  - Object (o): Object storage operations statistics
  - Go (g): Go runtime statistics

Select the sections you want to see and click Start to view real-time stats.

---
`

const StatsModal: React.FC<{
  namespace: string
  name: string
  container: ContainerStatus
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ namespace, name, container, children }) => {
  const isEE = isEEImage(container.image)
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [data, setData] = useState<string>(isEE ? EEhelpMessage : CEhelpMessage)
  const [start, setStart] = useState(false)
  const [editor, setEditor] = useState<editor.IStandaloneCodeEditor | null>(
    null,
  )
  const [autoReveal, setAutoReveal] = useState<boolean>(true)

  // Schema checkboxes
  const [showTime, setShowTime] = useState(false)
  const [showUsage, setShowUsage] = useState(true)
  const [showFuse, setShowFuse] = useState(true)
  const [showMeta, setShowMeta] = useState(true)
  const [showBlockCache, setShowBlockCache] = useState(true)
  const [showRemoteCache, setShowRemoteCache] = useState(true)
  const [showObject, setShowObject] = useState(true)
  const [showReadahead, setShowReadahead] = useState(false)
  const [showGo, setShowGo] = useState(false)

  const getSchema = () => {
    let schema = ''
    if (showTime) schema += 't'
    if (showUsage) schema += 'u'
    if (showFuse) schema += 'f'
    if (showMeta) schema += 'm'
    if (showBlockCache) schema += 'c'
    if (isEE) {
      if (showRemoteCache) schema += 'r'
    }
    if (showObject) schema += 'o'
    if (isEE) {
      if (showReadahead) schema += 'a'
    } else {
      if (showGo) schema += 'g'
    }
    return schema || (isEE ? 'ufmcro' : 'ufmco')
  }

  useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/${container.name}/stats`,
    {
      queryParams: {
        schema: getSchema(),
      },
      onClose: () => {
        setStart(false)
      },
      onMessage: createAnsiStrippedMessageHandlerWithCallback(setData, () => {
        if (editor) {
          const model = editor.getModel()
          if (!model) return
          const visibleLine = editor.getVisibleRanges()[0]
          if (visibleLine.endLineNumber < model.getLineCount()) {
            setAutoReveal(false)
          } else {
            setAutoReveal(true)
          }
        }
      }),
    },
    isModalOpen && start,
  )

  useEffect(() => {
    if (!editor) return
    const model = editor.getModel()
    if (!model) return
    if (autoReveal) {
      editor.revealLine(model.getLineCount())
    }
  }, [data, editor, autoReveal])

  const showModal = () => {
    setIsModalOpen(true)
    setStart(false)
  }
  const handleOk = () => {
    setIsModalOpen(false)
  }
  const handleCancel = () => {
    setIsModalOpen(false)
  }

  useEffect(() => {
    if (!isModalOpen) {
      setData(isEE ? EEhelpMessage : CEhelpMessage)
    }
  }, [isModalOpen, isEE])

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Stats: ${namespace}/${name}/${container.name}`}
          open={isModalOpen}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space wrap>
                <Checkbox
                  checked={showTime}
                  onChange={(e) => setShowTime(e.target.checked)}
                >
                  Time
                </Checkbox>
                <Checkbox
                  checked={showUsage}
                  onChange={(e) => setShowUsage(e.target.checked)}
                >
                  Usage
                </Checkbox>
                <Checkbox
                  checked={showFuse}
                  onChange={(e) => setShowFuse(e.target.checked)}
                >
                  Fuse
                </Checkbox>
                <Checkbox
                  checked={showMeta}
                  onChange={(e) => setShowMeta(e.target.checked)}
                >
                  Meta
                </Checkbox>
                <Checkbox
                  checked={showBlockCache}
                  onChange={(e) => setShowBlockCache(e.target.checked)}
                >
                  BlockCache
                </Checkbox>
                {isEE ? (
                  <>
                    <Checkbox
                      checked={showRemoteCache}
                      onChange={(e) => setShowRemoteCache(e.target.checked)}
                    >
                      RemoteCache
                    </Checkbox>
                    <Checkbox
                      checked={showObject}
                      onChange={(e) => setShowObject(e.target.checked)}
                    >
                      Object
                    </Checkbox>
                    <Checkbox
                      checked={showReadahead}
                      onChange={(e) => setShowReadahead(e.target.checked)}
                    >
                      Readahead
                    </Checkbox>
                  </>
                ) : (
                  <>
                    <Checkbox
                      checked={showObject}
                      onChange={(e) => setShowObject(e.target.checked)}
                    >
                      Object
                    </Checkbox>
                    <Checkbox
                      checked={showGo}
                      onChange={(e) => setShowGo(e.target.checked)}
                    >
                      Go
                    </Checkbox>
                  </>
                )}

                <Space style={{ marginLeft: 'auto' }}>
                  <Button
                    onClick={() => {
                      setData(isEE ? EEhelpMessage : CEhelpMessage)
                      setStart(true)
                    }}
                    type="primary"
                  >
                    {start ? 'Refresh' : 'Start'}
                  </Button>
                  <Button
                    onClick={() => {
                      setStart(false)
                      handleCancel()
                    }}
                  >
                    Close
                  </Button>
                </Space>
              </Space>
            </div>
          )}
          onOk={handleOk}
          onCancel={handleCancel}
        >
          <Editor
            defaultLanguage="yaml"
            onMount={(editor) => {
              setEditor(editor)
            }}
            value={data}
            options={{
              readOnly: true,
              theme: 'vs-light',
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              wordWrap: 'off',
            }}
          />
        </Modal>
      ) : null}
    </>
  )
})

export default StatsModal
