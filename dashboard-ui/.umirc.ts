import { defineConfig } from '@umijs/max';

export default defineConfig({
  antd: {},
  access: {},
  model: {},
  initialState: {},
  request: {},
  layout: {
    title: 'JuiceFS',
    locale: true,
  },
  favicons: ['https://static1.juicefs.com/images/favicon.d6e4afae8198.ico'],
  base: '/app/',
  publicPath: '/app/',
  routes: [
    {
      path: '/',
      redirect: '/pods',
    },
    {
      title: 'appPodTable',
      path: '/pods',
      component: './AppPodTable',
      icon: '/app/pod-256.png',
    },
    {
      title: 'sysPodTable',
      path: '/syspods',
      component: './SystemPodTable',
      icon: '/app/ds-256.png',
    },
    {
      name: 'PV',
      path: '/pvs',
      component: './PVTable',
      icon: '/app/pv-256.png',
    },
    {
      name: 'PVC',
      path: '/pvcs',
      component: './PVCTable',
      icon: '/app/pvc-256.png',
    },
    {
      name: 'StorageClass',
      path: '/storageclasses',
      component: './SCTable',
      icon: '/app/sc-256.png',
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
  locale: {
    antd: true,
    baseNavigator: true,
    title: true,
    // useLocalStorage: true,
    default: 'en-US',
    baseSeparator: '-',
  },
});
