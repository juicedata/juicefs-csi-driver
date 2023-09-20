import { PageContainer } from '@ant-design/pro-components';
import React, { useState, useEffect, Suspense } from 'react';
import { useMatch } from '@umijs/max';
import { Pod } from '@/services/pod';
import { PV, getPV, getMountPodofPV } from '@/services/pv';
import { PageLoading } from '@ant-design/pro-components';
import { Link } from 'umi';

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
    const [ mountpod, setMountPod ] = useState<Pod>()
    useEffect(() => {
        getPV(namespace, name)
            .then(setPV)
            .then(() => getMountPodofPV(namespace, name))
            .then(setMountPod)
    }, [setPV, setMountPod])
    if (!pv) {
        return <PageLoading />
    } else {
        return (
            <PageContainer
            header={{
                title: `持久卷: ${pv?.metadata?.name}`,
            }}
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

export default DetailedPV;
