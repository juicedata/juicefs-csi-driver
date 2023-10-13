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
import {Empty, List, Table, TabsProps, Tag} from "antd";
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

    const getSCTabsContent = (sc: StorageClass) => {
        let content: any
        let parameters: string[] = []
        for (const key in sc.parameters) {
            if (sc.parameters.hasOwnProperty(key)) {
                const value = sc.parameters[key];
                parameters.push(`${key}: ${value}`)
            }
        }
        content = <div>
            <ProCard title="基础信息">
                <ProDescriptions
                    column={2}
                    dataSource={{
                        reclaimPolicy: sc.reclaimPolicy,
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
                            title: '支持扩容',
                            key: 'expansion',
                            dataIndex: 'expansion',
                        },
                        {
                            title: '创建时间',
                            key: 'time',
                            dataIndex: 'time',
                        },
                        {
                            title: 'Yaml',
                            key: 'yaml',
                            render: (index) => {
                                // todo
                                return <div>点击查看</div>
                            }
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

            <ProCard title={"PV"}>
                <Table columns={[
                    {
                        title: '名称',
                        key: 'name',
                        render: (pv) => (
                            <Link to={`/pv/${pv.metadata.name}/`}>
                                {pv.metadata.name}
                            </Link>
                        ),
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
                                case "Bound":
                                    text = "已绑定"
                                    color = "green"
                                    break
                                case "Available":
                                    text = "可绑定"
                                    color = "blue"
                                    break
                                case "Failed":
                                    text = "失败"
                                    color = "red"
                                    break
                                case "Released":
                                default:
                                    text = "已释放"
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
                       dataSource={pvs}
                       rowKey={c => c.metadata?.uid!}
                       pagination={false}
                />
            </ProCard>
        </div>
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
        return (
            <PageContainer
                fixedHeader
                header={{
                    title: `StorageClass: ${sc?.metadata?.name}`,
                }}
            >
                <ProCard direction="column">
                    {getSCTabsContent(sc)}
                </ProCard>
            </PageContainer>
        )
    }
}

export default DetailedSC;
