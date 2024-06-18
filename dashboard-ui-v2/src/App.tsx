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

import { loader } from '@monaco-editor/react'
import { Button, ConfigProvider } from 'antd'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { SWRConfig } from 'swr'

import { Layout, ResourceDetail, ResourceList } from '@/components'

async function fetcher<T>(url: string, init?: RequestInit): Promise<T> {
  const protocol = window.location.protocol === 'https:' ? 'https' : 'http'
  const host = import.meta.env.VITE_HOST ?? window.location.host
  const res = await fetch(`${protocol}://${host}${url}`, init)
  if (!res.ok) {
    throw new Error('Failed to fetch')
  }
  return res.json()
}

loader.config({
  paths: { vs: 'https://unpkg.com/monaco-editor@0.43.0/min/vs' },
})

const App = () => (
  <SWRConfig
    value={{
      fetcher,
    }}
  >
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: '#00b96b',
          borderRadius: 3,
          colorBgContainer: '#f5f5f5',
        },
        components: {
          Layout: {
            headerBg: '#ffffff',
          },
        },
      }}
    >
      <BrowserRouter>
        <Layout>
          <Routes>
            <Route path="/" element={<Button> Home </Button>} />
            <Route path="/:resources" element={<ResourceList />} />
            <Route
              path="/:resources/:namespace/:name"
              element={<ResourceDetail />}
            />
          </Routes>
        </Layout>
      </BrowserRouter>
    </ConfigProvider>
  </SWRConfig>
)

export default App
