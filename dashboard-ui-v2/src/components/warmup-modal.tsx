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
import { Button, Checkbox, Input, InputNumber, Modal, Space } from 'antd'

import { useWebsocket } from '@/hooks/use-api'

const helpMessage = `Build cache for target directories/files

  - Threads         value  number of concurrent workers (default: 50)
  - IO retries      number of retries after failure (default: 1)
  - Max Failure     max number of allowed failed blocks (-1 for unlimited) (default: -1)
  - Background      run in background (default: false)
  - Check           check whether the data blocks are cached or not (default: false)

Click Start

---
`

function removeAnsiSequences(text: string) {
  // eslint-disable-next-line no-control-regex
  return text.replace(/\x1b\[[0-9;]*[A-Za-z]/g, '')
}

const WarmupModal: React.FC<{
  namespace: string
  name: string
  container: string
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ namespace, name, container, children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [data, setData] = useState<string>(helpMessage)
  const [start, setStart] = useState(false)

  const [threads, setThreads] = useState(50)
  const [ioRetries, setIoRetries] = useState(1)
  const [maxFailure, setMaxFailure] = useState(-1)
  const [background, setBackground] = useState(false)
  const [check, setCheck] = useState(false)
  const [subPath, setSubPath] = useState('')

  useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/${container}/warmup`,
    {
      queryParams: {
        threads,
        ioRetries,
        maxFailure,
        background: background ? 'true' : 'false',
        check: check ? 'true' : 'false',
        subPath,
      },
      onClose: () => {
        setStart(false)
      },
      onMessage: (msg) => {
        // ignore and remove ANSI escape sequences
        if (msg.data.includes('…')) {
          return
        }
        setData((prev) => prev + removeAnsiSequences(msg.data))
      },
    },
    isModalOpen && start,
  )

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
      setData(helpMessage)
    }
  }, [isModalOpen])

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Warmup: ${namespace}/${name}/${container}`}
          open={isModalOpen}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space>
                <InputNumber
                  addonBefore="Threads"
                  value={threads}
                  onChange={(v) => v && setThreads(v)}
                />
                <InputNumber
                  addonBefore="IO retries"
                  value={ioRetries}
                  onChange={(v) => v && setIoRetries(v)}
                />
                <InputNumber
                  addonBefore="Max failure"
                  value={maxFailure}
                  onChange={(v) => v && setMaxFailure(v)}
                />
                <Input
                  addonBefore="subpath"
                  value={subPath}
                  onChange={(v) => v && setSubPath(v.target.value)}
                />
                <Checkbox
                  checked={background}
                  onChange={(v) => v && setBackground(v.target.checked)}
                >
                  Background
                </Checkbox>
                <Checkbox
                  checked={check}
                  onChange={(v) => v && setCheck(v.target.checked)}
                >
                  Check
                </Checkbox>

                <Space style={{ textAlign: 'end' }}>
                  <Button
                    onClick={() => {
                      setStart(true)
                    }}
                    disabled={start}
                    type="primary"
                  >
                    Start
                  </Button>
                </Space>
              </Space>
            </div>
          )}
          onOk={handleOk}
          onCancel={handleCancel}
        >
          <Editor
            language="shell"
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
})

export default WarmupModal
