/*
 * Copyright 2026 Juicedata Inc
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

import { useState } from 'react'
import {
  PageContainer,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components'
import { Alert, Button, message, Popconfirm, Space, Table } from 'antd'
import { omit } from 'lodash'
import { useIntl } from 'react-intl'
import { Link, useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import YAML from 'yaml'

import { YamlModal } from '@/components'
import { Sync } from '@/types/k8s'
import { apiFetch } from '@/utils'

type SyncPod = { name: string; phase: string; creationTimestamp: string }

export default function SyncDetail({
  namespace,
  name,
}: {
  namespace?: string
  name?: string
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const navigate = useNavigate()
  const base = `/api/v1/syncs/${encodeURIComponent(namespace || '')}/${encodeURIComponent(name || '')}`
  const { data, error, isLoading } = useSWR<Sync>(
    namespace && name ? base : null,
    { refreshInterval: 10000 },
  )
  const {
    data: pods,
    error: podsError,
    isLoading: podsLoading,
  } = useSWR<SyncPod[]>(namespace && name ? `${base}/pods` : null, {
    refreshInterval: 10000,
  })
  const [yamlOpen, setYamlOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const date = (value?: string) =>
    value ? new Date(value).toLocaleString() : '-'
  const sink = (value?: Sync['spec']['from']) =>
    value?.external?.uri ||
    (value?.juicefs
      ? `${value.juicefs.volumeName || ''}:${value.juicefs.path || '/'}`
      : '') ||
    (value?.juicefsCE
      ? `${value.juicefsCE.metaURL || ''}:${value.juicefsCE.path || '/'}`
      : '') ||
    '-'
  const deleteSync = async () => {
    setDeleting(true)
    try {
      await apiFetch(base, { method: 'DELETE' })
      message.success(t('syncDeleted'))
      navigate('/syncs')
    } catch (err) {
      const text = (err as Error).message
      try {
        message.error((JSON.parse(text) as { error: string }).error || text)
      } catch {
        message.error(text)
      }
    } finally {
      setDeleting(false)
    }
  }
  return (
    <PageContainer
      header={{ title: name, subTitle: namespace }}
      loading={isLoading}
    >
      {error && <Alert type="error" showIcon message={t('syncDetailError')} />}
      {data && (
        <>
          <ProCard
            title={t('basic')}
            extra={
              <Space>
                <Button onClick={() => setYamlOpen(true)}>YAML</Button>
                <Popconfirm
                  title={t('syncDeleteConfirm')}
                  onConfirm={deleteSync}
                  okText={t('warmupDelete')}
                  cancelText={t('warmupCancel')}
                >
                  <Button danger loading={deleting}>
                    {t('warmupDelete')}
                  </Button>
                </Popconfirm>
              </Space>
            }
          >
            <ProDescriptions<Sync>
              column={2}
              dataSource={data}
              columns={[
                { title: t('name'), dataIndex: ['metadata', 'name'] },
                { title: t('namespace'), dataIndex: ['metadata', 'namespace'] },
                { title: t('phase'), dataIndex: ['status', 'phase'] },
                {
                  title: t('warmupProgress'),
                  dataIndex: ['status', 'progress'],
                },
                { title: t('syncImage'), dataIndex: ['spec', 'image'] },
                { title: t('syncReplicas'), dataIndex: ['spec', 'replicas'] },
                { title: t('syncFrom'), render: () => sink(data.spec.from) },
                { title: t('syncTo'), render: () => sink(data.spec.to) },
                {
                  title: t('createAt'),
                  render: () => date(data.metadata.creationTimestamp),
                },
                {
                  title: t('syncOptions'),
                  render: () => data.spec.options?.join(', ') || '-',
                },
                {
                  title: t('syncReason'),
                  span: 2,
                  render: () => data.status?.reason || '-',
                },
                {
                  title: t('syncFinishLog'),
                  span: 2,
                  render: () => (
                    <pre style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', margin: 0 }}>
                      <code>{data.status?.finishLog || '-'}</code>
                    </pre>
                  ),
                },
              ]}
            />
          </ProCard>
          <YamlModal
            isOpen={yamlOpen}
            onClose={() => setYamlOpen(false)}
            content={YAML.stringify(omit(data, ['metadata.managedFields']))}
          />
          <ProCard title={t('warmupPods')} style={{ marginTop: 16 }}>
            {podsError && (
              <Alert type="error" showIcon message={t('syncPodsError')} />
            )}
            <Table<SyncPod>
              rowKey="name"
              dataSource={pods || []}
              loading={podsLoading}
              pagination={false}
              columns={[
                {
                  title: t('name'),
                  dataIndex: 'name',
                  render: (podName: string) => (
                    <Link
                      to={`/syspods/${encodeURIComponent(namespace || '')}/${encodeURIComponent(podName)}`}
                    >
                      {podName}
                    </Link>
                  ),
                },
                { title: t('status'), dataIndex: 'phase' },
                {
                  title: t('createAt'),
                  render: (_, pod) => date(pod.creationTimestamp),
                },
              ]}
              locale={{ emptyText: t('warmupNoPods') }}
            />
          </ProCard>
        </>
      )}
    </PageContainer>
  )
}
