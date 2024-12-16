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
  InputNumber,
  MenuProps,
  Modal,
  Space,
} from 'antd'
import { FormattedMessage } from 'react-intl'

import PodToUpgradeTable from '@/components/pod-to-upgrade-table.tsx'
import { useConfigDiff } from '@/hooks/cm-api.ts'
import { useBatchPlan, useCreateUpgradeJob } from '@/hooks/job-api.ts'
import { usePVCs, usePVCsWithUniqueId } from '@/hooks/pv-api.ts'
import { useNodes } from '@/hooks/use-api.ts'
import { PVC } from '@/types/k8s.ts'
import { useNavigate } from 'react-router-dom'

const BatchUpgradeModal: React.FC<{
  modalOpen: boolean
  onOk: () => void
  onCancel: () => void
}> = (props) => {
  const { modalOpen, onOk, onCancel } = props

  const [selectedPVCName, setSelectedPVCName] = useState('')
  const { data: selectedPVC } = usePVCsWithUniqueId(selectedPVCName)
  const [uniqueId, setUniqueId] = useState('')
  const { data: pvcs } = usePVCs({ name: '' })

  const [selectedNode, setSelectedNode] = useState('All Nodes')
  const { data: nodes } = useNodes()
  const [allNodes, setAllNodes] = useState([``])

  const [worker, setWorker] = useState(1)
  const [ignoreError, setIgnoreError] = useState(false)

  const { data: diffPods } = useConfigDiff(selectedNode, uniqueId)
  const { data: batchConfig } = useBatchPlan(
    selectedNode,
    uniqueId,
    worker,
    ignoreError,
    true,
  )
  const [, actions] = useCreateUpgradeJob()
  const navigate = useNavigate()

  const resetState = () => {
    setWorker(1)
    setSelectedNode('All Nodes')
    setSelectedPVCName('')
  }

  const handleStartClick = () => {
    resetState()
    actions.execute(batchConfig).then((response) => {
        onOk()
        navigate(`/jobs/${response.jobName}`)
      },
    )
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
          <Button type="primary" key="start" onClick={handleStartClick}>
            <FormattedMessage id="start" />
          </Button>
        )}
      >
        <ProCard>
          <Space size="large" style={{ width: '100%' }}>
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

          <PodToUpgradeTable diffPods={diffPods} batchConfig={batchConfig} />
        </ProCard>
      </Modal>
    </>
  )
}

export default BatchUpgradeModal

function getAllPVCs(pvcs: PVC[]) {
  return pvcs.map((v) => ({
    key: v.metadata?.uid,
    value: `${v.metadata?.namespace}/${v.metadata?.name}`,
    pvc: v,
  }))
}
