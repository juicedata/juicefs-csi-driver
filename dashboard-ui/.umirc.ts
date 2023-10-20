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
            icon: 'https://github.com/kubernetes/community/blob/master/icons/svg/resources/labeled/pod.svg'
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

