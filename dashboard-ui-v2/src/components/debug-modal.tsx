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
import { Button, InputNumber, Modal, Space } from 'antd'
import { max } from 'lodash'
import { useCountdown } from 'usehooks-ts'

import { useDownloadPodDebugFiles, useWebsocket } from '@/hooks/use-api'

const helpMessage = `Collect and display system static and runtime information

- stats-sec value    stats sampling duration (default: 5)
- trace-sec value    trace sampling duration (default: 5)
- profile-sec value  profile sampling duration (default: 30)

Click Start to collect the information, and click Download to download the collected all files.

---
`

const DebugModal: React.FC<{
  namespace: string
  name: string
  container: string
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ namespace, name, container, children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [collecting, setCollecting] = useState(false)
  const [data, setData] = useState<string>(helpMessage)
  const [canDownload, setCanDownload] = useState(false)
  const [profileSec, setProfileSec] = useState(30)
  const [traceSec, setTraceSec] = useState(5)
  const [statsSec, setStatsSec] = useState(5)
  const [state, actions] = useDownloadPodDebugFiles(namespace, name)

  useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/${container}/debug`,
    {
      queryParams: {
        statsSec,
        traceSec,
        profileSec,
      },
      onMessage: (msg) => {
        if (msg.data.includes('All files are collected to')) {
          setCanDownload(true)
        }
        setData((prev) => prev + msg.data)
      },
      onClose: () => {
        setCollecting(false)
      },
    },
    isModalOpen && collecting,
  )

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
      setData(helpMessage)
      setCanDownload(false)
    }
  }, [isModalOpen])

  const [countStart, setCountStart] = useState(30)
  const [count, { startCountdown, resetCountdown }] = useCountdown({
    countStart,
    intervalMs: 1000,
  })

  useEffect(() => {
    setCountStart(max([profileSec, traceSec, statsSec]) || 30)
  }, [profileSec, traceSec, statsSec, resetCountdown])

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Debug: ${namespace}/${name}/${container}`}
          open={isModalOpen}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space>
                <InputNumber
                  addonBefore="Stats Sec"
                  value={statsSec}
                  disabled={collecting}
                  onChange={(v) => v && setStatsSec(v)}
                />
                <InputNumber
                  addonBefore="Trace Sec"
                  value={traceSec}
                  disabled={collecting}
                  onChange={(v) => v && setTraceSec(v)}
                />
                <InputNumber
                  addonBefore="Profile Sec"
                  value={profileSec}
                  disabled={collecting}
                  onChange={(v) => v && setProfileSec(v)}
                />

                <Space style={{ textAlign: 'end' }}>
                  <Button
                    onClick={() => {
                      setCanDownload(false)
                      setCollecting(true)
                      resetCountdown()
                      startCountdown()
                    }}
                    disabled={collecting}
                    type="primary"
                  >
                    {collecting ? `Collecting...(${count})` : 'Start'}
                  </Button>
                  <Button
                    disabled={!canDownload}
                    loading={state.status === 'loading'}
                    onClick={() => {
                      actions.execute()
                    }}
                  >
                    Download
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

export default DebugModal
