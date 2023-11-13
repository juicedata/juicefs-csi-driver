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
import {useParams, useSearchParams, useLocation} from '@umijs/max';
import {getSC, getPVOfSC} from '@/services/pv';
import * as jsyaml from "js-yaml";
import {Empty, List, Table, TabsProps, Tag} from "antd";
import {StorageClass} from "kubernetes-types/storage/v1";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";
import {PersistentVolume} from "kubernetes-types/core/v1";
import {Link} from "@@/exports";
import {PVStatusEnum} from "@/services/common";
import {formatData} from '../utils';

const DetailedSC: React.FC<unknown> = () => {
    const location = useLocation()
    const params = useParams()
    const [searchParams, setSearchParams] = useSearchParams()
    const scName = params['scName']
    const format = searchParams.get('raw')
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
                            render: () => (
                                <Link to={`${location.pathname}?raw=yaml`}>
                                    {'点击查看'}
                                </Link>
                            )
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
                            let text = status
                            switch (status) {
                                case "Pending":
                                    color = 'yellow'
                                    break
                                case "Bound":
                                    color = "green"
                                    break
                                case "Available":
                                    color = "blue"
                                    break
                                case "Failed":
                                    color = "red"
                                    break
                                case "Released":
                                default:
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

    let contents
    if (!sc) {
        return <Empty/>
    } else if (typeof format === 'string' && (format === 'json' || format === 'yaml')) {
        contents = formatData(sc, format)
    } else {
        contents = (
            <ProCard direction="column">
                {getSCTabsContent(sc)}
            </ProCard>
        )
    }
    return (
        <PageContainer
            fixedHeader
            header={{
                title: <Link to={`/storageclass/${sc.metadata?.name}`}>
                    {sc.metadata?.name}
                </Link>
            }}
        >
            {contents}
        </PageContainer>
    )
}

export default DetailedSC;
