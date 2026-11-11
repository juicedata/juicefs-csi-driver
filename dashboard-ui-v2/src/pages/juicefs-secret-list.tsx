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
import Editor from '@monaco-editor/react'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import {
  Alert,
  Button,
  Drawer,
  Form,
  Input,
  message,
  Modal,
  Radio,
  Select,
  Space,
  Table,
  Tag,
} from 'antd'
import { Secret } from 'kubernetes-types/core/v1'
import { ObjectMeta } from 'kubernetes-types/meta/v1'
import { omit } from 'lodash'
import { useIntl } from 'react-intl'
import useSWR from 'swr'
import YAML from 'yaml'

import { apiFetch } from '@/utils'
import { envVarNameRe, k8sResourceNameRe } from '@/utils/k8s-validation'

type SecretSummary = {
  metadata: ObjectMeta & { name: string; namespace: string }
  type: string
  keys: string[]
}
type Entry = { key: string; value: string }
type Values = {
  name: string
  namespace: string
  edition: 'enterprise' | 'community'
  entries: Entry[]
}

const secretKeys = [
  'name',
  'token',
  'metaurl',
  'storage',
  'bucket',
  'access-key',
  'secret-key',
  'envs',
  'format-options',
]

const decode = (value: string) =>
  new TextDecoder().decode(
    Uint8Array.from(atob(value), (char) => char.charCodeAt(0)),
  )

function secretValues(secret: Secret): Values {
  const entries = Object.entries(secret.data || {}).map(([key, value]) => ({
    key,
    value: decode(value),
  }))
  for (const [key, value] of Object.entries(secret.stringData || {})) {
    const index = entries.findIndex((entry) => entry.key === key)
    if (index < 0) entries.push({ key, value })
    else entries[index].value = value
  }
  return {
    name: secret.metadata?.name || '',
    namespace: secret.metadata?.namespace || '',
    edition: entries.some((entry) => entry.key === 'metaurl')
      ? 'community'
      : 'enterprise',
    entries: [
      ...secretKeys.map((key) => ({
        key,
        value: entries.find((entry) => entry.key === key)?.value || '',
      })),
      ...entries.filter((entry) => !secretKeys.includes(entry.key)),
    ],
  }
}

function parseSecretEnvs(value: string): Entry[] {
  if (!value.trim()) return []
  const doc = YAML.parseDocument(value)
  if (doc.errors.length || !YAML.isMap(doc.contents)) throw new Error('invalid envs')
  const entries = Object.entries(doc.toJS() as Record<string, unknown>)
  if (entries.some(([key, item]) =>
    !envVarNameRe.test(key) ||
    !['string', 'number', 'boolean'].includes(typeof item),
  )) throw new Error('invalid envs')
  return entries.map(([key, item]) => ({ key, value: String(item) }))
}

