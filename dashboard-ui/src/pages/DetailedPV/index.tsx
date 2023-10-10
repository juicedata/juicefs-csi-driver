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
import {getMountPodOfPVC, getPVC, getPV, PV} from '@/services/pv';
import * as jsyaml from "js-yaml";
import {Empty, List, TabsProps} from "antd";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";
import {PVStatusEnum} from "@/services/common"
import {Pod as RawPod} from "kubernetes-types/core/v1"
import {getPodTableContent} from "@/pages/DetailedPod";

const DetailedPV: React.FC<unknown> = () => {
    const routeData = useMatch('/pv/:name')
    const pvName = routeData?.params?.name
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
    const [pv, setPV] = useState<PV>()
    const [mountpods, setMountPods] = useState<RawPod[]>()
    const pvcNamespace = pv?.spec?.claimRef?.namespace || ""
    const pvcName = pv?.spec?.claimRef?.name || ""

    useEffect(() => {
        getPV(pvName).then(setPV)
        if (pvcNamespace && pvcName) {
            getPVC(pvcNamespace, pvName)
                .then(setPV)
                .then(() => getMountPodOfPVC(pvcNamespace, pvName))
                .then(setMountPods)
        }
    }, [setPV, setMountPods])

    const [activeTab, setActiveTab] = useState('1');
    const handleTabChange = (key: string) => {
        setActiveTab(key);
    };
    const getPVTabs = (pv: PV) => {
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
                label: '事件',
            },
        ]
        if (mountpods) {
            tabList.push({
                key: '4',
                label: 'Mount Pod',
            })
        }
        return tabList
    }

    const getPVTabsContent = (activeTab: string, pv: PV, pvcNamespace: string, pvName: string) => {
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
        switch (activeTab) {
            case "1":
                content = <div>
                    <ProCard title="基础信息">
                        <ProDescriptions
                            column={2}
                            dataSource={{
                                pvc: `${pvcNamespace}/${pvName}`,  // todo: link
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
                                    dataIndex: 'storageClass',
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
                </div>
                break
            case "2":
                content = <SyntaxHighlighter language="yaml">
                    {jsyaml.dump(p)}
                </SyntaxHighlighter>
                break
            case '3':
                content = <div>todo...</div>
                break
            case "4":
                if (mountpods != undefined) {
                    content = getPodTableContent(mountpods)
                }
        }
        return content
    }

    if (!pv) {
        return <Empty/>
    } else {
        const tabList: TabsProps['items'] = getPVTabs(pv)
        return (
            <PageContainer
                fixedHeader
                header={{
                    title: `持久卷: ${pv?.metadata?.name}`,
                }}
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <ProCard direction="column">
                    {getPVTabsContent(activeTab, pv, pvcNamespace, pvName)}
                </ProCard>
            </PageContainer>
        )
    }
}

export default DetailedPV;
