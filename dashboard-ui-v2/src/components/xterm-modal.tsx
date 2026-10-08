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
import React, { ReactNode, useEffect, useRef, useState } from 'react'
import { DownloadOutlined, UploadOutlined } from '@ant-design/icons'
import { FitAddon } from '@xterm/addon-fit'
import { Button, Input, Modal, Space } from 'antd'
import { Terminal } from 'xterm'
import { Xterm } from 'xterm-react'
import Zmodem, { type Session } from 'zmodem.js/src/zmodem_browser'

import { triggerBlobDownload, useWebsocket } from '@/hooks/use-api'

const XTermModal: React.FC<{
  namespace: string
  name: string
  container: string
  enableFileTransfer?: boolean
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = ({
  namespace,
  name,
  container,
  enableFileTransfer = false,
  children,
}) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [isDownloadOpen, setIsDownloadOpen] = useState(false)
  const [isTransferActive, setIsTransferActive] = useState(false)
  const [isWaitingForFiles, setIsWaitingForFiles] = useState(false)
  const [downloadPath, setDownloadPath] = useState('')
  const [terminal, setTerminal] = useState<Terminal | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const sentryRef = useRef<InstanceType<typeof Zmodem.Sentry> | null>(null)
  const sessionRef = useRef<Session | null>(null)
  const pendingFilesRef = useRef<File[]>([])
  const pendingOutputRef = useRef<ArrayBuffer[]>([])
  const transferTimerRef = useRef<number | null>(null)
  const fitAddon = React.useMemo(() => new FitAddon(), [])

  const showModal = () => setIsModalOpen(true)
  const handleCancel = () => {
    if (transferTimerRef.current !== null) {
      window.clearTimeout(transferTimerRef.current)
      transferTimerRef.current = null
    }
    sessionRef.current?.abort()
    sessionRef.current = null
    pendingFilesRef.current = []
    pendingOutputRef.current = []
    setIsTransferActive(false)
    setIsWaitingForFiles(false)
    setIsModalOpen(false)
    setIsDownloadOpen(false)
    setDownloadPath('')
    terminal?.dispose()
    setTerminal(null)
  }
  const onTermInit = (term: Terminal) => {
    setTerminal(term)
    term.reset()
  }

  const { sendJsonMessage, sendMessage } = useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/${container}/exec`,
    {
      onOpen: (event) => {
        ;(event.currentTarget as WebSocket).binaryType = 'arraybuffer'
        fitAddon.fit()
        terminal?.focus()
      },
      onMessage: (event) => {
        if (event.data instanceof ArrayBuffer) {
          if (sentryRef.current) {
            sentryRef.current.consume(event.data)
          } else {
            pendingOutputRef.current.push(event.data.slice(0))
          }
        } else if (typeof event.data === 'string') {
          terminal?.write(event.data)
        }
      },
      onClose: () => {
        if (transferTimerRef.current !== null) {
          window.clearTimeout(transferTimerRef.current)
          transferTimerRef.current = null
        }
        sessionRef.current = null
        pendingFilesRef.current = []
        setIsTransferActive(false)
        setIsWaitingForFiles(false)
        terminal?.write('\r\n\r\nConnection closed.\r\n')
      },
      onError: (error) => {
        terminal?.write(`\r\n\r\n${error}\r\n`)
      },
    },
    isModalOpen,
  )

  const sendFiles = React.useCallback(
    (session: Session, files: File[]) => {
      setIsWaitingForFiles(false)
      Zmodem.Browser.send_files(session, files)
        .then(() => session.close())
        .catch((error: unknown) => {
          session.abort()
          sessionRef.current = null
          setIsTransferActive(false)
          setIsWaitingForFiles(false)
          terminal?.write(`\r\nFile upload failed: ${String(error)}\r\n`)
        })
    },
    [terminal],
  )

  useEffect(() => {
    if (!isModalOpen || !terminal) return

    const sentry = new Zmodem.Sentry({
      to_terminal: (octets) => terminal.write(Uint8Array.from(octets)),
      sender: (octets) => sendMessage(Uint8Array.from(octets), false),
      on_detect: (detection) => {
        if (transferTimerRef.current !== null) {
          window.clearTimeout(transferTimerRef.current)
          transferTimerRef.current = null
        }
        const session = detection.confirm()
        sessionRef.current = session
        setIsTransferActive(true)
        session.on('session_end', () => {
          sessionRef.current = null
          setIsTransferActive(false)
          setIsWaitingForFiles(false)
        })
        if (session.type === 'send') {
          if (pendingFilesRef.current.length > 0) {
            const files = pendingFilesRef.current
            pendingFilesRef.current = []
            sendFiles(session, files)
          } else {
            setIsWaitingForFiles(true)
            terminal.write('\r\nSelect files with the Upload button.\r\n')
          }
          return
        }
        setIsWaitingForFiles(false)
        session.on('offer', (offer) => {
          offer
            .accept()
            .then((payloads) => {
              triggerBlobDownload(
                new Blob(payloads),
                offer.get_details().name,
              )
            })
            .catch((error: unknown) => {
              session.abort()
              sessionRef.current = null
              setIsTransferActive(false)
              setIsWaitingForFiles(false)
              terminal.write(`\r\nFile download failed: ${String(error)}\r\n`)
            })
        })
        session.start()
      },
      on_retract: () => undefined,
    })
    sentryRef.current = sentry
    pendingOutputRef.current.forEach((data) => sentry.consume(data))
    pendingOutputRef.current = []

    return () => {
      sentryRef.current = null
      sessionRef.current = null
    }
  }, [isModalOpen, sendFiles, sendMessage, terminal])

  useEffect(() => {
    if (!isModalOpen) return
    const fit = () => fitAddon.fit()
    window.addEventListener('resize', fit)
    return () => window.removeEventListener('resize', fit)
  }, [fitAddon, isModalOpen])

  const upload = (files: FileList | null) => {
    const selected = Array.from(files ?? [])
    if (selected.length === 0) return
    const session = sessionRef.current
    if (session?.type === 'send') {
      sendFiles(session, selected)
    } else {
      pendingFilesRef.current = selected
      setIsTransferActive(true)
      if (transferTimerRef.current !== null) {
        window.clearTimeout(transferTimerRef.current)
      }
      transferTimerRef.current = window.setTimeout(() => {
        pendingFilesRef.current = []
        transferTimerRef.current = null
        setIsTransferActive(false)
        setIsWaitingForFiles(false)
        terminal?.write('\r\nUpload failed: rz did not start.\r\n')
      }, 30_000)
      sendJsonMessage({ type: 'stdin', data: 'rz\r' })
    }
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  const cancelTransfer = () => {
    if (transferTimerRef.current !== null) {
      window.clearTimeout(transferTimerRef.current)
      transferTimerRef.current = null
    }
    pendingFilesRef.current = []
    if (sessionRef.current) {
      sessionRef.current.abort()
      sessionRef.current = null
    } else {
      sendJsonMessage({ type: 'stdin', data: '\u0003' })
    }
    setIsTransferActive(false)
    setIsWaitingForFiles(false)
    terminal?.focus()
  }

  const download = () => {
    const path = downloadPath.trim()
    if (!path) return
    const quotedPath = `'${path.replace(/'/g, `'\\''`)}'`
    setIsTransferActive(true)
    transferTimerRef.current = window.setTimeout(() => {
      transferTimerRef.current = null
      setIsTransferActive(false)
      setIsWaitingForFiles(false)
      terminal?.write('\r\nDownload failed: sz did not start.\r\n')
    }, 30_000)
    sendJsonMessage({ type: 'stdin', data: `sz -- ${quotedPath}\r` })
    setIsDownloadOpen(false)
    setDownloadPath('')
    terminal?.focus()
  }

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Exec: ${namespace}/${name}/${container}`}
          open={isModalOpen}
          footer={null}
          onCancel={handleCancel}
          width={800}
        >
          {enableFileTransfer ? (
            <Space style={{ marginBottom: 12 }}>
              <Button
                icon={<UploadOutlined />}
                disabled={isTransferActive && !isWaitingForFiles}
                onClick={() => fileInputRef.current?.click()}
              >
                Upload
              </Button>
              <Button
                icon={<DownloadOutlined />}
                disabled={isTransferActive}
                onClick={() => setIsDownloadOpen(true)}
              >
                Download
              </Button>
              {isTransferActive ? (
                <Button danger onClick={cancelTransfer}>
                  Cancel transfer
                </Button>
              ) : null}
              <input
                ref={fileInputRef}
                type="file"
                multiple
                hidden
                onChange={(event) => upload(event.target.files)}
              />
            </Space>
          ) : null}
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
          <Modal
            title="Download file from pod"
            open={isDownloadOpen}
            onOk={download}
            onCancel={() => setIsDownloadOpen(false)}
            okButtonProps={{ disabled: !downloadPath.trim() }}
          >
            <Input
              autoFocus
              placeholder="Absolute or relative file path"
              value={downloadPath}
              onChange={(event) => setDownloadPath(event.target.value)}
              onPressEnter={download}
            />
          </Modal>
        </Modal>
      ) : null}
    </>
  )
}

export default XTermModal