function SecretEnvsInput({
  value = '',
  onChange,
  disabled,
  edition,
}: {
  value?: string
  onChange?: (value: string) => void
  disabled?: boolean
  edition: Values['edition']
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const [form] = Form.useForm<{ entries: Entry[] }>()
  const [open, setOpen] = useState(false)
  const [raw, setRaw] = useState<string>()

  const openEditor = () => {
    form.resetFields()
    try {
      form.setFieldsValue({ entries: parseSecretEnvs(value) })
      setRaw(undefined)
    } catch {
      setRaw(value)
    }
    setOpen(true)
  }
  const apply = async () => {
    if (raw !== undefined) {
      try {
        form.setFieldsValue({ entries: parseSecretEnvs(raw) })
        setRaw(undefined)
      } catch {
        message.error(t('secretEnvsInvalid'))
      }
      return
    }
    try {
      const { entries = [] } = await form.validateFields()
      if (new Set(entries.map((entry) => entry.key)).size !== entries.length) {
        message.error(t('secretEnvsDuplicate'))
        return
      }
      onChange?.(
        entries.length
          ? JSON.stringify(Object.fromEntries(entries.map(({ key, value }) => [key, value || ''])))
          : '',
      )
      setOpen(false)
    } catch {
      return
    }
  }
  return (
    <>
      <Button disabled={disabled} onClick={openEditor}>
        {t('secretConfigureEnvs')}
      </Button>
      <Modal
        open={open}
        title={t('envs')}
        width="min(800px, calc(100vw - 32px))"
        className="secret-envs-modal"
        onCancel={() => setOpen(false)}
        onOk={() => void apply()}
        okText={t('secretEnvsConfirm')}
        cancelText={t('warmupCancel')}
        forceRender
      >
        {edition === 'enterprise' && (
          <Alert
            type="info"
            showIcon
            message={t('secretEnvsBaseUrlHint')}
            style={{ marginBottom: 12 }}
          />
        )}
        {raw !== undefined && (
          <>
            <Alert type="error" showIcon message={t('secretEnvsInvalid')} />
            <Input.TextArea
              value={raw}
              onChange={(event) => setRaw(event.target.value)}
              rows={5}
              style={{ marginTop: 12 }}
            />
          </>
        )}
        <div style={{ display: raw === undefined ? undefined : 'none' }}>
          <Form form={form} layout="vertical">
            <Form.List name="entries">
              {(fields, { add, remove }) => (
                <>
                  {fields.map((field) => (
                    <div key={field.key} style={{ display: 'flex', gap: 8, alignItems: 'flex-start', marginBottom: 8 }}>
                      <Form.Item
                        name={[field.name, 'key']}
                        rules={[
                          { required: true, whitespace: true, message: t('warmupRequired') },
                          { pattern: envVarNameRe, message: t('secretEnvNameInvalid') },
                        ]}
                        style={{ marginBottom: 0, flex: 1, minWidth: 0 }}
                      >
                        <Input placeholder={t('cacheGroupEnvName')} />
                      </Form.Item>
                      <Form.Item name={[field.name, 'value']} style={{ marginBottom: 0, flex: 1, minWidth: 0 }}>
                        <Input.Password placeholder={t('cacheGroupEnvValue')} />
                      </Form.Item>
                      <Button
                        type="text"
                        icon={<MinusCircleOutlined />}
                        aria-label={t('cacheGroupRemoveEnv')}
                        onClick={() => remove(field.name)}
                      />
                    </div>
                  ))}
                  <Button type="dashed" icon={<PlusOutlined />} onClick={() => add({ key: '', value: '' })}>
                    {t('cacheGroupAddEnv')}
                  </Button>
                </>
              )}
            </Form.List>
          </Form>
        </div>
      </Modal>
    </>
  )
}

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
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [base, setBase] = useState<Secret>(() =>
    initial
      ? { ...omit(initial, ['metadata.managedFields']), apiVersion: 'v1', kind: 'Secret' }
      : { apiVersion: 'v1', kind: 'Secret', type: 'Opaque' },
  )
  const [yaml, setYaml] = useState(() => YAML.stringify(base))
  const edition = Form.useWatch('edition', form) || 'enterprise'
  const baseValues = secretValues(base)
  const originalEntries = baseValues.entries.filter((entry) =>
    Object.prototype.hasOwnProperty.call(base.data || {}, entry.key) ||
    Object.prototype.hasOwnProperty.call(base.stringData || {}, entry.key),
  )
  const initialEdition = baseValues.edition
  const isVisible = (key: string) =>
    edition === 'community'
      ? key !== 'token'
      : key !== 'metaurl' &&
        (!['storage', 'bucket'].includes(key) ||
          originalEntries.some((entry) => entry.key === key))
  const required = [
    { required: true, whitespace: true, message: t('warmupRequired') },
  ]
  const syncFormToYaml = (values: Values): Secret => {
    const doc = YAML.parseDocument(yaml)
    if (doc.errors.length) throw doc.errors[0]
    for (const key of ['metadata', 'stringData'])
      if (doc.has(key) && doc.get(key) === null) doc.delete(key)
    const allKeys = values.entries.map((entry) => entry.key)
    if (new Set(allKeys).size !== allKeys.length)
      throw new Error(t('secretDuplicateKey'))
    const entries = values.entries.filter((entry, index) => {
      if (index >= secretKeys.length) return true
      if (!isVisible(entry.key)) return false
      return (
        entry.value !== '' ||
        originalEntries.some((item) => item.key === entry.key)
      )
    })
    if (values.edition === initialEdition) {
      entries.push(...originalEntries.filter((entry) => !isVisible(entry.key)))
    }
    const keys = entries.map((entry) => entry.key)
    for (const field of ['data', 'stringData'] as const)
      for (const key of Object.keys(base[field] || {}))
        if (!keys.includes(key)) doc.deleteIn([field, key])
    for (const entry of entries) {
      const original = originalEntries.find((item) => item.key === entry.key)
      if (!original || original.value !== entry.value) {
        if (doc.hasIn(['data', entry.key])) doc.deleteIn(['data', entry.key])
        doc.setIn(['stringData', entry.key], entry.value ?? '')
      }
    }
    for (const key of ['name', 'namespace'] as const) {
      const value = values[key]?.trim()
      if (value) doc.setIn(['metadata', key], value)
      else if (doc.hasIn(['metadata', key])) doc.deleteIn(['metadata', key])
    }
    setYaml(doc.toString())
    return doc.toJS() as Secret
  }

  const parseSecret = (content: string): Secret => {
    const doc = YAML.parseDocument(content)
    if (doc.errors.length) throw doc.errors[0]
    const secret = doc.toJS() as Secret
    if (secret?.apiVersion !== 'v1' || secret.kind !== 'Secret')
      throw new Error(t('storageYamlInvalid'))
    return secret
  }

  const switchMode = (next: 'form' | 'yaml') => {
    try {
      if (next === 'yaml') syncFormToYaml(form.getFieldsValue(true))
      else {
        const secret = parseSecret(yaml)
        const values = secretValues(secret)
        form.setFieldsValue(values)
        setBase(secret)
      }
      setMode(next)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const save = async (body: Secret) => {
    const values = secretValues(body)
    const requiredKey = values.edition === 'community' ? 'metaurl' : 'token'
    if (!body.metadata?.name || !body.metadata.namespace)
      throw new Error(t('storageYamlInvalid'))
    if (
      initial &&
      (body.metadata.name !== initial.metadata?.name ||
        body.metadata.namespace !== initial.metadata?.namespace)
    )
      throw new Error(t('storageIdentityImmutable'))
    if (!values.entries.find((entry) => entry.key === 'name')?.value.trim() ||
        !values.entries.find((entry) => entry.key === requiredKey)?.value.trim())
      throw new Error(t('secretCredentialsRequired'))
    try {
      parseSecretEnvs(values.entries.find((entry) => entry.key === 'envs')?.value || '')
    } catch {
      throw new Error(t('secretEnvsInvalid'))
    }
    setSaving(true)
    try {
      const path = initial
        ? `/api/v1/secret/${encodeURIComponent(body.metadata.namespace)}/${encodeURIComponent(body.metadata.name)}`
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
          <Button
            type="primary"
            loading={saving}
            onClick={() => {
              if (mode === 'form') form.submit()
              else {
                try {
                  void save(parseSecret(yaml)).catch((err: Error) =>
                    message.error(err.message),
                  )
                } catch (err) {
                  message.error((err as Error).message)
                }
              }
            }}
          >
            {t(initial ? 'save' : 'storageCreate')}
          </Button>
        </Space>
      }
    >
      <Radio.Group
        value={mode}
        onChange={(event) => switchMode(event.target.value)}
        options={[
          { label: t('createFormMode'), value: 'form' },
          { label: 'YAML', value: 'yaml' },
        ]}
        style={{ marginBottom: 16 }}
      />
      {mode === 'yaml' && (
        <Editor
          height="60vh"
          defaultLanguage="yaml"
          value={yaml}
          onChange={(value) => setYaml(value ?? '')}
          options={{ minimap: { enabled: false }, scrollBeyondLastLine: false }}
        />
      )}
      <div style={{ display: mode === 'form' ? undefined : 'none' }}>
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          ...baseValues,
          namespace: baseValues.namespace || 'default',
        }}
        onFinish={(values: Values) => {
          try {
            void save(syncFormToYaml(values)).catch((err: Error) =>
              message.error(err.message),
            )
          } catch (err) {
            message.error((err as Error).message)
          }
        }}
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
        <Form.Item name="edition" label={t('secretEdition')}>
          <Radio.Group
            disabled={!!initial?.immutable}
            options={[
              { label: t('secretEnterprise'), value: 'enterprise' },
              { label: t('secretCommunity'), value: 'community' },
            ]}
          />
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
                <Table
                  size="small"
                  rowKey="key"
                  pagination={false}
                  dataSource={fields.filter(
                    (field) =>
                      field.name >= secretKeys.length ||
                      isVisible(
                        form.getFieldValue(['entries', field.name, 'key']),
                      ),
                  )}
                  columns={[
                    {
                      title: 'Key',
                      width: '35%',
                      render: (_, field) => {
                        const key = form.getFieldValue([
                          'entries', field.name, 'key',
                        ]) as string
                        return field.name < secretKeys.length ? (
                          <span>
                            {key === 'name' ||
                            key === (edition === 'community' ? 'metaurl' : 'token') ? (
                              <span style={{ color: '#ff4d4f' }}>* </span>
                            ) : null}
                            {key}
                          </span>
                        ) : (
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
                        )
                      },
                    },
                    {
                      title: 'Value',
                      render: (_, field) => {
                        const key = form.getFieldValue([
                          'entries', field.name, 'key',
                        ]) as string
                        return (
                          <div style={{ display: 'flex' }}>
                            <Form.Item
                              name={[field.name, 'value']}
                              rules={
                                key === 'envs'
                                  ? [{ validator: (_, value?: string) => {
                                      try {
                                        parseSecretEnvs(value || '')
                                        return Promise.resolve()
                                      } catch {
                                        return Promise.reject(new Error(t('secretEnvsInvalid')))
                                      }
                                    } }]
                                  : field.name < secretKeys.length &&
                                (key === 'name' ||
                                  key === (edition === 'community' ? 'metaurl' : 'token'))
                                  ? required
                                  : undefined
                              }
                              style={{ marginBottom: 0, flex: 1 }}
                            >
                              {key === 'envs' ? (
                                <SecretEnvsInput disabled={!!initial?.immutable} edition={edition} />
                              ) : (
                                <Input.Password
                                  placeholder="Value"
                                  disabled={!!initial?.immutable}
                                />
                              )}
                            </Form.Item>
                          </div>
                        )
                      },
                    },
                    {
                      width: 48,
                      render: (_, field) =>
                        field.name < secretKeys.length ? null : (
                          <Button
                            type="text"
                            icon={<MinusCircleOutlined />}
                            aria-label={t('secretRemoveKey')}
                            disabled={!!initial?.immutable}
                            onClick={() => remove(field.name)}
                          />
                        ),
                    },
                  ]}
                />
                <Button
                  type="dashed"
                  icon={<PlusOutlined />}
                  disabled={!!initial?.immutable}
                  onClick={() => add({ key: '', value: '' })}
                  style={{ marginTop: 8 }}
                >
                  {t('secretAddKey')}
                </Button>
              </>
            )}
          </Form.List>
        </Form.Item>
      </Form>
      </div>
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
            disabled={Object.prototype.hasOwnProperty.call(
              row.metadata.labels || {},
              'juicefs/secret',
            )}
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
