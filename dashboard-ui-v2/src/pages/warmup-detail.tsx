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
import { Warmup } from '@/types/k8s'
import { apiFetch } from '@/utils'

type WarmupPod = {
  name: string
  job: string
  phase: string
  creationTimestamp: string
}

export default function WarmupDetail({
  namespace,
  name,
}: {
  namespace?: string
  name?: string
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const navigate = useNavigate()
  const base = `/api/v1/warmups/${encodeURIComponent(namespace || '')}/${encodeURIComponent(name || '')}`
  const { data, error, isLoading } = useSWR<Warmup>(
    namespace && name ? base : null,
    { refreshInterval: 10000 },
  )
  const {
    data: pods,
    error: podsError,
    isLoading: podsLoading,
  } = useSWR<WarmupPod[]>(namespace && name ? `${base}/pods` : null, {
    refreshInterval: 10000,
  })
  const [yamlOpen, setYamlOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const target = data?.spec.targetsFrom
  const targets =
    target?.files?.join('\n') ||
    (target?.configMap
      ? `${target.configMap.name}/${target.configMap.key}`
      : target?.filePath) ||
    data?.spec.targets?.join('\n') ||
    t('warmupTarget_all')
  const date = (value?: string) =>
    value ? new Date(value).toLocaleString() : '-'

  const deleteWarmup = async () => {
    setDeleting(true)
    try {
      await apiFetch(base, { method: 'DELETE' })
      message.success(t('warmupDeleted'))
      navigate('/warmups')
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
      {error && (
        <Alert type="error" showIcon message={t('warmupDetailError')} />
      )}
      {data && (
        <>
          <ProCard
            title={t('basic')}
            extra={
              <Space>
                <Button onClick={() => setYamlOpen(true)}>YAML</Button>
                <Popconfirm
                  title={t('warmupDeleteConfirm')}
                  onConfirm={deleteWarmup}
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
            <ProDescriptions<Warmup>
              column={2}
              dataSource={data}
              columns={[
                { title: t('name'), dataIndex: ['metadata', 'name'] },
                { title: t('namespace'), dataIndex: ['metadata', 'namespace'] },
                {
                  title: t('cacheGroup'),
                  dataIndex: ['spec', 'cacheGroupName'],
                },
                { title: t('phase'), dataIndex: ['status', 'phase'] },
                {
                  title: t('warmupPolicy'),
                  render: () =>
                    data.spec.policy?.type === 'Cron'
                      ? `${t('warmupCron')} (${data.spec.policy.cron?.schedule || '-'})`
                      : t('warmupOnce'),
                },
                {
                  title: t('warmupProgress'),
                  dataIndex: ['status', 'progress'],
                },
                {
                  title: t('warmupScanned'),
                  dataIndex: ['status', 'stats', 'scannedData'],
                },
                {
                  title: t('warmupCompleted'),
                  dataIndex: ['status', 'stats', 'completedData'],
                },
                {
                  title: t('warmupFailed'),
                  dataIndex: ['status', 'stats', 'failedData'],
                },
                {
                  title: t('warmupSpeed'),
                  dataIndex: ['status', 'stats', 'speed'],
                },
                {
                  title: t('warmupDuration'),
                  dataIndex: ['status', 'duration'],
                },
                {
                  title: t('createAt'),
                  render: () => date(data.metadata.creationTimestamp),
                },
                {
                  title: t('warmupLastSchedule'),
                  render: () => date(data.status?.lastScheduleTime),
                },
                {
                  title: t('warmupLastComplete'),
                  render: () => date(data.status?.lastCompleteTime),
                },
                {
                  title: t('warmupTargetType'),
                  span: 2,
                  render: () => (
                    <span
                      style={{
                        whiteSpace: 'pre-wrap',
                        overflowWrap: 'anywhere',
                      }}
                    >
                      {targets}
                    </span>
                  ),
                },
                {
                  title: t('warmupOptions'),
                  span: 2,
                  render: () => data.spec.options?.join(', ') || '-',
                },
                {
                  title: t('mountOptions'),
                  span: 2,
                  render: () => data.spec.mountOptions?.join(', ') || '-',
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
              <Alert type="error" showIcon message={t('warmupPodsError')} />
            )}
            <Table<WarmupPod>
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
                { title: 'Job', dataIndex: 'job' },
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
