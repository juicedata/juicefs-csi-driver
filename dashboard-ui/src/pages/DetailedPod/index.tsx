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

import { PageContainer, ProCard, ProDescriptions, FooterToolbar } from '@ant-design/pro-components';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { Pod as RawPod, Event } from 'kubernetes-types/core/v1'
import React, { useEffect, useState } from 'react';
import { useParams, useSearchParams, useLocation } from '@umijs/max';
import { getPod, getLog, Pod } from '@/services/pod';
import * as jsyaml from "js-yaml";
import { TabsProps, Select, Empty, Table, Button, Tag, Typography, Space } from "antd";
import { Link } from 'umi';
import { PodStatusEnum } from "@/services/common";
import { SyncOutlined, DownloadOutlined } from '@ant-design/icons';

type LogToolProps = {
    pod: Pod
    setPod: (pod: Pod) => void
    container: string
    data: Blob
}
const LogTools: React.FC<LogToolProps> = (props) => {
    const [logLoading, setLogLoading] = useState<boolean>(false);
    return (
        <Space>
            <Button
                loading={logLoading}
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
            刷新
            </Button>
            <Typography.Link href={URL.createObjectURL(props.data)} download={`${props.pod.metadata?.name}-${props.container}.log`}>
                <Button type="link" icon={<DownloadOutlined />} >
                    下载完整日志
                </Button>
            </Typography.Link>
        </Space>
    )
}

const DetailedPod: React.FC<unknown> = () => {
    const params = useParams()
    console.log(JSON.stringify(params))
    const [searchParams, setSearchParams] = useSearchParams()
    const location = useLocation()
    const namespace = params['namespace']
    const name = params['podName']
    if (!namespace || !name) {
        return (
            <PageContainer
                header={{
                    title: 'Pod 不存在',
                }}
            >
            </PageContainer>
        )
    }
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
        pod.status?.containerStatuses?.forEach((cnStatus, _) => {
            const cnState: string = cnStatus.ready ? "Ready" : "NotReady"
            containers.push({
                name: cnStatus.name,
                status: cnState,
                restartCount: cnStatus.restartCount,
                startAt: cnStatus.state?.running?.startedAt,
            })
        })
        pod.status?.initContainerStatuses?.forEach((cnStatus, _) => {
            const cnState: string = cnStatus.ready ? "Ready" : "NotReady"
            containers.push({
                name: cnStatus.name,
                status: cnState,
                restartCount: cnStatus.restartCount,
                startAt: cnStatus.state?.running?.startedAt,
            })
        })
        let content: any

        let exhibitPods: RawPod[] = []
        let exhibitPodTableName = ""
        let currentPodType
        if (pod.mountPods?.length !== 0) {
            exhibitPods = pod.mountPods || []
            exhibitPodTableName = "Mount Pods"
            currentPodType = 'app'
        }
        if (pod.appPods?.length !== 0) {
            exhibitPods = pod.appPods || []
            exhibitPodTableName = "App Pods"
            currentPodType = 'mount'
        }
        content = <div>
            <ProCard title="基础信息">
                <ProDescriptions
                    column={2}
                    dataSource={{
                        namespace: pod.metadata?.namespace,
                        node: pod.spec?.nodeName,
                        status: pod.status?.phase,
                        time: pod.metadata?.creationTimestamp,
                    }}
                    columns={[
                        {
                            title: '命名空间',
                            key: 'namespace',
                            dataIndex: 'namespace',
                        },
                        {
                            title: '所在节点',
                            key: 'node',
                            dataIndex: 'node',
                        },
                        {
                            title: '状态',
                            key: 'status',
                            dataIndex: 'status',
                            valueType: 'select',
                            valueEnum: PodStatusEnum,
                        },
                        {
                            title: '创建时间',
                            key: 'time',
                            dataIndex: 'time',
                        },
                        {
                            title: 'Yaml',
                            key: 'yaml',
                            render: () => (
                                <Link to={`${location.pathname}?raw=yaml`}>
                                    {'点击查看'}
                                </Link>
                            )
                        },
                    ]}
                />
            </ProCard>

            <ProCard title={"容器列表"}>
                <Table columns={[
                    {
                        title: '容器名',
                        dataIndex: 'name',
                        key: 'name',
                    },
                    {
                        title: '重启次数',
                        dataIndex: 'restartCount',
                        key: 'restartCount',
                    },
                    {
                        title: '状态',
                        key: 'status',
                        dataIndex: 'status',
                        render: (status) => {
                            const color = status === 'Ready' ? 'green' : 'red';
                            const text = status === 'Ready' ? 'Ready' : 'NotReady';
                            return <Tag color={color}>{text}</Tag>;
                        },
                    },
                    {
                        title: '启动时间',
                        dataIndex: 'startAt',
                        key: 'startAt',
                    },
                    {
                        title: "日志",
                        key: 'action',
                        render: (record) => (
                            (
                                <Link to={`${location.pathname}?log=${record.name}`}>
                                    {'查看日志'}
                                </Link>
                            )
                        )
                    }
                ]}
                    dataSource={containers}
                    rowKey={c => c.name}
                    pagination={false}
                />
            </ProCard>

            {getPodTableContent(exhibitPods, exhibitPodTableName, currentPodType)}

            {EventTable(pod.events || [])}
        </div>
        return content
    }

    let content = <Empty />
    let subtitle
    let logTools
    if (!pod) {
        return content
    } else if (typeof format === 'string' && (format === 'json' || format === 'yaml')) {
        pod.metadata?.managedFields?.forEach((managedField) => {
            managedField.fieldsV1 = undefined
        })
        const p = {
            metadata: pod.metadata,
            spec: pod.spec,
            status: pod.status,
        }
        const data = format === 'json' ? JSON.stringify(p, null, "\t") : jsyaml.dump(p)
        content = (
            <SyntaxHighlighter language={format} showLineNumbers>
                {data.trim()}
            </SyntaxHighlighter>
        )
    } else if (typeof container === 'string') {
        subtitle = container
        const log = pod.logs.get(container)
        if (!log) {
            content = <Empty />
        } else {
            logTools = <LogTools pod={pod} setPod={setPod} container={container} data={new Blob([log], { type: 'text/plain' })} />
            let logs = log.split("\n")
            let start = 1
            if (logs.length > 1024) {
                start = logs.length - 1023
                logs = logs.splice(logs.length - 1024)
            }
            content = <SyntaxHighlighter language={"log"} startingLineNumber={start} showLineNumbers wrapLongLines={true}>
                {logs.join("\n").trim()}
            </SyntaxHighlighter>
        }
    } else {
        content = (
            <ProCard direction="column">
                {getPobTabsContent(pod)}
            </ProCard>
        )
    }
    return (
        <PageContainer
            fixedHeader
            header={
                {
                    title: selfLink(location.pathname, pod.metadata?.name || ''),
                    subTitle: subtitle,
                    extra: logTools,
                }}
        >
            {content}
        </PageContainer>
    )
}

