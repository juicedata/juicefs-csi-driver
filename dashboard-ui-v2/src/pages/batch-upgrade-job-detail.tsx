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

import React, { memo, useEffect, useState } from 'react'
import { PageContainer, ProCard } from '@ant-design/pro-components'
import Editor from '@monaco-editor/react'
import { Collapse, Empty, Progress, Spin } from 'antd'
import { FormattedMessage } from 'react-intl'

import PodUpgradeTable from '@/components/pod-upgrade-table.tsx'
import UpgradeBasic from '@/components/upgrade-basic.tsx'
import { useUpgradeJob } from '@/hooks/job-api.ts'
import { useWebsocket } from '@/hooks/use-api.ts'
import { formatTime, timeToBeDeletedOfJob } from '@/utils'

const BatchUpgradeJobDetail: React.FC<{
  jobName?: string
  namespace?: string
}> = memo((props) => {
  const { jobName, namespace } = props
  const [jobStatus, setJobStatus] = useState<string>('running')
  const [data, setData] = useState<string>('')
  const [percent, setPercent] = useState(0)

  const {
    data: upgradeJob,
    isLoading,
    mutate: jobMutate,
  } = useUpgradeJob(jobName || '')

  const [total, setTotal] = useState(0)
  const [diffStatus, setDiffStatus] = useState<Map<string, string>>(new Map())
  const [failReasons, setFailReasons] = useState<Map<string, string>>(new Map())
  const [deletedTime, setDeleteTime] = useState<string>()

  useEffect(() => {
    let totalPods = 0
    upgradeJob?.config?.batches?.forEach((podUpgrades) => {
      totalPods += podUpgrades?.length || 0
    })
    setTotal(totalPods)
    setJobStatus(upgradeJob?.config?.status || 'running')
    setDeleteTime(formatTime(timeToBeDeletedOfJob(upgradeJob?.job)))
  }, [upgradeJob])

  const handleWebSocketMessage = (msg: MessageEvent) => {
    setData((prev) => prev + msg.data)
    if (msg.data.includes('POD-')) {
      updatePodStatus(msg.data)
    }
    if (msg.data.includes('POD-FAIL')) {
      failReason(msg.data,
        /POD-FAIL \[([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*)\]/g,
      )
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
    setPercent((prev) => {
      if (total != 0) {
        return Math.min(
          Math.ceil(prev + (successMatches.length / total) * 100),
          100,
        )
      }
      return 0
    })
  }

  const failReason = (message: string, regex: RegExp) => {
    const pods: string[] = []
    for (const match of message.matchAll(regex)) {
      const podName = match[1]
      pods.push(podName)
    }

    pods.forEach((pod) => {
      const podReg = new RegExp(String.raw`POD-FAIL \[${pod}\] (.+).`, 'g')
      for (const match of message.matchAll(podReg)) {
        const reason = match[1]
        setFailReasons((prev) => new Map(prev).set(pod, reason))
      }
    })
  }

  useWebsocket(
    `/api/v1/ws/batch/upgrade/jobs/${jobName}/logs`,
    {
      onClose: () => jobMutate(),
      onMessage: handleWebSocketMessage,
    },
    upgradeJob !== undefined,
  )

  if (
    jobName === '' ||
    namespace === '' ||
    !upgradeJob ||
    !upgradeJob.job ||
    upgradeJob.job.metadata?.name === ''
  ) {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="jobNotFound" />,
        }}
      ></PageContainer>
    )
  }

  return (
    <PageContainer
      fixedHeader
      loading={isLoading}
      header={{
        title: jobName,
        subTitle: deletedTime === '' ? '' : `deleted after ${deletedTime}`,
        ghost: true,
      }}
    >
      <UpgradeBasic
        upgradeJob={upgradeJob}
        freshJob={() => {
          jobMutate()
        }}
      />

      <ProCard
        title={<FormattedMessage id="upgradeDetail" />}
        key="upgradeDetail"
        style={{ marginBlockStart: 4 }}
        gutter={4}
        wrap
      >
        <ProCard>
          <div style={{ display: 'flex', alignItems: 'center', flexShrink: 0 }}>
            {isRunning(jobStatus) && <Spin style={{ marginRight: 16 }} />}
            <Progress
              percent={total > 0 ? percent : 100}
              status={jobStatus.includes('fail') ? 'exception' : undefined}
              format={(percent) => `${Math.round(percent || 0)}%`}
            />
          </div>
        </ProCard>

        {total !== 0 && (
          <PodUpgradeTable
            diffPods={upgradeJob?.diffs}
            batchConfig={upgradeJob?.config}
            diffStatus={diffStatus}
            failReasons={failReasons}
          />
        )}
      </ProCard>

      {data && (
        <ProCard key="upgrade log">
          <Collapse
            defaultActiveKey="1"
            items={[
              {
                key: '1',
                label: <FormattedMessage id="upgradeLog" />,
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
})

export default BatchUpgradeJobDetail

const isRunning = (jobStatus: string): boolean => {
  return jobStatus === 'running'
}
