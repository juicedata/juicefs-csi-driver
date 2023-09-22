import {PageContainer, PageLoading, ProCard} from '@ant-design/pro-components';
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getPod, Pod} from '@/services/pod';
import * as jsyaml from "js-yaml";
import {TabsProps} from "antd";

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
        getPod(namespace, name).then(setPod)
    }, [setPod])

    const [activeTab, setActiveTab] = useState('1');
    const handleTabChange = (key: any) => {
        setActiveTab(key);
    };

    if (!pod) {
        return <PageLoading/>
    } else {
        const tabList: TabsProps['items'] = [
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
                label: 'Mount Pods',
            },
        ];

        const p = {
            metadata: pod.metadata,
            spec: pod.spec,
            status: pod.status,
        }
        const podYaml = jsyaml.dump(p)
        const podLog = pod.log


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
                content = <div>
                    <pre><code>{podYaml}</code></pre>
                </div>
                break
            case "2":
                content = <div>
                    <pre><code>{podLog}</code></pre>
                </div>
                break
            case "3":
                content = (
                    <div>
                        {Array.from(mountPods).map(([name, mountPod]) => (
                            <ProCard
                                key={name}
                                title={name}
                                headerBordered
                                collapsible
                                defaultCollapsed
                            >
                                <pre><code>{jsyaml.dump(mountPod)}</code></pre>
                            </ProCard>
                        ))}
                    </div>
                );
        }

        return (
            <PageContainer
                fixedHeader
                header={{
                    title: pod?.metadata?.name,
                }}
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <ProCard direction="column">
                    <ProCard style={{height: 200}}/>
                    {content}
                </ProCard>
            </PageContainer>
        )
    }
}

export default DetailedPod;