export const getPodTableContent = (pods: RawPod[], title: string, podType?: string) => {
    return <ProCard title={title}>
        <Table columns={[
            {
                title: '名称',
                key: 'name',
                render: (pod) => podLink(pod, podType),
            },
            {
                title: 'Namespace',
                key: 'namespace',
                dataIndex: ["metadata", 'namespace'],
            },
            {
                title: '状态',
                key: 'status',
                dataIndex: ['status', "phase"],
                render: (status) => {
                    let color = "grey"
                    let text = "未知"
                    switch (status) {
                        case "Pending":
                            color = 'yellow'
                            text = '等待运行'
                            break
                        case "Running":
                            text = "运行中"
                            color = "green"
                            break
                        case "Succeed":
                            text = "已完成"
                            color = "blue"
                            break
                        case "Failed":
                            text = "失败"
                            color = "red"
                            break
                        case "Unknown":
                        default:
                            text = "未知"
                            color = "grey"
                            break
                    }
                    return <Tag color={color}>{text}</Tag>;
                },
            },
            {
                title: '启动时间',
                dataIndex: ['metadata', 'creationTimestamp'],
                key: 'startAt',
            },
        ]}
            dataSource={pods}
            rowKey={c => c.metadata?.uid!}
            pagination={false}
        />
    </ProCard>
}

export const EventTable = (events: Event[]) => {
    const podEvents: Event[] = events
    podEvents.forEach(event => {
        event.firstTimestamp = event.firstTimestamp || event.eventTime
        event.reportingComponent = event.reportingComponent || event.source?.component
    })
    return <ProCard title={"事件"}>
        <Table columns={[
            {
                title: 'Type',
                dataIndex: 'type',
                key: 'type',
            },
            {
                title: 'Reason',
                dataIndex: 'reason',
                key: 'reason',
            },
            {
                title: 'CreatedTime',
                key: 'firstTimestamp',
                dataIndex: 'firstTimestamp',
            },
            {
                title: 'From',
                dataIndex: 'reportingComponent',
                key: 'reportingComponent',
            },
            {
                title: "Message",
                key: 'message',
                dataIndex: 'message',
            }
        ]}
            dataSource={podEvents}
            size="small"
            pagination={false}
            rowKey={(c) => c.metadata?.uid!}
        />
    </ProCard>
}

const podLink = (pod: Pod, podType?: string) => {
    let base = 'pod'
    if (podType === 'app') {
        base = 'apppod'
    } else if (podType === 'mount') {
        base = 'mountpod'
    }
    return <Link to={`/${base}/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
        {pod.metadata?.name}
    </Link>
}

const selfLink = (path: string, podName: string) => {
    return <Link to={`${path}`}>
        {podName}
    </Link>
}

export default DetailedPod;
