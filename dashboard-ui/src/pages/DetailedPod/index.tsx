import { PageContainer } from '@ant-design/pro-components';
import React, { useState, useEffect, Suspense } from 'react';
import { useMatch } from '@umijs/max';
import { Pod, getPod } from '@/services/pod';
import { PageLoading } from '@ant-design/pro-components';

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
    const [ pod, setPod ] = useState<Pod>()
    useEffect(() => {
        getPod(namespace, name).then(setPod)
    }, [setPod])
    if (!pod) {
        return <PageLoading />
    } else {
        return (
            <PageContainer
            header={{
                title: pod?.metadata?.name,
            }}
            >
                TODO...
            </PageContainer>
        )
    }
    
}

export default DetailedPod;
