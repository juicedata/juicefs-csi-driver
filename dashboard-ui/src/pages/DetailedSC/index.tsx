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
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getSC, getPVOfSC} from '@/services/pv';
import * as jsyaml from "js-yaml";
import {Empty, List, TabsProps} from "antd";
import {StorageClass} from "kubernetes-types/storage/v1";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";
import {PersistentVolume} from "kubernetes-types/core/v1";
import {Link} from "@@/exports";
import {PVStatusEnum} from "@/services/common";

const DetailedSC: React.FC<unknown> = () => {
    const routeData = useMatch('/storageclass/:name/')
    const scName = routeData?.params?.name
    if (!scName) {
        return (
            <PageContainer
                header={{
                    title: 'StorageClass 不存在',
                }}
            >
            </PageContainer>
        )
    }
    const [sc, setSC] = useState<StorageClass>()
    const [pvs, setPVs] = useState<PersistentVolume[]>()

    useEffect(() => {
        getSC(scName).then(setSC)
    }, [setSC])
    useEffect(() => {
        getPVOfSC(scName).then(setPVs)
    }, [setPVs])

    const [activeTab, setActiveTab] = useState('1');
    const handleTabChange = (key: string) => {
        setActiveTab(key);
    };

    const getSCTabs = (sc: StorageClass) => {
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
                key: "3",
                label: 'PV',
            }
        ]
        return tabList
    }

    const getSCTabsContent = (activeTab: string, sc: StorageClass) => {
        let content: any
        let parameters: string[] = []
        for (const key in sc.parameters) {
            if (sc.parameters.hasOwnProperty(key)) {
                const value = sc.parameters[key];
                parameters.push(`${key}: ${value}`)
            }
        }
        switch (activeTab) {
            case "1":
                content = <div>
                    <ProCard title="基础信息">
                        <ProDescriptions
                            column={2}
                            dataSource={{
                                reclaimPolicy: sc.reclaimPolicy,
                                provisioner: sc.provisioner,
                                expansion: sc.allowVolumeExpansion ? "支持" : "不支持",
                                time: sc.metadata?.creationTimestamp,
                            }}
                            columns={[
                                {
                                    title: '回收策略',
                                    key: 'reclaimPolicy',
                                    dataIndex: 'reclaimPolicy',
                                },
                                {
                                    title: 'Provisioner',
                                    key: 'provisioner',
                                    dataIndex: 'provisioner',
                                },
                                {
                                    title: '支持扩容',
                                    key: 'expansion',
                                    dataIndex: 'expansion',
                                },
                                {
                                    title: '创建时间',
                                    key: 'time',
                                    dataIndex: 'time',
                                },
                            ]}
                        />
                    </ProCard>
                    <ProCard title="Paramters">
                        <List
                            dataSource={parameters}
                            split={false}
                            size="small"
                            renderItem={(item) => <List.Item><code>{item}</code></List.Item>}
                        />
                    </ProCard>
                    <ProCard title="挂载参数">
                        <List
                            dataSource={sc.mountOptions}
                            split={false}
                            size="small"
                            renderItem={(item) => <List.Item><code>{item}</code></List.Item>}
                        />
                    </ProCard>
                </div>
                break
            case "2":
                content = <SyntaxHighlighter language="yaml">
                    {jsyaml.dump(sc)}
                </SyntaxHighlighter>
                break
            case "3":
                if (pvs) {
                    content = getPVsResult(pvs)
                }
        }
        return content
    }

    const getPVsResult = (pvs: PersistentVolume[]) => {
        return (
            <ProCard
                direction="column"
                gutter={[0, 16]}
                style={{marginBlockStart: 8}}>
                {pvs.map((pv) => (
                    <ProCard title={`${pv.metadata?.name}`} type="inner" bordered
                             extra={<Link to={`/pv/${pv.metadata?.name}/`}> 查看详情 </Link>}>
                        <ProDescriptions
                            column={2}
                            dataSource={{
                                status: pv.status?.phase,
                                time: pv.metadata?.creationTimestamp,
                            }}
                            columns={[
                                {
                                    title: '状态',
                                    key: 'status',
                                    dataIndex: 'status',
                                    valueType: 'select',
                                    valueEnum: PVStatusEnum,
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

    if (!sc) {
        return <Empty/>
    } else {
        const tabList: TabsProps['items'] = getSCTabs(sc)
        return (
            <PageContainer
                fixedHeader
                header={{
                    title: `StorageClass: ${sc?.metadata?.name}`,
                }}
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <ProCard direction="column">
                    {getSCTabsContent(activeTab, sc)}
                </ProCard>
            </PageContainer>
        )
    }
}

export default DetailedSC;
