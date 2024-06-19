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

import { ReactNode, useEffect, useState } from 'react'
import { QuestionCircleOutlined } from '@ant-design/icons'
import {
  Layout as AntdLayout,
  Button,
  ConfigProvider,
  Menu,
  MenuProps,
} from 'antd'
import enUS from 'antd/locale/en_US'
import zhCN from 'antd/locale/zh_CN'
import { FormattedMessage, IntlProvider } from 'react-intl'
import { Link } from 'react-router-dom'

import { DSIcon, PODIcon, PVCIcon, PVIcon, SCIcon } from '@/icons'
import en from '@/locales/en-US'
import cn from '@/locales/zh-CN'

const { Header, Sider, Content } = AntdLayout

const items: MenuProps['items'] = [
  {
    icon: <PODIcon />,
    label: (
      <Link to="/pods">
        <FormattedMessage id="appPodTable" />
      </Link>
    ),
    key: '1',
  },
  {
    icon: <DSIcon />,
    label: (
      <Link to="/syspods">
        <FormattedMessage id="sysPodTable" />
      </Link>
    ),
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
  const [locale, setLocale] = useState<string>()

  useEffect(() => {
    if (locale) {
      window.localStorage.setItem('locale', locale)
    }
  }, [locale])

  useEffect(() => {
    const locale = window.localStorage.getItem('locale')
    setLocale(locale ?? 'cn')
  }, [])

  return (
    <AntdLayout>
      <Header
        style={{
          position: 'fixed',
          top: 0,
          zIndex: 1,
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          fontSize: '14px',
          padding: '0 40px',
        }}
      >
        <h2>JuiceFS CSI</h2>
        <div style={{ marginLeft: 'auto', justifyContent: 'space-between' }}>
          <QuestionCircleOutlined
            width={20}
            onClick={() => {
              // open a new tab to the JuiceFS CSI documentation
              window.open(
                'https://juicefs.com/docs/csi/introduction/',
                '_blank',
              )
            }}
          />
          <Button
            style={{ marginLeft: '10px' }}
            // shape="round"
            size="small"
            onClick={() => {
              setLocale(locale === 'cn' ? 'en' : 'cn')
            }}
          >
            {(locale == 'cn' ? 'en' : 'cn').toUpperCase()}
          </Button>
        </div>
      </Header>
      <IntlProvider messages={locale == 'cn' ? cn : en} locale={locale ?? 'cn'}>
        <ConfigProvider locale={locale == 'cn' ? zhCN : enUS}>
          <AntdLayout hasSider>
            <Sider
              style={{
                overflow: 'auto',
                height: '100vh',
                position: 'fixed',
                marginTop: '64px',
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
          </AntdLayout>
          <AntdLayout style={{ marginLeft: 200, marginTop: '64px' }}>
            <Content>{props.children}</Content>
          </AntdLayout>
        </ConfigProvider>
      </IntlProvider>
    </AntdLayout>
  )
}
