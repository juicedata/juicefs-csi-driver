/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import { ReactComponent as Logo } from './assets/logo.svg'
import { RuntimeConfig } from 'umi';

// 运行时配置

// 全局初始化数据配置，用于 Layout 用户信息和权限初始化
// 更多信息见文档：https://umijs.org/docs/api/runtime-config#getinitialstate
export async function getInitialState(): Promise<{}> {
  return {};
}

export const layout: RuntimeConfig['layout'] = () => {
  return {
    // navTheme: "realDark",
    layout: "mix",
    title: "JuiceFS CSI",
    logo: false,
    menu: {
      locale: false,
    },
    rightContentRender: false,
    colorPrimary: '#0ABD59',
  };
};
