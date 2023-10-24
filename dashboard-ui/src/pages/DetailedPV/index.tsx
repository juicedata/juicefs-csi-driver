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

import { PageContainer, PageLoading, ProCard, ProDescriptions } from '@ant-design/pro-components';
import React, { useEffect, useState } from 'react';
import { useMatch } from '@umijs/max';
import { useParams, useSearchParams } from '@umijs/max';
import { getMountPodOfPV, getPVC, getPV, getPVEvents } from '@/services/pv';
import * as jsyaml from "js-yaml";
import { Empty, List, TabsProps } from "antd";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { PVStatusEnum } from "@/services/common"
import { Pod as RawPod, PersistentVolume, Event } from "kubernetes-types/core/v1"
import { EventTable, getPodTableContent } from "@/pages/DetailedPod";
import { Link } from 'umi';
import { formatData } from '../utils';

const DetailedPV: React.FC<unknown> = () => {
    const params = useParams()
    const [searchParams, setSearchParams] = useSearchParams()
    const pvName = params['pvName']
    const format = searchParams.get('raw')
    if (!pvName) {
        return (
            <PageContainer
                header={{
                    title: 'PV 不存在',
                }}
            >
            </PageContainer>
        )
    }
    const [pv, setPV] = useState<PersistentVolume>()
    const [mountpods, setMountPods] = useState<RawPod[]>()
    const [events, setEvents] = useState<Event[]>()
    const pvcNamespace = pv?.spec?.claimRef?.namespace || ""
    const pvcName = pv?.spec?.claimRef?.name || ""

    useEffect(() => {
        getPV(pvName).then(setPV)
        getMountPodOfPV(pvName).then(setMountPods)
    }, [setPV, setMountPods])
    useEffect(() => {
        getPVEvents(pvName).then(setEvents)
    }, [setEvents]);

    const getPVTabsContent = (pv: PersistentVolume, pvcNamespace: string, pvName: string) => {
        const p = {
            metadata: pv.metadata,
            spec: pv.spec,
            status: pv.status,
        }

        p.metadata?.managedFields?.forEach((managedField) => {
            managedField.fieldsV1 = undefined
        })
        const accessModeMap: { [key: string]: string } = {
            ReadWriteOnce: 'RWO',
            ReadWriteMany: 'RWM',
            ReadOnlyMany: 'ROM',
            ReadWriteOncePod: 'RWOP',
        };

        let content: any
        let parameters: string[] = []
        const volumeAttributes = pv.spec?.csi?.volumeAttributes
        if (volumeAttributes) {
            for (const key in volumeAttributes) {
                if (volumeAttributes.hasOwnProperty(key)) {
                    const value = volumeAttributes[key];
                    parameters.push(`${key}: ${value}`)
                }
            }
        }
        content = <div>
            <ProCard title="基础信息">
                <ProDescriptions
                    column={2}
                    dataSource={{
                        pvc: `${pvcNamespace}/${pvcName}`,  // todo: link
                        capacity: pv.spec?.capacity?.storage,
                        accessMode: pv.spec?.accessModes?.map(accessMode => accessModeMap[accessMode] || 'Unknown').join(","),
                        reclaimPolicy: pv.spec?.persistentVolumeReclaimPolicy,
                        storageClass: pv.spec?.storageClassName,
                        volumeHandle: pv.spec?.csi?.volumeHandle,
                        status: pv.status?.phase,
                        time: pv.metadata?.creationTimestamp,
                    }}
                    columns={[
                        {
                            title: 'PVC',
                            key: 'pvc',
                            dataIndex: 'pvc',
                            render: (_, record) => {
                                const [namespace, name] = record.pvc.split("/")
                                return <Link to={`/pvc/${namespace}/${name}`}>{record.pvc}</Link>
                            }
                        },
                        {
                            title: '容量',
                            key: 'capacity',
                            dataIndex: 'capacity',
                        },
                        {
                            title: '访问模式',
                            key: 'accessMode',
                            dataIndex: 'accessMode',
                        },
                        {
                            title: '回收策略',
                            key: 'reclaimPolicy',
                            dataIndex: 'reclaimPolicy',
                        },
                        {
                            title: 'StorageClass',
                            key: 'storageClass',
                            render: (_, record) => {
                                return <Link to={`/storageclass/${record.storageClass}`}>{record.storageClass}</Link>
                            }
                        },
                        {
                            title: 'volumeHandle',
                            key: 'volumeHandle',
                            dataIndex: 'volumeHandle',
                        },
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
            <ProCard title="VolumeAttributes">
                <List
                    dataSource={parameters}
                    split={false}
                    size="small"
                    renderItem={(item) => <List.Item><code>{item}</code></List.Item>}
                />
            </ProCard>
            <ProCard title="挂载参数">
                <List
                    dataSource={pv.spec?.mountOptions}
                    split={false}
                    size="small"
                    renderItem={(item) => <List.Item><code>{item}</code></List.Item>}
                />
            </ProCard>

            {getPodTableContent(mountpods || [], "Mount Pods")}

            {EventTable(events || [])}
        </div>
        return content
    }

    let contents
    if (!pv) {
        return <PageLoading />
    } else if (typeof format === 'string' && (format === 'json' || format === 'yaml')) {
        contents = formatData(pv, format)
    } else {
        contents = (
            <ProCard direction="column">
                {getPVTabsContent(pv, pvcNamespace, pvName)}
            </ProCard>
        )
    }
    return (
        <PageContainer
            fixedHeader
            header={{
                title: `PersistentVolume: ${pv?.metadata?.name}`,
            }}
        >
            {contents}
        </PageContainer>
    )
}

export default DetailedPV;
