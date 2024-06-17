/**
 * Copyright 2024 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { ReactNode } from 'react'
import { Layout as AntdLayout, Menu, MenuProps } from 'antd'
import { IntlProvider } from 'react-intl'
import { Link } from 'react-router-dom'

import { DSIcon, LOGOIcon, PODIcon, PVCIcon, PVIcon, SCIcon } from '@/icons'
import cn from '@/locales/zh-CN'

const { Header, Sider, Content } = AntdLayout

const items: MenuProps['items'] = [
  {
    icon: <PODIcon />,
    label: <Link to="/pods">应用 Pod</Link>,
    key: '1',
  },
  {
    icon: <DSIcon />,
    label: <Link to="/syspods">系统 Pod</Link>,
    key: '2',
  },
  {
    icon: <PVIcon />,
    label: <Link to="/pvs">PV</Link>,
    key: '3',
  },
  {
    icon: <PVCIcon />,
    label: <Link to="/pvcs">PVC</Link>,
    key: '4',
  },
  {
    icon: <SCIcon />,
    label: <Link to="/storageclass">Storage Class</Link>,
    key: '5',
  },
]

export default function Layout(props: { children: ReactNode }) {
  return (
    <AntdLayout style={{ height: '100vh', width: '100%' }}>
      <Header
        style={{
          position: 'sticky',
          top: 0,
          zIndex: 1,
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          padding: '0 30px',
        }}
      >
        <LOGOIcon width={30} />
        <h2>JuiceFS CSI</h2>
      </Header>
      <IntlProvider messages={cn} locale="en" defaultLocale="en">
        <AntdLayout hasSider>
          <Sider
            style={{
              overflow: 'auto',
              height: '100vh',
              position: 'fixed',
            }}
          >
            <Menu
              mode="inline"
              defaultSelectedKeys={['1']}
              defaultOpenKeys={['1']}
              style={{ height: '100%' }}
              items={items}
            />
          </Sider>
          <AntdLayout style={{ marginLeft: 200, padding: 4 }}>
            <Content>{props.children}</Content>
          </AntdLayout>
        </AntdLayout>
      </IntlProvider>
    </AntdLayout>
  )
}
