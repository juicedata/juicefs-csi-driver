import { defineConfig } from '@umijs/max'

const ROOT_PATH = '/dashboard'

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
  base: `${ROOT_PATH}/app/`,
  publicPath: `${ROOT_PATH}/app/`,
  routes: [
    {
      path: '/',
      redirect: '/pods',
    },
    {
      title: 'appPodTable',
      path: '/pods',
      component: './AppPodTable',
      icon: `${ROOT_PATH}/app/pod-256.png`,
    },
    {
      title: 'sysPodTable',
      path: '/syspods',
      component: './SystemPodTable',
      icon: `${ROOT_PATH}/app/ds-256.png`,
    },
    {
      name: 'PV',
      path: '/pvs',
      component: './PVTable',
      icon: `${ROOT_PATH}/app/pv-256.png`,
    },
    {
      name: 'PVC',
      path: '/pvcs',
      component: './PVCTable',
      icon: `${ROOT_PATH}/app/pvc-256.png`,
    },
    {
      name: 'StorageClass',
      path: '/storageclasses',
      component: './SCTable',
      icon: `${ROOT_PATH}/app/sc-256.png`,
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
  define: {
    HOST: `${
      process.env.NODE_ENV === 'development' ? 'http://localhost:8088' : ''
    }${ROOT_PATH}`,
  },
})
