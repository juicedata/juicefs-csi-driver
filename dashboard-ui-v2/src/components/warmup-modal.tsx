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

import { memo, ReactNode, useEffect, useMemo, useState } from 'react'
import { FitAddon } from '@xterm/addon-fit'
import { Button, Checkbox, Input, InputNumber, Modal, Space } from 'antd'
import { ContainerStatus } from 'kubernetes-types/core/v1'
import { ReadyState } from 'react-use-websocket'
import { Terminal } from 'xterm'
import { Xterm } from 'xterm-react'

import { useWebsocket } from '@/hooks/use-api'
import { isEEImage } from '@/utils'

const EEhelpMessage = `Build cache for target directories/files

  - Threads         value  number of concurrent workers (default: 50)
  - IO retries      number of retries after failure (default: 1)
  - Max Failure     max number of allowed failed blocks (-1 for unlimited) (default: -1)
  - Background      run in background (default: false)
  - Check           check whether the data blocks are cached or not (default: false)
  - Subpath        subpath to warmup (default: "")

Click Start

---
`

const CEhelpMessage = `Build cache for target directories/files

  - Threads         value  number of concurrent workers (default: 50)
  - Background      run in background (default: false)
  - Check           check whether the data blocks are cached or not (default: false)
  - Subpath        subpath to warmup (default: "")

Click Start

---
`

const WarmupModal: React.FC<{
  namespace: string
  name: string
  container?: ContainerStatus
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ namespace, name, container, children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [start, setStart] = useState(false)
  const [showHelp, setShowHelp] = useState(true)
  const [terminal, setTerminal] = useState<Terminal | null>(null)

  const [threads, setThreads] = useState(50)
  const [ioRetries, setIoRetries] = useState(1)
  const [maxFailure, setMaxFailure] = useState(-1)
  const [background, setBackground] = useState(false)
  const [check, setCheck] = useState(false)
  const [subPath, setSubPath] = useState('')

  const fitAddon = useMemo(() => new FitAddon(), [])
  const isEE = container !== undefined && isEEImage(container.image)
  const helpMessage = isEE ? EEhelpMessage : CEhelpMessage

  const { readyState, sendJsonMessage } = useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/${container?.name}/warmup`,
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
      // warmup draws its progress by rewriting terminal lines, so its output
      // goes to a terminal emulator: a text editor would show every redraw as
      // another line.
      onMessage: (msg) => {
        terminal?.write(msg.data)
      },
    },
    isModalOpen && start,
  )

  useEffect(() => {
    if (readyState !== ReadyState.OPEN || !terminal) return
    // The command runs on a pty, which stays 0x0 until a size is sent. Without
    // it the progress display fits nothing and renders as just an ellipsis.
    fitAddon.fit()
    sendJsonMessage({ type: 'resize', cols: terminal.cols, rows: terminal.rows })
  }, [readyState, terminal, sendJsonMessage, fitAddon])

  const showModal = () => {
    setIsModalOpen(true)
    setStart(false)
    setShowHelp(true)
  }
  const handleOk = () => {
    setIsModalOpen(false)
  }
  const handleCancel = () => {
    setIsModalOpen(false)
    setStart(false)
    terminal?.dispose()
    setTerminal(null)
  }
  const handleStart = () => {
    // Drop what the previous run left on screen before streaming again.
    terminal?.reset()
    setShowHelp(false)
    setStart(true)
  }
  const handleReset = () => {
    setStart(false)
    setShowHelp(true)
  }

  const onTermInit = (term: Terminal) => {
    setTerminal(term)
    fitAddon.fit()
  }

  useEffect(() => {
    if (!terminal) return
    terminal.reset()
    // The help stays on screen after the command ends, and comes back only
    // when the view is reset.
    if (showHelp) {
      // xterm only returns to the first column on a carriage return.
      terminal.write(helpMessage.replace(/\n/g, '\r\n'))
    }
  }, [terminal, showHelp, helpMessage])

  useEffect(() => {
    if (!isModalOpen) return
    const fit = () => fitAddon.fit()
    window.addEventListener('resize', fit)
    return () => window.removeEventListener('resize', fit)
  }, [isModalOpen, fitAddon])

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Warmup: ${namespace}/${name}/${container?.name ?? ''}`}
          open={isModalOpen}
          width={800}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space>
                <InputNumber
                  addonBefore="Threads"
                  value={threads}
                  onChange={(v) => v && setThreads(v)}
                />
                {isEE ? (
                  <>
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
                  </>
                ) : null}

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
                  <Button onClick={handleStart} disabled={start} type="primary">
                    Start
                  </Button>
                  <Button onClick={handleReset} disabled={showHelp}>
                    Reset
                  </Button>
                </Space>
              </Space>
            </div>
          )}
          onOk={handleOk}
          onCancel={handleCancel}
        >
          <Xterm
            className="xterm-container"
            onInit={onTermInit}
            addons={[fitAddon]}
          />
        </Modal>
      ) : null}
    </>
  )
})

export default WarmupModal
