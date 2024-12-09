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

import { useCallback, useEffect, useState } from 'react'
import { DownOutlined } from '@ant-design/icons'
import { PageContainer, ProCard } from '@ant-design/pro-components'
import Editor from '@monaco-editor/react'
import {
  AutoComplete,
  Button,
  Checkbox,
  Collapse,
  Dropdown,
  Empty,
  InputNumber,
  MenuProps,
  Progress,
  Space,
  Spin,
} from 'antd'
import { FormattedMessage } from 'react-intl'

import PodUpgradeTable from '@/components/pod-upgrade-table.tsx'
import { useConfigDiff } from '@/hooks/cm-api.ts'
import { usePVCs, usePVCWithUniqueId } from '@/hooks/pv-api.ts'
import {
  useBatchPlan,
  useClearUpgradeStatus,
  useNodes,
  useUpgradePods,
  useUpgradeStatus,
  useWebsocket,
} from '@/hooks/use-api.ts'
import { PVC } from '@/types/k8s.ts'

const BatchUpgradeDetail = () => {
  const [jobStatus, setJobStatus] = useState<string>('diff')
  const [data, setData] = useState<string>('')
  const [percent, setPercent] = useState(Number)

  const [selectedPVCName, setSelectedPVCName] = useState('')
  const { data: selectedPVC } = usePVCWithUniqueId(selectedPVCName)
  const [uniqueId, setUniqueId] = useState('')
  const { data: pvcs } = usePVCs({ name: '' })

  const [selectedNode, setSelectedNode] = useState('All Nodes')
  const { data: nodes } = useNodes()
  const [allNodes, setAllNodes] = useState([``])

  const { data: diffPods } = useConfigDiff(selectedNode, uniqueId)

  const { data: job, mutate: jobMutate } = useUpgradeStatus()
  const [, actions] = useUpgradePods()
  const [, clearActions] = useClearUpgradeStatus()
  const [jobName, setJobName] = useState('')

  const [worker, setWorker] = useState(1)
  const [ignoreError, setIgnoreError] = useState(false)
  const { data: batchConfig, mutate: planMutate } = useBatchPlan(
    selectedNode,
    uniqueId,
    worker,
    ignoreError,
    true,
  )
  const [total, setTotal] = useState(0)
  const [diffStatus, setDiffStatus] = useState<Map<string, string>>(new Map())

  const resetState = useCallback(() => {
    setData('')
    setDiffStatus(new Map())
    setJobStatus('diff')
    planMutate()
  }, [planMutate])

  useEffect(() => {
    if (jobStatus === 'start') {
      jobMutate()
    }
  }, [jobMutate, jobStatus])

  useEffect(() => {
    setUniqueId(selectedPVC?.UniqueId || '')
  }, [selectedPVC])

  useEffect(() => {
    setDiffStatus((prevStatus) => {
      const newStatus = new Map(prevStatus)
      batchConfig?.batches?.forEach((podUpgrades) => {
        podUpgrades?.forEach((podUpgrade) => {
          newStatus.set(podUpgrade.name, 'pending')
        })
      })
      return newStatus
    })
  }, [batchConfig])

  useEffect(() => {
    setAllNodes([
      'All Nodes',
      ...(nodes?.map((node) => node.metadata?.name || '') || []),
    ])
  }, [nodes])

  useEffect(() => {
    let totalPods = 0
    batchConfig?.batches?.forEach((podUpgrades) => {
      totalPods += podUpgrades?.length || 0
    })
    setTotal(totalPods)
    setJobName(job?.metadata?.name || '')
    if (!job || !job.metadata?.name) {
      resetState()
      if (totalPods <= 0) {
        setJobStatus('nodiff')
      }
      if (totalPods > 0) {
        setJobStatus('diff')
      }
    } else {
      setSelectedNode(batchConfig?.node || '')
      setUniqueId(batchConfig?.uniqueId || '')

      if ((job.status?.failed || 0) > 0) {
        setJobStatus('batch-fail')
      } else {
        setJobStatus((prevState) => {
          if ((job.status?.succeeded || 0) > 0) {
            return prevState
          }
          return prevState !== 'fail' &&
            prevState !== 'batch-fail' &&
            prevState !== 'success'
            ? 'start'
            : prevState
        })
      }
    }
  }, [job, batchConfig, resetState])

  const handleWebSocketMessage = (msg: MessageEvent) => {
    setData((prev) => prev + msg.data)
    if (msg.data.includes('POD-')) {
      updatePodStatus(msg.data)
    }
    if (msg.data.includes('FAIL')) {
      setJobStatus('fail')
    }
    if (msg.data.includes('BATCH-SUCCESS')) {
      setJobStatus('success')
    }
    if (msg.data.includes('BATCH-FAIL')) {
      setJobStatus('batch-fail')
    }
    if (msg.data.includes('BATCH-')) {
      return
    }
  }

  const updatePodStatus = (message: string) => {
    const updateStatus = (regex: RegExp, status: string) => {
      for (const match of message.matchAll(regex)) {
        const podName = match[1]
        setDiffStatus((prev) => new Map(prev).set(podName, status))
      }
    }

    updateStatus(
      /POD-START \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]/g,
      'running',
    )
    updateStatus(
      /POD-SUCCESS \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]/g,
      'success',
    )
    updateStatus(
      /POD-FAIL \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]/g,
      'fail',
    )

    const successMatches = message.match(/POD-SUCCESS/g) || []
    setPercent((prev) =>
      Math.min(Math.ceil(prev + (successMatches.length / total) * 100), 100),
    )
  }

  useWebsocket(
    `/api/v1/ws/batch/upgrade/logs`,
    {
      queryParams: { job: jobName },
      onClose: () => jobMutate(),
      onMessage: handleWebSocketMessage,
    },
    jobName !== '',
  )

  const nodeItems = allNodes.map((item) => ({ key: item, label: item }))

  const handleNodeSelected: MenuProps['onClick'] = (e) => {
    const selectedItem = nodeItems.find((item) => item.key === e.key)
    setSelectedNode(selectedItem?.label || '')
  }

  const menuProps = {
    items: nodeItems,
    onClick: handleNodeSelected,
  }

  const handleStartClick = () => {
    setData('')
    actions.execute(batchConfig).then((response) => {
      setJobName(response.jobName)
    })
    setJobStatus('start')
    setPercent(0)
    setDiffStatus((prevStatus) => {
      const newStatus = new Map(prevStatus)
      batchConfig?.batches?.forEach((podUpgrades) => {
        podUpgrades?.forEach((podUpgrade) => {
          newStatus.set(podUpgrade.name, 'pending')
        })
      })
      return newStatus
    })
  }

  return (
    <PageContainer
      fixedHeader
      header={{
        title: <FormattedMessage id="upgrade" />,
        ghost: true,
      }}
      extra={[
        <Dropdown key="select node" menu={menuProps}>
          <Button>
            <Space>
              {selectedNode || 'All Nodes'}
              <DownOutlined />
            </Space>
          </Button>
        </Dropdown>,
        <AutoComplete
          key="select pvc"
          style={{ width: 200 }}
          options={getAllPVCs(pvcs?.pvcs || [])}
          filterOption={(inputValue, option) =>
            option!.value.toUpperCase().indexOf(inputValue.toUpperCase()) !== -1
          }
          placeholder={<FormattedMessage id="selectPVC" />}
          onSelect={setSelectedPVCName}
        />,
        <Checkbox
          key="ignore error"
          checked={ignoreError}
          onChange={(e) => setIgnoreError(e.target.checked)}
        >
          <FormattedMessage id="ignoreError" />
        </Checkbox>,
        <InputNumber
          key="parallel num"
          style={{ width: '180px' }}
          min={1}
          max={50}
          defaultValue={1}
          addonBefore={<FormattedMessage id="parallelNum" />}
          onChange={(v) => {
            setWorker(v || 1)
          }}
        ></InputNumber>,
        isComplete(jobStatus) ? (
          <Button
            type="primary"
            key="complete"
            onClick={() => {
              clearActions.execute().then(() => {
                jobMutate()
              })
              resetState()
            }}
          >
            <FormattedMessage id="complete" />
          </Button>
        ) : (
          <Button
            disabled={!canStart(jobStatus)}
            type="primary"
            key="start"
            onClick={handleStartClick}
          >
            <FormattedMessage id="start" />
          </Button>
        ),
      ]}
    >
      {jobStatus !== 'diff' && jobStatus !== 'nodiff' && (
        <ProCard>
          <div style={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
            {!isComplete(jobStatus) && <Spin style={{ marginRight: 16 }} />}
            <Progress
              percent={total > 0 ? percent : 100}
              status={jobStatus.includes('fail') ? 'exception' : undefined}
              format={(percent) => `${Math.round(percent || 0)}%`}
            />
          </div>
        </ProCard>
      )}

      {total !== 0 && (
        <PodUpgradeTable
          diffPods={diffPods}
          batchConfig={batchConfig}
          diffStatus={diffStatus}
        />
      )}

      {data && (
        <ProCard key="upgrade log">
          <Collapse
            items={[
              {
                key: '1',
                label: <FormattedMessage id="clickToViewDetail" />,
                children: (
                  <Editor
                    height="calc(100vh - 200px)"
                    language="shell"
                    options={{
                      wordWrap: 'on',
                      readOnly: true,
                      theme: 'vs-light', // TODO dark mode
                      scrollBeyondLastLine: false,
                    }}
                    value={data}
                  />
                ),
              },
            ]}
          />
        </ProCard>
      )}

      {data === '' && total === 0 && (
        <Empty description={<FormattedMessage id="noDiff" />} />
      )}
    </PageContainer>
  )
}

export default BatchUpgradeDetail

function getAllPVCs(pvcs: PVC[]) {
  return pvcs.map((v) => ({
    key: v.metadata?.uid,
    value: `${v.metadata?.namespace}/${v.metadata?.name}`,
    pvc: v,
  }))
}

const isComplete = (jobStatus: string): boolean => {
  return jobStatus === 'batch-fail' || jobStatus === 'success'
}

const canStart = (jobStatus: string): boolean => {
  return (
    jobStatus === 'batch-fail' ||
    jobStatus === 'success' ||
    jobStatus === 'diff'
  )
}
