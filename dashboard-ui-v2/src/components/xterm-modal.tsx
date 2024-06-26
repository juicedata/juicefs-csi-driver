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
import React, { ReactNode, useEffect, useState } from 'react'
import { FitAddon } from '@xterm/addon-fit'
import { Modal } from 'antd'
import { Terminal } from 'xterm'
import { Xterm } from 'xterm-react'

import { useWebsocket } from '@/hooks/use-api'

const XTermModal: React.FC<{
  namespace: string
  name: string
  container: string
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = ({ namespace, name, container, children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const showModal = () => {
    setIsModalOpen(true)
  }
  const handleOk = () => {
    setIsModalOpen(false)
  }
  const [terminal, setTerminal] = useState<Terminal | null>(null)
  const onTermInit = (term: Terminal) => {
    setTerminal(term)
    term?.reset()
  }
  const handleCancel = () => {
    setIsModalOpen(false)
    terminal?.dispose()
    setTerminal(null)
  }
  const fitAddon = React.useMemo(() => new FitAddon(), [])
  const { sendJsonMessage } = useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/${container}/exec`,
    {
      onOpen: () => {
        fitAddon.fit()
        terminal?.focus()
      },
      onMessage: (data) => {
        terminal?.write(data.data)
      },
      onClose: () => {
        terminal?.write('\r\n\r\nConnection closed.\r\n')
      },
      onError: (error) => {
        terminal?.write(`\r\n\r\n${error}\r\n`)
      },
    },
    isModalOpen,
  )

  useEffect(() => {
    if (isModalOpen) {
      window.addEventListener('resize', fitAddon.fit)
    }
    return () => {
      window.removeEventListener('resize', fitAddon.fit)
    }
  }, [fitAddon, isModalOpen, terminal])

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Exec: ${namespace}/${name}/${container}`}
          open={isModalOpen}
          footer={null}
          onOk={handleOk}
          onCancel={handleCancel}
        >
          <Xterm
            className="xterm-container"
            onInit={onTermInit}
            onResize={(event) => {
              fitAddon.fit()
              sendJsonMessage({
                type: 'resize',
                cols: event.cols,
                rows: event.rows,
              })
            }}
            addons={[fitAddon]}
            onData={(data) => {
              sendJsonMessage({
                type: 'stdin',
                data,
              })
            }}
          />
        </Modal>
      ) : null}
    </>
  )
}

export default XTermModal
