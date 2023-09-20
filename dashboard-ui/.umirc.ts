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
  ],
  npmClient: 'yarn',
});

