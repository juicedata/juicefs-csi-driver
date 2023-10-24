import { defineConfig } from '@umijs/max';

export default defineConfig({
    antd: {},
    access: {},
    model: {},
    initialState: {},
    request: {},
    layout: {
        title: 'JuiceFS',
    },
    favicons: [
        'https://static1.juicefs.com/images/favicon.d6e4afae8198.ico',
    ],
    routes: [
        {
            path: '/',
            redirect: '/pods',
        },
        {
            name: '应用 Pod',
            path: '/pods',
            component: './AppPodTable',
            icon: '/pod-256.png'
        },
        {
            name: '系统 Pod',
            path: '/syspods',
            component: './SystemPodTable',
            icon: '/ds-256.png'
        },
        {
            name: 'PV',
            path: '/pvs',
            component: './PVTable',
            icon: '/pv-256.png'
        },
        {
            name: 'PVC',
            path: '/pvcs',
            component: './PVCTable',
            icon: '/pvc-256.png'
        },
        {
            name: 'StorageClass',
            path: '/storageclasses',
            component: './SCTable',
            icon: '/sc-256.png'
        },
        {
            path: '/pod/:namespace/:podName',
            component: './DetailedPod',
        },
        {
            path: '/apppod/:namespace/:podName',
            component: './DetailedPod',
        },
        {
            path: '/mountpod/:namespace/:podName',
            component: './DetailedPod',
        },
        {
            path: '/pv/:pvName',
            component: './DetailedPV',
        },
        {
            path: '/pvc/:namespace/:name',
            component: './DetailedPVC',
        },
        {
            path: '/storageclass/:scName',
            component: './DetailedSC',
        },
    ],
    npmClient: 'yarn',
});

