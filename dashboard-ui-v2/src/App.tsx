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
            <Route path="/:resources/:namespace/:name" element={<ResourceDetail />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </ConfigProvider>
  </SWRConfig>
)

export default App
