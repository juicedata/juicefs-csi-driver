import { ReactComponent as Logo } from './assets/logo.svg'
import { RuntimeConfig } from 'umi';

// 运行时配置

// 全局初始化数据配置，用于 Layout 用户信息和权限初始化
// 更多信息见文档：https://umijs.org/docs/api/runtime-config#getinitialstate
export async function getInitialState(): Promise<{ name: string }> {
  return { name: 'JuiceFS CSI Driver' };
}

export const layout: RuntimeConfig['layout'] = () => {
  return {
    layout: "side",
    title: "JuiceFS CSI",
    logo: <Logo />,
    menu: {
      locale: false,
    },
    rightContentRender: false,
    colorPrimary: '#0ABD59',
  };
};