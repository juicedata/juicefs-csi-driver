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


import { memo, ReactNode, useState } from 'react'
import { Button, Modal, Space, Progress, Flex, Input } from 'antd'
import { FormattedMessage } from 'react-intl'
import Editor from '@monaco-editor/react'
import { usePodsToUpgrade, useWebsocket } from '@/hooks/use-api.ts'
import { Pod } from 'kubernetes-types/core/v1'
import { PodToUpgrade } from '@/types/k8s.ts'

const BatchUpgradeModal: React.FC<{
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [start, setStart] = useState(false)
  const [progressShow, setProgressShow] = useState(false)
  const [fail, setFail] = useState(false)
  const [data, setData] = useState<string>('')
  const [node, setNode] = useState('')
  const [percent, setPercent] = useState(Number)
  const { data: podsToUpgrade } = usePodsToUpgrade(true, node)

  const showModal = () => {
    setIsModalOpen(true)
  }
  const handleOk = () => {
    setIsModalOpen(false)
  }
  const handleCancel = () => {
    setIsModalOpen(false)
    setData('')
    setProgressShow(false)
  }

  useWebsocket(
    `/api/v1/ws/upgrade-pods`,
    {
      queryParams: {
        recreate: 'true',
        nodeName: node,
      },
      onClose: () => {
        setStart(false)
      },
      onMessage: async (msg) => {
        setData((prev) => prev + msg.data)
        if (msg.data.includes('POD-SUCCESS')) {
          console.log('percent: ', percent)
          console.log('pods: ', podsToUpgrade)
          console.log('node: ', node)
          if (getPodsUpgradeOfNode(node, podsToUpgrade).length !== 0) {
            setPercent(percent + 1 / getPodsUpgradeOfNode(node, podsToUpgrade).length * 100)
            console.log('set percent: ', percent)
          }
        }
        if (msg.data.includes('BATCH-FAIL')) {
          setFail(true)
        }
      },
    },
    isModalOpen && start,
  )

  return (
    <>
      {children({ onClick: showModal })}
      {isModalOpen ? (
        <Modal
          title={<FormattedMessage id={start ? 'upgrading' : 'batchUpgrade'} />}
          open={isModalOpen}
          onOk={handleOk}
          onCancel={handleCancel}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space>
                <Input
                  addonBefore="node"
                  value={node}
                  onChange={(v) => v && setNode(v.target.value)}
                />

                <Space style={{ textAlign: 'end' }}>
                  <Button
                    onClick={() => {
                      setStart(true)
                      setProgressShow(true)
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
        >
          {progressShow ? (
            <Flex vertical gap="">
              <Progress percent={percent} status={fail ? 'exception' : 'active'} />
              <div style={{ height: '20px' }}></div>
            </Flex>
          ) : null}
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

export default BatchUpgradeModal

function getPodsUpgradeOfNode(node: string, podsForNode?: PodToUpgrade[]): Pod[] {
  let pods: Pod[] = []
  if (podsForNode) {
    podsForNode.forEach((v) => {
      if (v.node === node) {
        pods = v.pods
      }
    })
  }
  return pods
}