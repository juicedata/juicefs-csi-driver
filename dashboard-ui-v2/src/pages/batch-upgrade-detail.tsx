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


import { useEffect, useState } from 'react'
import { Button, Space, Progress, Dropdown, MenuProps, Checkbox, Spin, InputNumber, Empty, Collapse } from 'antd'
import { FormattedMessage } from 'react-intl'
import Editor from '@monaco-editor/react'
import { useNodes, usePodsToUpgrade, useUpgradePods, useUpgradeStatus, useWebsocket } from '@/hooks/use-api.ts'
import { Pod, Node } from 'kubernetes-types/core/v1'
import { PodToUpgrade } from '@/types/k8s.ts'
import { DownOutlined } from '@ant-design/icons'
import { PageContainer, ProCard } from '@ant-design/pro-components'
import { useConfigDiff } from '@/hooks/cm-api.ts'
import { Badge } from 'antd/lib'
import { Link } from 'react-router-dom'


const BatchUpgradeDetail = () => {
  const [start, setStart] = useState(false)
  const [fail, setFail] = useState(false)
  const [data, setData] = useState<string>('')
  const [percent, setPercent] = useState(Number)
  const [selectedNode, setSelectedNode] = useState('All Nodes')
  const { data: podsToUpgrade } = usePodsToUpgrade(true, selectedNode)
  const { data: nodes } = useNodes()
  const [allNodes, setAllNodes] = useState([``])
  const { data: job } = useUpgradeStatus()
  const [, actions] = useUpgradePods()
  const [jobName, setJobName] = useState('')
  const [worker, setWorker] = useState(1)
  const [ignoreError, setIgnoreError] = useState(false)
  const { data: diffPods } = useConfigDiff(selectedNode)
  const [diffStatus, setDiffStatus] = useState<Map<string, string>>(new Map())

  useEffect(() => {
    setDiffStatus((prevStatus) => {
      const newStatus = new Map(prevStatus)
      diffPods?.forEach((pod) => {
        const podName = pod?.metadata?.name
        if (podName && !newStatus.has(podName)) {
          newStatus.set(podName, 'pending')
        }
      })
      return newStatus
    })
  }, [diffPods])


  useEffect(() => {
    setAllNodes(getAllNodes(nodes || []))
  }, [nodes])

  useEffect(() => {
    if (job && (job.metadata?.name || '') !== '') {
      setJobName(job.metadata?.name || '')
      if (job.metadata?.labels) {
        setSelectedNode(job.metadata.labels['juicefs-upgrade-node'] || '')
        // setRecreate(job.metadata.labels['juicefs-upgrade-recreate'] === 'true')
      }
      if (job.status?.active && job.status?.active !== 0) {
        setStart(true)
      }
    } else {
      setData('')
      setDiffStatus(new Map())
      setJobName('')
      setStart(false)
    }
  }, [job])

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

        if (msg.data.includes('POD-')) {
          const successRegex = /POD-SUCCESS \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]/g
          const successMatches = msg.data.matchAll(successRegex)
          for (const successMatch of successMatches) {
            if (successMatch && successMatch[1]) {
              const pod = successMatch[1]
              setDiffStatus((prevState) => {
                const newStatus = new Map(prevState)
                newStatus.set(pod, 'succeed')
                return newStatus
              })
            }
          }

          const failRegex = /POD-FAIL \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]/g
          const failMatches = msg.data.matchAll(failRegex)
          for (const failMatch of failMatches) {
            if (failMatch && failMatch[1]) {
              const pod = failMatch[1]
              setDiffStatus((prevState) => {
                const newStatus = new Map(prevState)
                newStatus.set(pod, 'fail')
                return newStatus
              })
            }
          }

          const matchRegex = new RegExp('POD-', 'g')
          const matches = msg.data.match(matchRegex)

          const totalPods = getPodsUpgradeOfNode(selectedNode, podsToUpgrade).length
          if (totalPods !== 0 && matches) {
            setPercent((prevPercent) => {
              const newPercent = prevPercent + (matches.length / totalPods) * 100
              return Math.min(Math.ceil(newPercent), 100)
            })
          }
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
    jobName !== '',
  )

  const nodeItems = allNodes?.map((item) => ({
    key: item,
    label: item,
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
        <Checkbox
          key="ignore error"
          checked={ignoreError}
          onChange={(value) => value && setIgnoreError(value.target.checked)}
        >
          <FormattedMessage id="ignoreError" />
        </Checkbox>,
        <InputNumber
          key="parallem num"
          style={{ width: '180px' }}
          min={1}
          max={50}
          defaultValue={1}
          addonBefore={<FormattedMessage id="parallelNum" />}
          keyboard={true}
          changeOnWheel
          onChange={(v) => {
            setWorker(v || 1)
          }}
        >
        </InputNumber>,
        <Button
          disabled={start}
          type="primary"
          key="start"
          onClick={() => {
            setData('')
            actions.execute({
              nodeName: selectedNode,
              recreate: true,
              worker: worker,
              ignoreError: ignoreError,
            }).then(response => {
              setJobName(response.jobName)
            })
            setStart(true)
            setPercent(0)
          }}
        >
          <FormattedMessage id="start" />
        </Button>,
      ]}
    >
      {jobName !== '' ? (
        <ProCard>
          <div style={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
            {start && <Spin style={{ marginRight: 16 }} />}
            {fail ?
              <Progress percent={percent} status="exception" format={percent => `${Math.round(percent || 0)}%`} /> :
              <Progress percent={percent} format={percent => `${Math.round(percent || 0)}%`} />
            }
          </div>
        </ProCard>
      ) : null}

      {diffPods?.length || 0 > 0 ? (
        <ProCard
          title={<FormattedMessage id="diffPods" />}
          key="diffPods"
        >
          {diffPods?.map(pod =>
            <ProCard key={pod.metadata?.uid || ''}>
              <Badge color={getUpgradeStatusBadge(diffStatus.get(pod?.metadata?.name || '') || '')}
                     text={
                       <Link to={`/syspods/${pod?.metadata?.namespace}/${pod?.metadata?.name}/`}>
                         {pod?.metadata?.name}
                       </Link>
                     }
              />
            </ProCard>,
          )}
        </ProCard>
      ) : null}

      {data !== '' ? (
        <ProCard key="upgrade log">
          <Collapse
            items={[{
              key: '1', label: <FormattedMessage id="clickToViewDetail" />, children:
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
                />,
            }]}
          />
        </ProCard>
      ) : null}

      {(data === '' && ((diffPods?.length || 0) <= 0)) ? (
        <Empty description={<FormattedMessage id="noDiff" />} />
      ) : null}
    </PageContainer>
  )
}


export default BatchUpgradeDetail

function getPodsUpgradeOfNode(node: string, podsForNode?: PodToUpgrade[]): Pod[] {
  const pods: Pod[] = []
  podsForNode?.forEach((v) => {
    if (v.node === node || node === 'All Nodes' || node === '') {
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

export const getUpgradeStatusBadge = (finalStatus: string) => {
  switch (finalStatus) {
    case 'pending':
      return 'grey'
    case 'running':
      return 'blue'
    case 'succeed':
      return 'green'
    case 'failed':
      return 'red'
    default:
      return 'grey'
  }
}
