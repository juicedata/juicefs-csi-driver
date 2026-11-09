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
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import {
  Alert,
  Button,
  Drawer,
  Form,
  Input,
  message,
  Modal,
  Select,
  Space,
  Switch,
  Tag,
} from 'antd'
import { Secret } from 'kubernetes-types/core/v1'
import { ObjectMeta } from 'kubernetes-types/meta/v1'
import { omit } from 'lodash'
import { useIntl } from 'react-intl'
import useSWR from 'swr'

import { apiFetch } from '@/utils'
import { k8sResourceNameRe } from '@/utils/k8s-validation'

type SecretSummary = {
  metadata: ObjectMeta & { name: string; namespace: string }
  type: string
  keys: string[]
}
type Entry = { key: string; value: string }
type Values = {
  name: string
  namespace: string
  entries: Entry[]
  immutable?: boolean
}

const decode = (value: string) =>
  new TextDecoder().decode(
    Uint8Array.from(atob(value), (char) => char.charCodeAt(0)),
  )

function SecretDrawer({
  initial,
  onClose,
  onSaved,
}: {
  initial?: Secret
  onClose: () => void
  onSaved: () => Promise<void>
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const [form] = Form.useForm<Values>()
  const [saving, setSaving] = useState(false)
  const originalEntries: Entry[] = Object.entries(initial?.data || {}).map(
    ([key, value]) => ({ key, value: decode(value) }),
  )
  const required = [
    { required: true, whitespace: true, message: t('warmupRequired') },
  ]
  const save = async (values: Values) => {
    const keys = values.entries.map((entry) => entry.key)
    if (new Set(keys).size !== keys.length) {
      message.error(t('secretDuplicateKey'))
      return
    }
    const data = { ...initial?.data }
    const stringData: Record<string, string> = {}
    for (const key of Object.keys(data))
      if (!keys.includes(key)) delete data[key]
    for (const entry of values.entries) {
      const original = originalEntries.find((item) => item.key === entry.key)
      if (!original || original.value !== entry.value)
        stringData[entry.key] = entry.value ?? ''
    }
    const body = {
      ...omit(initial || {}, ['metadata.managedFields']),
      apiVersion: 'v1',
      kind: 'Secret',
      metadata: {
        ...omit(initial?.metadata || {}, ['managedFields']),
        name: values.name.trim(),
        namespace: values.namespace.trim(),
      },
      type: initial?.type || 'Opaque',
      data,
      stringData,
      ...(initial ? {} : { immutable: values.immutable || undefined }),
    }
    setSaving(true)
    try {
      const path = initial
        ? `/api/v1/secret/${encodeURIComponent(values.namespace)}/${encodeURIComponent(values.name)}`
        : '/api/v1/secrets'
      await apiFetch(path, {
        method: initial ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      message.success(t('storageSaved'))
      await onSaved()
      onClose()
    } catch (err) {
      const text = (err as Error).message
      try {
        message.error((JSON.parse(text) as { error: string }).error || text)
      } catch {
        message.error(text)
      }
    } finally {
      setSaving(false)
    }
  }
  return (
    <Drawer
      title={t(initial ? 'secretEdit' : 'secretCreate')}
      open
      onClose={onClose}
      width="min(720px, 100vw)"
      destroyOnClose
      footer={
        <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Button onClick={onClose}>{t('warmupCancel')}</Button>
          <Button type="primary" loading={saving} onClick={() => form.submit()}>
            {t(initial ? 'save' : 'storageCreate')}
          </Button>
        </Space>
      }
    >
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          name: initial?.metadata?.name,
          namespace: initial?.metadata?.namespace || 'default',
          entries: initial
            ? originalEntries
            : [
                { key: 'name', value: '' },
                { key: 'token', value: '' },
              ],
          immutable: initial?.immutable,
        }}
        onFinish={(values: Values) => void save(values)}
      >
        <Form.Item
          name="name"
          label={t('name')}
          rules={[
            ...required,
            {
              pattern: k8sResourceNameRe,
              max: 253,
              message: t('resourceNameInvalid'),
            },
          ]}
        >
          <Input disabled={!!initial} />
        </Form.Item>
        <Form.Item name="namespace" label={t('namespace')} rules={required}>
          <Input disabled={!!initial} />
        </Form.Item>
        {initial?.immutable && (
          <Alert
            type="info"
            showIcon
            message={t('secretImmutableHint')}
            style={{ marginBottom: 16 }}
          />
        )}
        <Form.Item label={t('secretData')} extra={t('secretDataHint')}>
          <Form.List name="entries">
            {(fields, { add, remove }) => (
              <>
                {fields.map((field) => (
                  <Space
                    key={field.key}
                    align="start"
                    wrap
                    style={{ display: 'flex', marginBottom: 8 }}
                  >
                    <Form.Item
                      name={[field.name, 'key']}
                      rules={[
                        ...required,
                        {
                          pattern: /^[A-Za-z0-9._-]+$/,
                          max: 253,
                          message: t('secretKeyInvalid'),
                        },
                      ]}
                      style={{ marginBottom: 0 }}
                    >
                      <Input
                        placeholder="Key"
                        disabled={!!initial?.immutable}
                      />
                    </Form.Item>
                    <Form.Item
                      name={[field.name, 'value']}
                      style={{ marginBottom: 0 }}
                    >
                      <Input.Password
                        placeholder="Value"
                        disabled={!!initial?.immutable}
                      />
                    </Form.Item>
                    <Button
                      type="text"
                      icon={<MinusCircleOutlined />}
                      aria-label={t('secretRemoveKey')}
                      disabled={!!initial?.immutable}
                      onClick={() => remove(field.name)}
                    />
                  </Space>
                ))}
                <Button
                  type="dashed"
                  icon={<PlusOutlined />}
                  disabled={!!initial?.immutable}
                  onClick={() => add({ key: '', value: '' })}
                >
                  {t('secretAddKey')}
                </Button>
              </>
            )}
          </Form.List>
        </Form.Item>
        {!initial && (
          <Form.Item
            name="immutable"
            label={t('secretImmutable')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        )}
      </Form>
    </Drawer>
  )
}

export default function SecretList() {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const { data, error, isLoading, mutate } =
    useSWR<SecretSummary[]>('/api/v1/secrets')
  const [drawer, setDrawer] = useState<{ initial?: Secret }>()
  const [loading, setLoading] = useState<string>()
  const [nameFilter, setNameFilter] = useState('')
  const [namespaceFilter, setNamespaceFilter] = useState('')
  const [originFilter, setOriginFilter] = useState<'csi' | 'user'>()
  const [modal, contextHolder] = Modal.useModal()
  const edit = async (row: SecretSummary) => {
    const key = `${row.metadata.namespace}/${row.metadata.name}`
    setLoading(key)
    try {
      const initial = await apiFetch<Secret>(
        `/api/v1/secret/${encodeURIComponent(row.metadata.namespace)}/${encodeURIComponent(row.metadata.name)}`,
      )
      setDrawer({ initial })
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setLoading(undefined)
    }
  }
  const remove = (row: SecretSummary) =>
    modal.confirm({
      title: t('storageDeleteConfirm'),
      content: (
        <>
          {`Secret: ${row.metadata.namespace}/${row.metadata.name}`}
          <br />
          {t('secretDeleteRisk')}
        </>
      ),
      okText: t('warmupDelete'),
      cancelText: t('warmupCancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await apiFetch(
            `/api/v1/secret/${encodeURIComponent(row.metadata.namespace)}/${encodeURIComponent(row.metadata.name)}`,
            { method: 'DELETE' },
          )
          message.success(t('storageDeleted'))
          await mutate()
        } catch (err) {
          message.error((err as Error).message)
          throw err
        }
      },
    })
  const columns: ProColumns<SecretSummary>[] = [
    {
      title: t('name'),
      dataIndex: ['metadata', 'name'],
      sorter: (a, b) => a.metadata.name.localeCompare(b.metadata.name),
    },
    {
      title: t('namespace'),
      dataIndex: ['metadata', 'namespace'],
      sorter: (a, b) =>
        a.metadata.namespace.localeCompare(b.metadata.namespace),
    },
    {
      title: t('secretOrigin'),
      render: (_, row) => {
        const managed = Object.prototype.hasOwnProperty.call(
          row.metadata.labels || {},
          'juicefs/secret',
        )
        return (
          <Tag color={managed ? 'blue' : undefined}>
            {t(managed ? 'secretOriginCsi' : 'secretOriginUser')}
          </Tag>
        )
      },
    },
    { title: t('secretData'), render: (_, row) => row.keys.join(', ') },
    {
      title: t('createAt'),
      render: (_, row) =>
        row.metadata.creationTimestamp
          ? new Date(row.metadata.creationTimestamp).toLocaleString()
          : '-',
    },
    {
      title: t('action'),
      render: (_, row) => (
        <Space>
          <Button
            size="small"
            loading={
              loading === `${row.metadata.namespace}/${row.metadata.name}`
            }
            onClick={() => void edit(row)}
          >
            {t('edit')}
          </Button>
          <Button size="small" danger onClick={() => remove(row)}>
            {t('warmupDelete')}
          </Button>
        </Space>
      ),
    },
  ]
  return (
    <PageContainer header={{ title: t('secretManagement') }}>
      {contextHolder}
      {error && (
        <Alert
          type="error"
          showIcon
          message={t('secretListError')}
          description={String(error)}
        />
      )}
      <ProTable<SecretSummary>
        rowKey={(row) =>
          row.metadata.uid || `${row.metadata.namespace}/${row.metadata.name}`
        }
        loading={isLoading}
        dataSource={(data || []).filter((secret) => {
          const managed = Object.prototype.hasOwnProperty.call(
            secret.metadata.labels || {},
            'juicefs/secret',
          )
          return (
            secret.metadata.name
              .toLowerCase()
              .includes(nameFilter.trim().toLowerCase()) &&
            secret.metadata.namespace
              .toLowerCase()
              .includes(namespaceFilter.trim().toLowerCase()) &&
            (!originFilter || (originFilter === 'csi') === managed)
          )
        })}
        columns={columns}
        search={false}
        toolbar={{
          search: (
            <Space>
              <Input
                allowClear
                placeholder={t('name')}
                value={nameFilter}
                onChange={(event) => setNameFilter(event.target.value)}
              />
              <Input
                allowClear
                placeholder={t('namespace')}
                value={namespaceFilter}
                onChange={(event) => setNamespaceFilter(event.target.value)}
              />
              <Select
                allowClear
                placeholder={t('secretOrigin')}
                value={originFilter}
                onChange={setOriginFilter}
                style={{ width: 140 }}
                options={[
                  { value: 'csi', label: t('secretOriginCsi') },
                  { value: 'user', label: t('secretOriginUser') },
                ]}
              />
            </Space>
          ),
          actions: [
            <Button key="create" type="primary" onClick={() => setDrawer({})}>
              {t('secretCreate')}
            </Button>,
          ],
          settings: undefined,
        }}
      />
      {drawer && (
        <SecretDrawer
          initial={drawer.initial}
          onClose={() => setDrawer(undefined)}
          onSaved={async () => {
            await mutate()
          }}
        />
      )}
    </PageContainer>
  )
}
