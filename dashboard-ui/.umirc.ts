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
      path: '/pod/:namespace/:podName',
      component: './DetailedPod',
    },
    {
      path: '/pv/:namespace/:podName',
      component: './DetailedPV',
    },
  ],
  npmClient: 'yarn',
});

