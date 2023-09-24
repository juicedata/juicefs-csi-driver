import {PageContainer, PageLoading, ProCard} from '@ant-design/pro-components';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import {Pod as RawPod} from 'kubernetes-types/core/v1'
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getPod, getLog, Pod} from '@/services/pod';
import * as jsyaml from "js-yaml";
import {TabsProps, Select } from "antd";
import { Link } from 'umi';

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
    const handleTabChange = (key: string) => {
        setActiveTab(key);
        if (key === '2' && pod) {
            const cname = getContainer()!
            if (pod.logs.has(cname)) {
                return
            }
            getLog(pod, cname).then((log) => {
                const newLogs = new Map(pod!.logs)
                newLogs.set(cname, log)
                setPod({
                    ...pod,
                    logs: newLogs
                })
            })
        }
    };

    const handleContainerChange = (container: string) => {
        setContainer(container)
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

    if (!pod) {
        return <PageLoading/>
    } else {
        const tabList: TabsProps['items'] = getPodTabs(pod)
        const extra = []
        if (activeTab === '2') {
            const containers: string[] = []
            pod.spec?.containers?.forEach((container) => {
                containers.push(container.name)
            })
            pod.spec?.initContainers?.forEach((container) => {
                containers.push(container.name)
            })
            if (containers.length > 1) {
                extra.push(
                    <Select
                        key="container"
                        placeholder='选择容器'
                        value={container}
                        style={{ width: 200 }}
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
                    title: pod.metadata?.name,
                    extra: extra,
                }}
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <ProCard direction="column">
                    {getPobTabsContent(activeTab, pod, getContainer()!)}
                </ProCard>
            </PageContainer>
        )
    }
}

const getPodTabs = (pod: Pod) => {
    let tabList: TabsProps['items'] = [
        {
            key: '1',
            label: 'Yaml',
        },
        {
            key: '2',
            label: '日志',
        },
        {
            key: '3',
            label: '事件',
        },
    ]
    if (pod.mountPods?.size !== 0) {
        tabList.push({
            key: '4',
            label: 'Mount Pods',
        })
    }
    return tabList
}

const getPobTabsContent = (activeTab: string, pod: Pod, container: string) => {
    const p = {
        metadata: pod.metadata,
        spec: pod.spec,
        status: pod.status,
    }

    p.metadata?.managedFields?.forEach((managedField) => {
        managedField.fieldsV1 = undefined
    })

    let mountPods: Map<string, RawPod> = new Map
    if (pod.mountPods && pod.mountPods.size != 0) {
        pod.mountPods.forEach((mountPod, pvcName) => {
            if (mountPod.metadata != undefined && mountPod.metadata?.name != undefined) {
                mountPods.set(mountPod.metadata.name, mountPod)
            }
        })
    }

    let content: any
    switch (activeTab) {
        case "1":
            content = <SyntaxHighlighter language="yaml">
                {jsyaml.dump(p)}
            </SyntaxHighlighter>
            break
        case "2":
            console.log(`logs: ${pod.logs}`)
            if (!pod.logs.has(container)) {
                content = <PageLoading/>
            } else {
                const log = pod.logs.get(container)!
                let language = "text"
                if (log.length < 16 * 1024) {
                    language = "log"
                }
                content = <SyntaxHighlighter language={language} wrapLongLines={true}>
                    {log}
                </SyntaxHighlighter>
            }
            break
        case '3':
            content = <div>
                <pre><code>{pod.events}</code></pre>
            </div>
            break
        case "4":
            if (pod.mountPods) {
                content = getMountPodsResult(mountPods)
            }
    }
    return content
}

function getMountPodsResult(mountPods: Map<string, RawPod>): any {
    return (
        <div>
            {Array.from(mountPods).map(([name, mountPod]) => (
                <ProCard
                    key={name}
                    title={
                        <Link to={`/pod/${mountPod.metadata?.namespace}/${name}`}>
                            {name}
                        </Link>
                    }
                    headerBordered
                    collapsible
                    defaultCollapsed
                >
                    <pre><code>{jsyaml.dump(mountPod)}</code></pre>
                </ProCard>
            ))}
        </div>
    )
}

export default DetailedPod;
