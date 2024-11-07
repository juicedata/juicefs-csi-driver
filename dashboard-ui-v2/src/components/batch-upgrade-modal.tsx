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


import React, { memo, ReactNode, useEffect, useState } from 'react'
import { Button, Modal, Space, Progress, Dropdown, MenuProps, Checkbox, Spin } from 'antd'
import { FormattedMessage } from 'react-intl'
import Editor from '@monaco-editor/react'
import { useNodes, usePodsToUpgrade, useUpgradePods, useUpgradeStatus, useWebsocket } from '@/hooks/use-api.ts'
import { Pod } from 'kubernetes-types/core/v1'
import { PodToUpgrade } from '@/types/k8s.ts'
import { DownOutlined } from '@ant-design/icons'


const helpMessage = `Click Start to upgrade mount pod by batch.

- node: select a node to upgrade all Mount Pods on it, not select means all nodes. 
- recreate: upgrade Mount Pod with recreating it or not.

---
`

const BatchUpgradeModal: React.FC<{
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = memo(({ children }) => {
  const [isBatchModalOpen, setIsBatchModelOpen] = useState(false)
  const [start, setStart] = useState(false)
  const [fail, setFail] = useState(false)
  const [data, setData] = useState<string>(helpMessage)
  const [percent, setPercent] = useState(Number)
  const [selectedNode, setSelectedNode] = useState('')
  const { data: podsToUpgrade } = usePodsToUpgrade(true, selectedNode)
  const { data: allNodes } = useNodes()
  const { data: job } = useUpgradeStatus()
  const [, actions] = useUpgradePods()
  const [jobName, setJobName] = useState('')
  const [recreate, setRecreate] = useState(false)

  const showModal = () => {
    setIsBatchModelOpen(true)
  }
  const handleOk = () => {
    setIsBatchModelOpen(false)
  }
  const handleCancel = () => {
    setIsBatchModelOpen(false)
    setData(helpMessage)
    setJobName('')
    setPercent(0)
    setFail(false)
  }

  useEffect(() => {
    if (job && (job.metadata?.name || '') !== '') {
      setJobName(job.metadata?.name || '')
      setStart(true)
      if (job.metadata?.labels) {
        setSelectedNode(job.metadata.labels['juicefs-upgrade-node'] || '')
        setRecreate(job.metadata.labels['juicefs-upgrade-recreate'] === 'true')
      }
    } else {
      setData(helpMessage)
      setJobName('')
      setStart(false)
    }
  }, [job, isBatchModalOpen])

  useWebsocket(
    `/api/v1/ws/batch/upgrade/logs`,
    {
      queryParams: {
        job: jobName,
      },
      onClose: () => {
        setStart(false)
      },
      onMessage: async (msg) => {
        setData((prev) => prev + msg.data)

        const matchRegex = new RegExp('POD-', 'g')
        const matches = msg.data.match(matchRegex)

        const totalPods = getPodsUpgradeOfNode(selectedNode, podsToUpgrade).length
        if (totalPods !== 0 && matches) {
          setPercent((prevPercent) => {
            const newPercent = prevPercent + (matches.length / totalPods) * 100
            return Math.min(Math.ceil(newPercent), 100)
          })
        }

        if (msg.data.includes('FAIL')) {
          setFail(true)
          close()
        }
        if (msg.data.includes('BATCH-')) {
          return
        }
      },
    },
    isBatchModalOpen && start && jobName !== '',
  )

  const nodeItems = allNodes?.map((item, index) => ({
    key: index.toString(),
    label: item.metadata?.name,
  }))

  const handleNodeSelected: MenuProps['onClick'] = (e) => {
    const selectedItem = nodeItems?.find(item => item.key === e.key)
    if (selectedItem) {
      setSelectedNode(selectedItem.label || '')
    }
  }

  const menuProps = {
    items: nodeItems,
    onClick: handleNodeSelected,
  }

  return (
    <>
      {children({ onClick: showModal })}
      {isBatchModalOpen ? (
        <Modal
          title={<FormattedMessage id={start ? 'upgrading' : 'batchUpgrade'} />}
          open={isBatchModalOpen}
          footer={() => (
            <div style={{ textAlign: 'start' }}>
              <Space>
                <Space style={{ textAlign: 'end' }}>
                  <Dropdown menu={menuProps}>
                    <Button>
                      <Space>
                        {selectedNode || 'select a node'}
                        <DownOutlined />
                      </Space>
                    </Button>
                  </Dropdown>

                  <Checkbox
                    checked={recreate}
                    onChange={(value) => value && setRecreate(value.target.checked)}
                  >
                    <FormattedMessage id="recreate" />
                  </Checkbox>

                  <Button
                    disabled={start}
                    type="primary"
                    onClick={() => {
                      setData(helpMessage)
                      actions.execute({
                        nodeName: selectedNode,
                        recreate: recreate,
                      }).then(response => {
                        setJobName(response.jobName)
                      })
                      setStart(true)
                      setPercent(0)
                    }}
                  >
                    <FormattedMessage id="batchUpgrade" />
                  </Button>
                </Space>
              </Space>
            </div>
          )}
          onOk={handleOk}
          onCancel={handleCancel}
        >

          <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
            {jobName !== '' ? (
              <div style={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
                {start && <Spin style={{ marginRight: 16 }} />}
                {fail ?
                  <Progress percent={percent} status="exception" format={percent => `${Math.round(percent || 0)}%`} /> :
                  <Progress percent={percent} format={percent => `${Math.round(percent || 0)}%`} />
                }
                <div style={{ height: '20px' }}></div>
              </div>
            ) : null}
            <div style={{ flexGrow: 1 }}>
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
            </div>
          </div>

        </Modal>
      ) : null}
    </>
  )
})

export default BatchUpgradeModal

function getPodsUpgradeOfNode(node: string, podsForNode?: PodToUpgrade[]): Pod[] {
  const pods: Pod[] = []
  if (podsForNode) {
    podsForNode.forEach((v) => {
      if (v.node === node || node === '') {
        pods.push(...v.pods)
      }
    })
  }
  return pods
}
