import {PageContainer, PageLoading, ProCard} from '@ant-design/pro-components';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getPod, getLog, Pod} from '@/services/pod';
import * as jsyaml from "js-yaml";
import {TabsProps} from "antd";
import {Link} from "@@/exports";

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
    useEffect(() => {
        getPod(namespace, name).then((pod) => {
            if (pod) {
                setPod(pod)
            }
        })
    }, [setPod])

    const [activeTab, setActiveTab] = useState('1');
    const handleTabChange = (key: string) => {
        setActiveTab(key);
        if (key === '2' && pod && !(pod.log)) {
            getLog(pod).then((log) => {
                setPod({
                    ...pod,
                    log: log
                })
            })
        }
    };
    if (!pod) {
        return <PageLoading/>
    } else {
        const tabList: TabsProps['items'] = getPodTabs(pod)

        return (
            <PageContainer
                fixedHeader
                header={{
                    title: pod.metadata?.name,
                }}
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <ProCard direction="column">
                    {getPobTabsContent(activeTab, pod)}
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

const getPobTabsContent = (activeTab: string, pod: Pod) => {
    const p = {
        metadata: pod.metadata,
        spec: pod.spec,
        status: pod.status,
    }

    p.metadata?.managedFields?.forEach((managedField) => {
        managedField.fieldsV1 = undefined
    })

    let mountPods: Map<string, Pod> = new Map
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
            if (pod.log === undefined) {
                content = <PageLoading/>
            } else {
                let language = "text"
                if (pod.log.length < 16 * 1024) {
                    language = "log"
                }
                content = <SyntaxHighlighter language={language} wrapLongLines={true}>
                    {pod.log}
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

function getMountPodsResult(mountPods: Map<string, Pod>): any {
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
