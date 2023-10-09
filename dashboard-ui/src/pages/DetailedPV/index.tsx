import {PageContainer, PageLoading, ProCard, ProDescriptions} from '@ant-design/pro-components';
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {getLog, Pod} from '@/services/pod';
import {getMountPodOfPVC, getPVC, getPV, PV} from '@/services/pv';
import {Link} from 'umi';
import * as jsyaml from "js-yaml";
import {Empty, Table, TabsProps} from "antd";
import {Pod as RawPod} from "kubernetes-types/core/v1";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";

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
    const [mountpods, setMountPods] = useState<Pod>()
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
    if (pv.Pod) {
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
                                valueEnum: {
                                    Pending: {
                                        text: '等待运行',
                                        color: 'yellow',
                                    },
                                    Bound: {
                                        text: '已绑定',
                                        color: 'green',
                                    },
                                    Available: {
                                        text: '可绑定',
                                        color: 'blue',
                                    },
                                    Released: {
                                        text: '已释放',
                                        color: 'grey',
                                    },
                                    Failed: {
                                        text: '失败',
                                        color: 'red',
                                    }
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
            content = <div>todo...</div>
    }
    return content
}
export default DetailedPV;
