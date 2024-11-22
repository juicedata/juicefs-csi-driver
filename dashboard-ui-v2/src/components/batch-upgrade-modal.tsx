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
import { DownOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import {
  Button,
  Checkbox,
  Dropdown,
  MenuProps,
  Modal,
  Progress,
  Space,
  Spin,
} from 'antd'
import { Node, Pod } from 'kubernetes-types/core/v1'
import { FormattedMessage } from 'react-intl'

import {
  useNodes,
  usePodsToUpgrade,
  useUpgradePods,
  useUpgradeStatus,
  useWebsocket,
} from '@/hooks/use-api.ts'
import { PodToUpgrade } from '@/types/k8s.ts'

const helpMessage = `Click Start to perform a batch upgrade.

- node: Select a node to upgrade all Mount Pods on it.
- recreate: Upgrade a Mount Pod, with or without recreating it.

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
  const { data: podsToUpgrade } = usePodsToUpgrade(
    isBatchModalOpen,
    true,
    selectedNode,
  )
  const { data: nodes } = useNodes(isBatchModalOpen)
  const [allNodes, setAllNodes] = useState([``])
  const { data: job } = useUpgradeStatus(isBatchModalOpen)
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
    setAllNodes(getAllNodes(nodes || []))
  }, [nodes])

  useEffect(() => {
    if (job && (job.metadata?.name || '') !== '') {
      setJobName(job.metadata?.name || '')
      if (job.metadata?.labels) {
        setSelectedNode(job.metadata.labels['juicefs-upgrade-node'] || '')
        setRecreate(job.metadata.labels['juicefs-upgrade-recreate'] === 'true')
      }
      if (job.status?.active && job.status?.active !== 0) {
        setStart(true)
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

        const totalPods = getPodsUpgradeOfNode(
          selectedNode,
          podsToUpgrade,
        ).length
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
    label: item,
  }))

  const handleNodeSelected: MenuProps['onClick'] = (e) => {
    const selectedItem = nodeItems?.find((item) => item.key === e.key)
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
              <Space style={{ textAlign: 'end' }}>
                <Dropdown menu={menuProps}>
                  <Button>
                    <Space>
                      {selectedNode || 'All Nodes'}
                      <DownOutlined />
                    </Space>
                  </Button>
                </Dropdown>

                <Checkbox
                  checked={recreate}
                  onChange={(value) =>
                    value && setRecreate(value.target.checked)
                  }
                >
                  <FormattedMessage id="recreate" />
                </Checkbox>

                <Button
                  disabled={start}
                  type="primary"
                  onClick={() => {
                    setData(helpMessage)
                    actions
                      .execute({
                        nodeName: selectedNode,
                        recreate: recreate,
                      })
                      .then((response) => {
                        setJobName(response.jobName)
                      })
                    setStart(true)
                    setPercent(0)
                  }}
                >
                  <FormattedMessage id="start" />
                </Button>
              </Space>
            </div>
          )}
          onOk={handleOk}
          onCancel={handleCancel}
        >
          {jobName !== '' ? (
            <div
              style={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}
            >
              {start && <Spin style={{ marginRight: 16 }} />}
              {fail ? (
                <Progress
                  percent={percent}
                  status="exception"
                  format={(percent) => `${Math.round(percent || 0)}%`}
                />
              ) : (
                <Progress
                  percent={percent}
                  format={(percent) => `${Math.round(percent || 0)}%`}
                />
              )}
            </div>
          ) : null}
          <Editor
            height="calc(100% - 20px)"
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

function getPodsUpgradeOfNode(
  node: string,
  podsForNode?: PodToUpgrade[],
): Pod[] {
  const pods: Pod[] = []
  podsForNode?.forEach((v) => {
    if (v.node === node || node === 'All Nodes') {
      pods.push(...v.pods)
    }
  })
  return pods
}

function getAllNodes(nodes: Node[]): string[] {
  const allNodes = ['All Nodes']
  nodes?.forEach((v) => {
    if (v.metadata?.name) {
      allNodes.push(v.metadata?.name)
    }
  })
  return allNodes
}
