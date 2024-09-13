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

import { useMountPodImage, useWebsocket } from '@/hooks/use-api'

const upgradeHelpMessage = `Click Start to upgrade mount pod

---
`

const binaryHelpMessage = `Click Start to upgrade binary in mount pod

---
`

const UpgradeModal: React.FC<{
  namespace: string
  name: string
  recreate: boolean
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ namespace, name,recreate, children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const { data: newImage } = useMountPodImage(namespace, name)
  const [data, setData] = useState<string>('')
  const [start, setStart] = useState(false)

  useWebsocket(
    `/api/v1/ws/pod/${namespace}/${name}/upgrade`,
    {
      queryParams: {
        recreate: recreate ? 'true' : 'false',
      },
      onClose: () => {
        setStart(false)
      },
      onMessage: (msg) => {
        setData((prev) => prev + msg.data)
      },
    },
    isModalOpen && start,
  )

  useEffect(() => {
    if (isModalOpen) {
      if (recreate){
        setData(`Smoothly upgrade mount pod to ${newImage}\n\n` + upgradeHelpMessage)
      } else {
        setData(`Smoothly upgrade mount pod to ${newImage}\n\n` + binaryHelpMessage)
      }
    }
  }, [recreate, isModalOpen, newImage])

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

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={`Upgrade: ${namespace}/${name}`}
          open={isModalOpen}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space>
                <Space style={{ textAlign: 'end' }}>
                  <Button
                    onClick={() => {
                      setStart(true)
                    }}
                    disabled={start}
                    type="primary"
                  >
                    Upgrade
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

export default UpgradeModal
