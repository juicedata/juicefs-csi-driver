import {defineConfig} from '@umijs/max';

export default defineConfig({
    antd: {},
    access: {},
    model: {},
    initialState: {},
    request: {},
    layout: {
        title: 'JuiceFS',
    },
    routes: [
        {
            path: '/',
            redirect: '/pods',
        },
        {
            name: '应用 Pod',
            path: '/pods',
            component: './AppPodTable',
        },
        {
            name: '系统 Pod',
            path: '/syspods',
            component: './SystemPodTable',
        },
        {
            name: 'PV',
            path: '/pvs',
            component: './PVTable',
        },
        {
            name: 'PVC',
            path: '/pvcs',
            component: './PVCTable',
        },
        {
            name: 'StorageClass',
            path: '/storageclasses',
            component: './SCTable',
        },
        {
            path: '/pod/:namespace/:podName',
            component: './DetailedPod',
        },
        {
            path: '/pv/:pvName',
            component: './DetailedPV',
        },
        {
            path: '/pvc/:namespace/:pvName',
            component: './DetailedPVC',
        },
        {
            path: '/storageclass/:scName',
            component: './DetailedSC',
        },
    ],
    npmClient: 'yarn',
});

