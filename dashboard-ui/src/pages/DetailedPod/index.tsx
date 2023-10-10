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

import {PageContainer, ProCard, ProDescriptions} from '@ant-design/pro-components';
import {Prism as SyntaxHighlighter} from 'react-syntax-highlighter';
import {Pod as RawPod} from 'kubernetes-types/core/v1'
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getPod, getLog, Pod} from '@/services/pod';
import * as jsyaml from "js-yaml";
import {TabsProps, Select, Empty, Table, Button} from "antd";
import {Link} from 'umi';
import {PodStatusEnum} from "@/services/common";

const DetailedPod: React.FC<unknown> = () => {
    const routeData = useMatch('/pod/:namespace/:name')
    const namespace = routeData?.params?.namespace
    const name = routeData?.params?.name
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
    const [pod, setPod] = useState<Pod>()
    const [container, setContainer] = useState<string>()

    useEffect(() => {
        getPod(namespace, name).then((pod) => {
            if (pod) {
                setPod(pod)
            }
        })
    }, [setPod])

    const getContainer = () => (container || pod?.spec?.containers?.[0].name)
    const [activeTab, setActiveTab] = useState('1');
    const ensureLog = (container: string) => {
        if (pod!.logs.has(container)) {
            return
        }
        getLog(pod!, container).then((log) => {
            const newLogs = new Map(pod!.logs)
            newLogs.set(container, log)
            setPod({
                ...pod!,
                logs: newLogs,
            })
        })
    }

    const handleTabChange = (key: string) => {
        setActiveTab(key);
        if (key === '3' && pod) {
            const cname = getContainer()!
            ensureLog(cname)
        }
    };

    const handleContainerChange = (container: string) => {
        setContainer(container)
        ensureLog(container)
    }

    const getPodTabs = (pod: Pod) => {
        let tabList: TabsProps['items'] = [
            {
                key: '1',
                label: '状态',
            },
            {
                key: '2',
                label: 'Yaml',
            },
            {
                key: '3',
                label: '日志',
            },
            {
                key: '4',
                label: '事件',
            },
        ]
        if (pod.mountPods?.length !== 0) {
            tabList.push({
                key: '5',
                label: 'Mount Pods',
            })
        } else if (pod.appPods?.length !== 0) {
            tabList.push({
                key: '5',
                label: 'App Pods',
            })
        }
        return tabList
    }

    const getPobTabsContent = (activeTab: string, pod: Pod) => {
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
        let content: any
        const handleButtonClick = (container: string) => {
            setActiveTab('3');
            setContainer(container);
            ensureLog(container)
        };

        switch (activeTab) {
            case "1":
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
                                    valueEnum: {
                                        all: {text: 'Running', status: 'Default'},
                                        Running: {
                                            text: 'Running',
                                            status: 'Success',
                                        },
                                        Pending: {
                                            text: 'Pending',
                                            status: 'Pending',
                                        },
                                    },
                                },
                                {
                                    title: '创建时间',
                                    key: 'time',
                                    dataIndex: 'time',
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
                                    <Button type="link" onClick={() => handleButtonClick(record.name)}>
                                        查看日志
                                    </Button>
                                )
                            }
                        ]}
                               dataSource={containers}
                               rowKey={c => c.name}
                        />
                    </ProCard>
                </div>
                break
            case "2":
                content = <SyntaxHighlighter language="yaml">
                    {jsyaml.dump(p)}
                </SyntaxHighlighter>
                break
            case "3":
                console.log(`logs: ${pod.logs}`)
                const container = getContainer()!
                if (!pod.logs.has(container)) {
                    content = <Empty/>
                } else {
                    const log = pod.logs.get(container)!
                    if (log.length < 16 * 1024) {
                        content = <SyntaxHighlighter language={"log"} wrapLongLines={true}>
                            {log}
                        </SyntaxHighlighter>
                    } else {
                        content = <pre><code>{log}</code></pre>
                    }

                }
                break
            case '4':
                if (pod.events?.length === 0) {
                    content = <Empty/>
                } else {
                    content = <div>
                        <pre><code>{pod.events}</code></pre>
                    </div>
                }
                break
            case "5":
                if (pod.mountPods && pod.mountPods?.length != 0) {
                    content = getPodTableContent(pod.mountPods)
                } else if (pod.appPods && pod.appPods.length != 0) {
                    content = getPodTableContent(pod.appPods)
                }
        }
        return content
    }


    if (!pod) {
        return <Empty/>
    } else {
        const tabList: TabsProps['items'] = getPodTabs(pod)
        const footer = []
        if (activeTab === '3') {
            const containers: string[] = []
            pod.spec?.containers?.forEach((container) => {
                containers.push(container.name)
            })
            pod.spec?.initContainers?.forEach((container) => {
                containers.push(container.name)
            })
            if (containers.length > 1) {
                footer.push(
                    <Select
                        key="container"
                        placeholder='选择容器'
                        value={container}
                        style={{width: 200}}
                        onChange={handleContainerChange}
                        options={containers.map((container) => {
                            return {
                                value: container,
                                label: container,
                            }
                        })}
                    />
                )
            }
        }
        return (
            <PageContainer
                fixedHeader
                header={{
                    title: `应用 Pod: ${pod.metadata?.name}`,
                }}
                tabList={tabList}
                tabActiveKey={activeTab}
                onTabChange={handleTabChange}
                footer={footer}
            >
                <ProCard
                    direction="column"
                >
                    {getPobTabsContent(activeTab, pod)}
                </ProCard>
            </PageContainer>
        )
    }
}

export const getPodTableContent = (pods: RawPod[]) => {
    return (
        <ProCard
            direction="column"
            gutter={[0, 16]}
            style={{marginBlockStart: 8}}>
            {pods.map((pod) => (
                <ProCard title={`${pod.metadata?.name}`} type="inner" bordered
                         extra={<Link to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/`}> 查看详情 </Link>}>
                    <ProDescriptions
                        column={4}
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
                        ]}
                    >
                    </ProDescriptions>
                </ProCard>
            ))}
        </ProCard>
    )
}
export default DetailedPod;
