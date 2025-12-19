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

import React, { useEffect, useState } from 'react'
import { DownOutlined } from '@ant-design/icons'
import { ProCard } from '@ant-design/pro-components'
import {
  AutoComplete,
  Button,
  Checkbox,
  Dropdown,
  Input,
  InputNumber,
  MenuProps,
  Modal,
  Space,
} from 'antd'
import { FormattedMessage } from 'react-intl'
import { useNavigate } from 'react-router-dom'

import PodToUpgradeTable from '@/components/pod-to-upgrade-table.tsx'
import { useCreateUpgradeJob } from '@/hooks/job-api.ts'
import { usePVCsBasicInfo, usePVCsWithUniqueId } from '@/hooks/pv-api.ts'
import { useNodes } from '@/hooks/use-api.ts'
import { PodDiffConfig, PVCBasicInfo } from '@/types/k8s.ts'

const BatchUpgradeModal: React.FC<{
  modalOpen: boolean
  onOk: () => void
  onCancel: () => void
}> = (props) => {
  const { modalOpen, onOk, onCancel } = props

  const [selectedPVCName, setSelectedPVCName] = useState('')
  const { data: selectedPVC } = usePVCsWithUniqueId(selectedPVCName)
  const [uniqueId, setUniqueId] = useState('')
  const { data: pvcs } = usePVCsBasicInfo()

  const [selectedNode, setSelectedNode] = useState('All Nodes')
  const { data: nodes } = useNodes()
  const [allNodes, setAllNodes] = useState([``])

  const [worker, setWorker] = useState(1)
  const [ignoreError, setIgnoreError] = useState(false)
  const [newJobName, setNewJobName] = useState(genNewJobName())
  const [diffPods, setDiffPods] = useState<PodDiffConfig[]>([])

  const [, actions] = useCreateUpgradeJob()
  const navigate = useNavigate()

  const resetState = () => {
    setWorker(1)
    setSelectedNode('All Nodes')
    setSelectedPVCName('')
  }

  const handleStartClick = () => {
    resetState()
    actions
      .execute(worker, ignoreError, newJobName, selectedNode, uniqueId)
      .then((response) => {
        onOk()
        navigate(`/jobs/${response.jobName}`)
      })
  }
  const handleCancel = () => {
    resetState()
    onCancel()
  }

  const nodeItems = allNodes.map((item) => ({ key: item, label: item }))

  const handleNodeSelected: MenuProps['onClick'] = (e) => {
    const selectedItem = nodeItems.find((item) => item.key === e.key)
    setSelectedNode(selectedItem?.label || '')
  }

  const menuProps = {
    items: nodeItems,
    onClick: handleNodeSelected,
  }

  useEffect(() => {
    setUniqueId(selectedPVC?.UniqueId || '')
  }, [selectedPVC])

  useEffect(() => {
    setAllNodes([
      'All Nodes',
      ...(nodes?.map((node) => node.metadata?.name || '') || []),
    ])
  }, [nodes])

  return (
    <>
      <Modal
        title={<FormattedMessage id="createUpgradeJob" />}
        onOk={handleStartClick}
        onCancel={handleCancel}
        className="batch-upgrade-modal"
        open={modalOpen}
        footer={() => (
          <Button
            disabled={!diffPods.length}
            type="primary"
            key="start"
            onClick={handleStartClick}
          >
            <FormattedMessage id="start" />
          </Button>
        )}
      >
        <ProCard>
          <Space size="large" style={{ width: '100%' }}>
            <Input
              addonBefore={<FormattedMessage id="jobName" />}
              defaultValue={newJobName}
              onChange={(v) => {
                setNewJobName(v.target.value)
              }}
            />
            <Dropdown key="select node" menu={menuProps}>
              <Button>
                <Space>
                  {selectedNode || 'All Nodes'}
                  <DownOutlined />
                </Space>
              </Button>
            </Dropdown>
            <AutoComplete
              key="select pvc"
              style={{ width: 200 }}
              options={getAllPVCs(pvcs?.pvcs || [])}
              filterOption={(inputValue, option) =>
                option!.value
                  .toUpperCase()
                  .indexOf(inputValue.toUpperCase()) !== -1
              }
              placeholder={<FormattedMessage id="selectPVC" />}
              onSelect={setSelectedPVCName}
            />
            <Checkbox
              key="ignore error"
              checked={ignoreError}
              onChange={(e) => setIgnoreError(e.target.checked)}
            >
              <FormattedMessage id="ignoreError" />
            </Checkbox>
            <InputNumber
              key="parallel num"
              style={{ width: '180px' }}
              min={1}
              max={50}
              value={worker}
              addonBefore={<FormattedMessage id="parallelNum" />}
              onChange={(v) => {
                setWorker(v || 1)
              }}
            ></InputNumber>
          </Space>

          <PodToUpgradeTable
            nodeName={selectedNode}
            uniqueId={uniqueId}
            setDiffPods={setDiffPods}
          />
        </ProCard>
      </Modal>
    </>
  )
}

export default BatchUpgradeModal

function getAllPVCs(pvcs: PVCBasicInfo[]) {
  return pvcs.map((v) => ({
    key: v.uid,
    value: `${v.namespace}/${v.name}`,
  }))
}

const genNewJobName = () => {
  function generateRandomString(length: number): string {
    const characters = 'abcdefghijklmnopqrstuvwxyz'
    return Array.from({ length }, () =>
      characters.charAt(Math.floor(Math.random() * characters.length)),
    ).join('')
  }

  const randomString = generateRandomString(6)
  return 'jfs-upgrade-job-' + randomString
}
