import { PageContainer } from '@ant-design/pro-components';
import React, { useState, useEffect, Suspense } from 'react';
import { useMatch } from '@umijs/max';
import { PV, getPV } from '@/services/pv';
import { PageLoading } from '@ant-design/pro-components';

const DetailedPV: React.FC<unknown> = () => {
    const routeData = useMatch('/pv/:namespace/:name')
    const namespace = routeData?.params?.namespace
    const name = routeData?.params?.name
    if (!namespace || !name) {
        return (
            <PageContainer
            header={{
                title: 'PV 不存在',
            }}
            >
            </PageContainer>
        )
    }
    const [ pv, setPV ] = useState<PV>()
    useEffect(() => {
        getPV(namespace, name).then(setPV)
    }, [setPV])
    if (!pv) {
        return <PageLoading />
    } else {
        return (
            <PageContainer
            header={{
                title: `持久卷: ${pv?.metadata?.name}`,
            }}
            >
                TODO...
            </PageContainer>
        )
    }
    
}

export default DetailedPV;
