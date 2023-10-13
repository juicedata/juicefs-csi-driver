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

import {PageContainer, PageLoading, ProCard, ProDescriptions} from '@ant-design/pro-components';
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getMountPodOfPVC, getPVC, getPVCEvents, getPVEvents, PV, PVC} from '@/services/pv';
import * as jsyaml from "js-yaml";
import {TabsProps} from "antd";
import {PVStatusEnum} from "@/services/common";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";
import {EventTable, getPodTableContent} from "@/pages/DetailedPod";
import {Pod as RawPod, PersistentVolumeClaim, Event} from "kubernetes-types/core/v1";

const DetailedPVC: React.FC<unknown> = () => {
    const routeData = useMatch('/pvc/:namespace/:name')
    const namespace = routeData?.params?.namespace
    const name = routeData?.params?.name
    if (!namespace || !name) {
        return (
            <PageContainer
                header={{
                    title: 'PVC 不存在',
                }}
            >
            </PageContainer>
        )
    }
    const [pvc, setPV] = useState<PersistentVolumeClaim>()
    const [mountpods, setMountPod] = useState<RawPod[]>()
    const [events, setEvents] = useState<Event[]>()
    useEffect(() => {
        getPVC(namespace, name)
            .then(setPV)
            .then(() => getMountPodOfPVC(namespace, name))
            .then(setMountPod)
    }, [setPV, setMountPod])
    useEffect(() => {
        getPVCEvents(pvc?.metadata?.namespace || "", pvc?.metadata?.name || "").then(setEvents)
    }, []);

    const getPVCTabsContent = (pvc: PersistentVolumeClaim) => {
        const accessModeMap: { [key: string]: string } = {
            ReadWriteOnce: 'RWO',
            ReadWriteMany: 'RWM',
            ReadOnlyMany: 'ROM',
            ReadWriteOncePod: 'RWOP',
        };

        let content: any
        content = <div>
            <ProCard title="基础信息">
                <ProDescriptions
                    column={2}
                    dataSource={{
                        name: pvc.metadata?.name,
                        namespace: pvc.metadata?.namespace,
                        pv: `${pvc.spec?.volumeName || "-"}`,  // todo: link
                        capacity: pvc.spec?.resources?.requests?.storage,
                        accessMode: pvc.spec?.accessModes?.map(accessMode => accessModeMap[accessMode] || 'Unknown').join(","),
                        storageClass: pvc.spec?.storageClassName,
                        status: pvc.status?.phase,
                        time: pvc.metadata?.creationTimestamp,
                    }}
                    columns={[
                        {
                            title: "名称",
                            key: 'name',
                            dataIndex: 'name',
                        },
                        {
                            title: "命名空间",
                            key: 'namespace',
                            dataIndex: 'namespace',
                        },
                        {
                            title: 'PV',
                            key: 'pv',
                            dataIndex: 'pv',
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
                            title: 'StorageClass',
                            key: 'storageClass',
                            dataIndex: 'storageClass',
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
                            render: (index) => {
                                // todo
                                return <div>点击查看</div>
                            }
                        },
                    ]}
                />
            </ProCard>

            {getPodTableContent(mountpods || [], "Mount Pods")}

            {EventTable(events || [])}
        </div>
        return content
    }

    if (!pvc) {
        return <PageLoading/>
    } else {
        return (
            <PageContainer
                header={{
                    title: `PersistentVolumeClaim: ${pvc?.metadata?.name}`,
                }}
                fixedHeader
            >
                <ProCard direction="column">
                    {getPVCTabsContent(pvc)}
                </ProCard>
            </PageContainer>
        )
    }
}

export default DetailedPVC;
