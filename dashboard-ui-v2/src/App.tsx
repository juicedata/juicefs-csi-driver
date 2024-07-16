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
import { ConfigProvider } from 'antd'
import * as monaco from 'monaco-editor'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { SWRConfig } from 'swr'

import { Layout, ResourceDetail, ResourceList } from '@/components'
import { getBasePath, getHost } from '@/utils'

async function fetcher<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${getHost()}${getBasePath()}${url}`, init)
  if (!res.ok) {
    throw new Error('Failed to fetch')
  }
  return res.json()
}

self.MonacoEnvironment = {
  getWorker() {
    return new editorWorker()
  },
}

loader.config({ monaco })

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
        },
        components: {
          Layout: {
            headerBg: '#ffffff',
          },
        },
      }}
    >
      <BrowserRouter basename={getBasePath()}>
        <Layout>
          <Routes>
            <Route path="/" element={<Navigate replace to="/pods" />} />
            <Route path="/:resources" element={<ResourceList />} />
            <Route
              path="/:resources/:namespace/:name"
              element={<ResourceDetail />}
            />
            <Route path="/:resources/:name" element={<ResourceDetail />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </ConfigProvider>
  </SWRConfig>
)

export default App
