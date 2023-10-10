import {PageContainer, PageLoading, ProCard, ProDescriptions} from '@ant-design/pro-components';
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {Pod} from '@/services/pod';
import {getMountPodOfPVC, getPVC, PV, PVC} from '@/services/pv';
import {Link} from 'umi';
import * as jsyaml from "js-yaml";
import {List, TabsProps} from "antd";
import {PVStatusEnum} from "@/services/common";
import {Prism as SyntaxHighlighter} from "react-syntax-highlighter";
import {getMountPodsResult} from "@/pages/DetailedPod";
import {Pod as RawPod, PersistentVolume, PersistentVolumeClaim} from "kubernetes-types/core/v1";

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
    useEffect(() => {
        getPVC(namespace, name)
            .then(setPV)
            .then(() => getMountPodOfPVC(namespace, name))
            .then(setMountPod)
    }, [setPV, setMountPod])

    const [activeTab, setActiveTab] = useState('1');
    const handleTabChange = (key: any) => {
        setActiveTab(key);
    };

    const getPVCTabs = () => {
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
    const getPVCTabsContent = (activeTab: string, pvc: PersistentVolumeClaim) => {
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
                                name: pvc.metadata?.name,
                                namespace: pvc.metadata?.namespace,
                                pv: `${pvc.spec?.volumeName}`,  // todo: link
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
                            ]}
                        />
                    </ProCard>
                </div>
                break
            case "2":
                content = <SyntaxHighlighter language="yaml">
                    {jsyaml.dump(pvc)}
                </SyntaxHighlighter>
                break
            case '3':
                content = <div>todo...</div>
                break
            case "4":
                if (mountpods != undefined) {
                    content = getMountPodsResult(mountpods)
                }
        }
        return content
    }

    if (!pvc) {
        return <PageLoading/>
    } else {
        const tabList: TabsProps['items'] = getPVCTabs()
        return (
            <PageContainer
                header={{
                    title: `持久卷: ${pvc?.metadata?.name}`,
                }}
                fixedHeader
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <ProCard direction="column">
                    {getPVCTabsContent(activeTab, pvc)}
                </ProCard>
            </PageContainer>
        )
    }
}

function getPVCTabs(pvc: PVC): any {
    return [
        {
            key: '1',
            label: 'Yaml',
        },
        {
            key: '2',
            label: '事件',
        },
        {
            key: '3',
            label: 'PVC',
        },
        {
            key: '4',
            label: 'MountPod',
        },
    ]
}

function getPVTabsContent(activeTab: string, pvc: PVC, mountPod: Pod | undefined): any {
    let content: any
    switch (activeTab) {
        case "1":
            content = <div>
                <pre><code>{jsyaml.dump(pvc)}</code></pre>
            </div>
            break
        case "2":
            content = <div>
                <pre><code>pv event</code></pre>
            </div>
            break
        case '3':
            content = <div>
                <pre><code>pvc</code></pre>
            </div>
            break
        case "4":
            if (mountPod !== undefined) {
                content = <div>
                    <Link to={`/pod/${mountPod.metadata?.namespace}/${mountPod.metadata?.name}`}>
                        {mountPod.metadata?.name}
                    </Link>
                </div>
            }
    }
    return content
}

export default DetailedPVC;
