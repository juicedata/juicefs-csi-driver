/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import { getNodeStatusBadge } from '@/pages/utils'
import { PodStatusEnum } from '@/services/common'
import { Pod, getLog, getPod, podStatus } from '@/services/pod'
import { DownloadOutlined, SyncOutlined } from '@ant-design/icons'
import {
  PageContainer,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components'
import { useLocation, useParams, useSearchParams } from '@umijs/max'
import { Button, Empty, Space, Table, Tag, Typography } from 'antd'
import { Badge } from 'antd/lib'
import { Event, Pod as RawPod } from 'kubernetes-types/core/v1'
import React, { useEffect, useState } from 'react'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { FormattedMessage, Link } from 'umi'
import { formatData } from '../utils'

type LogToolProps = {
  pod: Pod
  setPod: (pod: Pod) => void
  container: string
  data: Blob
}

const podLink = (pod: Pod, podType?: string) => {
  let base = 'pod'
  if (podType === 'app') {
    base = 'mountpod'
  } else if (podType === 'mount') {
    base = 'apppod'
  }
  return (
    <Link to={`/${base}/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
      {pod.metadata?.name}
    </Link>
  )
}

export const getPodTableContent = (
  pods: RawPod[],
  title: string,
  podType?: string,
) => {
  if (!title) {
    return
  }
  return (
    <ProCard title={title}>
      <Table
        columns={[
          {
            title: <FormattedMessage id="name" />,
            key: 'name',
            render: (pod) => podLink(pod, podType),
          },
          {
            title: <FormattedMessage id="namespace" />,
            key: 'namespace',
            dataIndex: ['metadata', 'namespace'],
          },
          {
            title: <FormattedMessage id="status" />,
            key: 'status',
            render: (pod) => {
              const finalStatus = podStatus(pod)
              let color = 'grey'
              let text = finalStatus
              switch (finalStatus) {
                case 'Pending':
                case 'ContainerCreating':
                case 'PodInitializing':
                  color = 'yellow'
                  break
                case 'Running':
                  color = 'green'
                  break
                case 'Succeed':
                  color = 'blue'
                  break
                case 'Failed':
                case 'Error':
                  color = 'red'
                  break
                case 'Unknown':
                case 'Terminating':
                default:
                  color = 'grey'
                  break
              }
              return <Tag color={color}>{text}</Tag>
            },
          },
          {
            title: <FormattedMessage id="startAt" />,
            dataIndex: ['metadata', 'creationTimestamp'],
            key: 'startAt',
          },
        ]}
        dataSource={pods}
        rowKey={(c) => c.metadata?.uid || ''}
        pagination={false}
      />
    </ProCard>
  )
}

export const EventTable = (events: Event[]) => {
  const podEvents: Event[] = events
  podEvents.forEach((event) => {
    event.firstTimestamp = event.firstTimestamp || event.eventTime
    event.reportingComponent =
      event.reportingComponent || event.source?.component
  })
  return (
    <ProCard title={<FormattedMessage id="event" />}>
      <Table
        columns={[
          {
            title: <FormattedMessage id="type" />,
            dataIndex: 'type',
            key: 'type',
          },
          {
            title: <FormattedMessage id="reason" />,
            dataIndex: 'reason',
            key: 'reason',
          },
          {
            title: <FormattedMessage id="createAt" />,
            key: 'firstTimestamp',
            dataIndex: 'firstTimestamp',
          },
          {
            title: <FormattedMessage id="from" />,
            dataIndex: 'reportingComponent',
            key: 'reportingComponent',
          },
          {
            title: <FormattedMessage id="message" />,
            key: 'message',
            dataIndex: 'message',
          },
        ]}
        dataSource={podEvents}
        size="small"
        pagination={false}
        rowKey={(c) => c.metadata?.uid || ''}
      />
    </ProCard>
  )
}

const selfLink = (path: string, podName: string) => {
  return <Link to={`${path}`}>{podName}</Link>
}

const LogTools: React.FC<LogToolProps> = (props) => {
  const [logLoading, setLogLoading] = useState<boolean>(false)
  return (
    <Space>
      <Button
        loading={logLoading}
        type="link"
        icon={<SyncOutlined />}
        onClick={() => {
          setLogLoading(true)
          getLog(props.pod, props.container).then((log) => {
            const newLogs = new Map(props.pod.logs)
            newLogs.set(props.container, log)
            props.setPod({
              ...props.pod,
              logs: newLogs,
            })
            setInterval(setLogLoading, 1000, false)
          })
        }}
      >
        {<FormattedMessage id="refresh" />}
      </Button>
      <Typography.Link
        href={URL.createObjectURL(props.data)}
        download={`${props.pod.metadata?.name}-${props.container}.log`}
      >
        <Button type="link" icon={<DownloadOutlined />}>
          {<FormattedMessage id="downloadLog" />}
        </Button>
      </Typography.Link>
    </Space>
  )
}

const DetailedPod: React.FC<unknown> = () => {
  const location = useLocation()
  const params = useParams()
  const [searchParams] = useSearchParams()
  const namespace = params['namespace'] || ''
  const name = params['podName'] || ''
  const format = searchParams.get('raw')
  const container = searchParams.get('log')
  const [pod, setPod] = useState<Pod>()

  useEffect(() => {
    getPod(namespace, name).then((pod) => {
      if (pod) {
        setPod(pod)
      }
    })
  }, [setPod])
  if (namespace === '' || name === '') {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="podNotFound" />,
        }}
      ></PageContainer>
    )
  }
  const ensureLog = (container: string) => {
    if (!pod) {
      return
    }
    if (pod.logs.has(container)) {
      return
    }
    getLog(pod, container).then((log) => {
      const newLogs = new Map(pod.logs)
      newLogs.set(container, log)
      setPod({
        ...pod,
        logs: newLogs,
      })
    })
  }
  if (typeof container === 'string') {
    ensureLog(container)
  }
  const getPobTabsContent = (pod: Pod) => {
    const p = {
      metadata: pod.metadata,
      spec: pod.spec,
      status: pod.status,
    }

    p.metadata?.managedFields?.forEach((managedField) => {
      managedField.fieldsV1 = undefined
    })

    const containers: any[] = []
    pod.status?.containerStatuses?.forEach((cnStatus) => {
      const cnState: string = cnStatus.ready ? 'Ready' : 'NotReady'
      containers.push({
        name: cnStatus.name,
        status: cnState,
        restartCount: cnStatus.restartCount,
        startAt: cnStatus.state?.running?.startedAt,
      })
    })
    pod.status?.initContainerStatuses?.forEach((cnStatus) => {
      const cnState: string = cnStatus.ready ? 'Ready' : 'NotReady'
      containers.push({
        name: cnStatus.name,
        status: cnState,
        restartCount: cnStatus.restartCount,
        startAt: cnStatus.state?.running?.startedAt,
      })
    })
    let content: any

    let exhibitPods: RawPod[] = []
    let exhibitPodTableName = ''
    let currentPodType
    if (pod.mountPods?.length !== 0) {
      exhibitPods = pod.mountPods || []
      exhibitPodTableName = 'Mount Pods'
      currentPodType = 'app'
    }
    if (pod.appPods?.length !== 0) {
      exhibitPods = pod.appPods || []
      exhibitPodTableName = 'App Pods'
      currentPodType = 'mount'
    }
    content = (
      <div>
        <ProCard title={<FormattedMessage id="basic" />}>
          <ProDescriptions
            column={2}
            dataSource={{
              namespace: pod.metadata?.namespace,
              node: pod.node,
              status: pod.finalStatus,
              time: pod.metadata?.creationTimestamp,
            }}
            columns={[
              {
                title: <FormattedMessage id="namespace" />,
                key: 'namespace',
                dataIndex: 'namespace',
              },
              {
                title: <FormattedMessage id="node" />,
                key: 'node',
                dataIndex: 'node',
                render: (_, record) => {
                  if (!record.node) {
                    return '-'
                  }
                  return (
                    <Badge
                      color={getNodeStatusBadge(record.node)}
                      text={record.node.metadata?.name}
                    />
                  )
                },
              },
              {
                title: <FormattedMessage id="status" />,
                key: 'status',
                dataIndex: 'status',
                valueType: 'select',
                valueEnum: PodStatusEnum,
              },
              {
                title: <FormattedMessage id="createAt" />,
                key: 'time',
                dataIndex: 'time',
              },
              {
                title: 'Yaml',
                key: 'yaml',
                render: () => (
                  <Link to={`${location.pathname}?raw=yaml`}>
                    {<FormattedMessage id="clickToView" />}
                  </Link>
                ),
              },
            ]}
          />
        </ProCard>

        <ProCard title={<FormattedMessage id="containerList" />}>
          <Table
            columns={[
              {
                title: <FormattedMessage id="containerName" />,
                dataIndex: 'name',
                key: 'name',
              },
              {
                title: <FormattedMessage id="restartCount" />,
                dataIndex: 'restartCount',
                key: 'restartCount',
              },
              {
                title: <FormattedMessage id="status" />,
                key: 'status',
                dataIndex: 'status',
                render: (status) => {
                  const color = status === 'Ready' ? 'green' : 'red'
                  const text = status === 'Ready' ? 'Ready' : 'NotReady'
                  return <Tag color={color}>{text}</Tag>
                },
              },
              {
                title: <FormattedMessage id="startAt" />,
                dataIndex: 'startAt',
                key: 'startAt',
              },
              {
                title: <FormattedMessage id="log" />,
                key: 'action',
                render: (record) => (
                  <Link to={`${location.pathname}?log=${record.name}`}>
                    {<FormattedMessage id="viewLog" />}
                  </Link>
                ),
              },
            ]}
            dataSource={containers}
            rowKey={(c) => c.name}
            pagination={false}
          />
        </ProCard>

        {getPodTableContent(exhibitPods, exhibitPodTableName, currentPodType)}

        {EventTable(pod.events || [])}
      </div>
    )
    return content
  }

  let content = <Empty />
  let subtitle
  let logTools
  if (!pod) {
    return content
  } else if (
    typeof format === 'string' &&
    (format === 'json' || format === 'yaml')
  ) {
    const raw: RawPod = {
      metadata: pod.metadata,
      spec: pod.spec,
      status: pod.status,
    }
    content = formatData(raw, format)
  } else if (typeof container === 'string') {
    subtitle = container
    const log = pod.logs.get(container)
    if (!log) {
      content = <Empty />
    } else {
      logTools = (
        <LogTools
          pod={pod}
          setPod={setPod}
          container={container}
          data={new Blob([log], { type: 'text/plain' })}
        />
      )
      let logs = log.split('\n')
      let start = 1
      if (logs.length > 1024) {
        start = logs.length - 1023
        logs = logs.splice(logs.length - 1024)
      }
      content = (
        <SyntaxHighlighter
          language={'log'}
          startingLineNumber={start}
          showLineNumbers
          wrapLongLines={true}
        >
          {logs.join('\n').trim()}
        </SyntaxHighlighter>
      )
    }
  } else {
    content = <ProCard direction="column">{getPobTabsContent(pod)}</ProCard>
  }
  return (
    <PageContainer
      fixedHeader
      header={{
        title: selfLink(location.pathname, pod.metadata?.name || ''),
        subTitle: subtitle,
        extra: logTools,
      }}
    >
      {content}
    </PageContainer>
  )
}

export default DetailedPod
