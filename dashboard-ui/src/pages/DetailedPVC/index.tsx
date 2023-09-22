import {PageContainer, PageLoading} from '@ant-design/pro-components';
import React, {useEffect, useState} from 'react';
import {useMatch} from '@umijs/max';
import {Pod} from '@/services/pod';
import {getMountPodOfPVC, getPVC, PVC} from '@/services/pv';
import {Link} from 'umi';
import * as jsyaml from "js-yaml";
import {TabsProps} from "antd";

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
    const [pvc, setPV] = useState<PVC>()
    const [mountpod, setMountPod] = useState<Pod>()
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

    if (!pvc) {
        return <PageLoading/>
    } else {
        const tabList: TabsProps['items'] = getPVCTabs(pvc)
        return (
            <PageContainer
                header={{
                    title: `持久卷: ${pvc?.metadata?.name}`,
                }}
                fixedHeader
                tabList={tabList}
                onTabChange={handleTabChange}
            >
                <h3> Mount Pod:&nbsp;
                    <Link to={`/pod/${mountpod?.metadata?.namespace}/${mountpod?.metadata?.name}`}>
                        {mountpod?.metadata?.name}
                    </Link>
                </h3>
                TODO...
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
