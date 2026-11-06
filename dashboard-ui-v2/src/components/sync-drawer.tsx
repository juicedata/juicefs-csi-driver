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

import { useRef, useState } from 'react'
import Editor from '@monaco-editor/react'
import {
  Button,
  Card,
  Drawer,
  Form,
  FormInstance,
  Input,
  InputNumber,
  message,
  Radio,
  Select,
  Space,
} from 'antd'
import { cloneDeep } from 'lodash'
import { useIntl } from 'react-intl'
import { useSWRConfig } from 'swr'
import YAML from 'yaml'

import { apiFetch } from '@/utils'
import { k8sResourceNameRe } from '@/utils/k8s-validation'

type Credential = {
  mode?: 'value' | 'secretKeyRef' | 'configMapKeyRef' | 'yaml'
  value?: string
  name?: string
  key?: string
}
type SinkKind = 'external' | 'juicefs' | 'juicefsCE'
type Sink = {
  kind: SinkKind
  uri?: string
  volumeName?: string
  metaURL?: string
  path?: string
  consoleUrl?: string
  accessKey?: Credential
  secretKey?: Credential
  token?: Credential
  metaPassWord?: Credential
}
type Values = {
  name: string
  namespace: string
  image: string
  replicas?: number
  options?: string
  ttlSecondsAfterFinished?: number
  from: Sink
  to: Sink
}

type SinkSpec = {
  external?: Record<string, unknown>
  juicefs?: Record<string, unknown>
  juicefsCE?: Record<string, unknown>
}

const defaultYaml = 'apiVersion: juicefs.io/v1\nkind: Sync\n'
const fields = ['uri', 'volumeName', 'metaURL', 'path', 'consoleUrl'] as const
const credentials = ['accessKey', 'secretKey', 'token', 'metaPassWord'] as const

function sinkFromYaml(sink?: SinkSpec): Sink {
  const kind = sink?.external
    ? 'external'
    : sink?.juicefs
      ? 'juicefs'
      : sink?.juicefsCE
        ? 'juicefsCE'
        : 'external'
  const source = sink?.[kind] || {}
  const result: Sink = { kind }
  for (const field of fields) result[field] = (source[field] as string) || ''
  for (const field of credentials) {
    const credential = source[field] as
      | {
          value?: string
          valueFrom?: {
            secretKeyRef?: { name?: string; key?: string }
            configMapKeyRef?: { name?: string; key?: string }
          }
        }
      | undefined
    const mode = credential?.valueFrom?.secretKeyRef
      ? 'secretKeyRef'
      : credential?.valueFrom?.configMapKeyRef
        ? 'configMapKeyRef'
        : credential?.valueFrom
          ? 'yaml'
          : 'value'
    result[field] = {
      mode,
      value: credential?.value || '',
      name:
        credential?.valueFrom?.[
          mode === 'secretKeyRef' || mode === 'configMapKeyRef'
            ? mode
            : 'secretKeyRef'
        ]?.name || '',
      key:
        credential?.valueFrom?.[
          mode === 'secretKeyRef' || mode === 'configMapKeyRef'
            ? mode
            : 'secretKeyRef'
        ]?.key || '',
    }
  }
  return result
}

function SyncSinkForm({
  side,
  form,
  t,
}: {
  side: 'from' | 'to'
  form: FormInstance<Values>
  t: (id: string) => string
}) {
  const kind = Form.useWatch([side, 'kind'], form) || 'external'
  const credentialFields: Array<(typeof credentials)[number]> =
    kind === 'juicefs'
      ? ['token', 'accessKey', 'secretKey']
      : kind === 'juicefsCE'
        ? ['metaPassWord']
        : ['accessKey', 'secretKey']
  return (
    <Card
      title={t(side === 'from' ? 'syncFrom' : 'syncTo')}
      size="small"
      style={{ marginBottom: 24 }}
    >
      <Form.Item name={[side, 'kind']} label={t('syncStorageType')}>
        <Select
          options={[
            { value: 'external', label: t('syncExternal') },
            { value: 'juicefs', label: 'JuiceFS Enterprise' },
            { value: 'juicefsCE', label: 'JuiceFS Community' },
          ]}
        />
      </Form.Item>
      {kind === 'external' && (
        <Form.Item
          name={[side, 'uri']}
          label="URI"
          rules={[{ required: true, message: t('warmupRequired') }]}
        >
          <Input placeholder="s3://bucket/path" />
        </Form.Item>
      )}
      {kind === 'juicefs' && (
        <Form.Item
          name={[side, 'volumeName']}
          label={t('syncVolumeName')}
          rules={[{ required: true, message: t('warmupRequired') }]}
        >
          <Input />
        </Form.Item>
      )}
      {kind === 'juicefsCE' && (
        <Form.Item
          name={[side, 'metaURL']}
          label={t('syncMetaURL')}
          rules={[{ required: true, message: t('warmupRequired') }]}
        >
          <Input placeholder="redis://host:6379/0" />
        </Form.Item>
      )}
      {kind !== 'external' && (
        <Form.Item name={[side, 'path']} label={t('syncPath')}>
          <Input placeholder="/" />
        </Form.Item>
      )}
      {kind === 'juicefs' && (
        <Form.Item name={[side, 'consoleUrl']} label="Console URL">
          <Input />
        </Form.Item>
      )}
      {credentialFields.map((field) => (
        <Form.Item key={field} label={t(`syncCredential_${field}`)}>
          <Space align="start" wrap>
            <Form.Item
              name={[side, field, 'mode']}
              initialValue="value"
              noStyle
            >
              <Select
                style={{ width: 145 }}
                options={[
                  { value: 'value', label: t('syncDirectValue') },
                  { value: 'secretKeyRef', label: 'Secret' },
                  { value: 'configMapKeyRef', label: 'ConfigMap' },
                  { value: 'yaml', label: 'YAML', disabled: true },
                ]}
              />
            </Form.Item>
            <Form.Item
              noStyle
              shouldUpdate={(prev, next) =>
                prev[side]?.[field as keyof Sink] !==
                next[side]?.[field as keyof Sink]
              }
            >
              {() => {
                const mode =
                  form.getFieldValue([side, field, 'mode']) || 'value'
                return mode === 'yaml' ? (
                  t('syncYamlCredentialHint')
                ) : mode === 'value' ? (
                  <Form.Item
                    name={[side, field, 'value']}
                    noStyle
                    rules={
                      field === 'token'
                        ? [{ required: true, message: t('warmupRequired') }]
                        : undefined
                    }
                  >
                    <Input.Password placeholder={t('syncDirectValue')} />
                  </Form.Item>
                ) : (
                  <>
                    <Form.Item
                      name={[side, field, 'name']}
                      noStyle
                      rules={[{ required: true, message: t('warmupRequired') }]}
                    >
                      <Input placeholder={t('name')} />
                    </Form.Item>
                    <Form.Item
                      name={[side, field, 'key']}
                      noStyle
                      rules={[{ required: true, message: t('warmupRequired') }]}
                    >
                      <Input placeholder="Key" />
                    </Form.Item>
                  </>
                )
              }}
            </Form.Item>
          </Space>
        </Form.Item>
      ))}
    </Card>
  )
}

export default function SyncDrawer({ onClose }: { onClose: () => void }) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const { mutate } = useSWRConfig()
  const [form] = Form.useForm<Values>()
  const [yaml, setYaml] = useState(defaultYaml)
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [submitting, setSubmitting] = useState(false)
  const synced = useRef<Partial<Values>>({})
  const required = [
    { required: true, whitespace: true, message: t('warmupRequired') },
  ]

  const syncFormToYaml = (values: Values) => {
    const doc = YAML.parseDocument(yaml)
    if (doc.errors.length) throw doc.errors[0]
    const changed = (a: unknown, b: unknown) =>
      JSON.stringify(a) !== JSON.stringify(b)
    const update = (path: string[], value: unknown) => {
      if (value === undefined || value === '') {
        if (doc.hasIn(path.slice(0, -1))) doc.deleteIn(path)
      } else doc.setIn(path, value)
    }
    for (const key of ['name', 'namespace'] as const)
      if (changed(values[key], synced.current[key]))
        update(['metadata', key], values[key]?.trim())
    for (const key of ['image', 'replicas', 'ttlSecondsAfterFinished'] as const)
      if (changed(values[key], synced.current[key]))
        update(
          ['spec', key],
          typeof values[key] === 'string' ? values[key]?.trim() : values[key],
        )
    if (changed(values.options, synced.current.options)) {
      const options = values.options
        ?.split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
      update(['spec', 'options'], options?.length ? options : undefined)
    }
    for (const side of ['from', 'to'] as const) {
      const sink = values[side]
      const previous = synced.current[side]
      if (!sink) continue
      const typeChanged = sink.kind !== previous?.kind
      if (typeChanged) {
        for (const kind of ['external', 'juicefs', 'juicefsCE'])
          if (kind !== sink.kind) update(['spec', side, kind], undefined)
      }
      const applicable =
        sink.kind === 'external'
          ? ['uri']
          : sink.kind === 'juicefs'
            ? ['volumeName', 'path', 'consoleUrl']
            : ['metaURL', 'path']
      for (const field of fields)
        if (
          applicable.includes(field) &&
          (typeChanged || changed(sink[field], previous?.[field]))
        )
          update(['spec', side, sink.kind, field], sink[field]?.trim())
      const credentialFields =
        sink.kind === 'juicefs'
          ? ['token', 'accessKey', 'secretKey']
          : sink.kind === 'juicefsCE'
            ? ['metaPassWord']
            : ['accessKey', 'secretKey']
      for (const field of credentials) {
        if (
          !credentialFields.includes(field) ||
          (!typeChanged && !changed(sink[field], previous?.[field]))
        )
          continue
        const credential = sink[field]
        const value =
          credential?.mode === 'secretKeyRef' ||
          credential?.mode === 'configMapKeyRef'
            ? credential.name && credential.key
              ? {
                  valueFrom: {
                    [credential.mode]: {
                      name: credential.name,
                      key: credential.key,
                    },
                  },
                }
              : undefined
            : credential?.value
              ? { value: credential.value }
              : undefined
        update(['spec', side, sink.kind, field], value)
      }
    }
    synced.current = cloneDeep(values)
    setYaml(doc.toString())
    return doc.toJS()
  }

  const switchMode = (next: 'form' | 'yaml') => {
    try {
      if (next === 'yaml') syncFormToYaml(form.getFieldsValue(true))
      else {
        const doc = YAML.parseDocument(yaml)
        if (doc.errors.length) throw doc.errors[0]
        const body = doc.toJS() as {
          apiVersion?: string
          kind?: string
          metadata?: { name?: string; namespace?: string }
          spec?: {
            image?: string
            replicas?: number
            options?: string[]
            ttlSecondsAfterFinished?: number
            from?: SinkSpec
            to?: SinkSpec
          }
        }
        if (body?.apiVersion !== 'juicefs.io/v1' || body.kind !== 'Sync')
          throw new Error(t('syncYamlInvalid'))
        const values: Values = {
          name: body.metadata?.name || '',
          namespace: body.metadata?.namespace || '',
          image: body.spec?.image || '',
          replicas: body.spec?.replicas,
          options: body.spec?.options?.join('\n') || '',
          ttlSecondsAfterFinished: body.spec?.ttlSecondsAfterFinished,
          from: sinkFromYaml(body.spec?.from),
          to: sinkFromYaml(body.spec?.to),
        }
        form.setFieldsValue(values)
        synced.current = cloneDeep(values)
      }
      setMode(next)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const create = async (body: unknown) => {
    setSubmitting(true)
    try {
      await apiFetch('/api/v1/syncs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      message.success(t('syncCreated'))
      await mutate('/api/v1/syncs')
      onClose()
    } catch (err) {
      const text = (err as Error).message
      try {
        message.error((JSON.parse(text) as { error: string }).error || text)
      } catch {
        message.error(text)
      }
    } finally {
      setSubmitting(false)
    }
  }
  const submitYaml = () => {
    try {
      const body = YAML.parse(yaml) as {
        apiVersion?: string
        kind?: string
        metadata?: { name?: string; namespace?: string }
        spec?: unknown
      }
      if (
        body?.apiVersion !== 'juicefs.io/v1' ||
        body.kind !== 'Sync' ||
        !body.metadata?.name ||
        !body.metadata.namespace ||
        !body.spec
      )
        throw new Error(t('syncYamlInvalid'))
      void create(body)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  return (
    <Drawer
      title={t('syncCreate')}
      open
      onClose={onClose}
      width="min(720px, 100vw)"
      destroyOnClose
      footer={
        <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Button onClick={onClose}>{t('warmupCancel')}</Button>
          <Button
            type="primary"
            loading={submitting}
            onClick={() => (mode === 'form' ? form.submit() : submitYaml())}
          >
            {t('syncCreate')}
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
            namespace: 'default',
            replicas: 1,
            from: { kind: 'external' },
            to: { kind: 'external' },
          }}
          onFinish={() => {
            try {
              void create(syncFormToYaml(form.getFieldsValue(true)))
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
            <Input />
          </Form.Item>
          <Form.Item name="namespace" label={t('namespace')} rules={required}>
            <Input />
          </Form.Item>
          <Form.Item name="image" label={t('syncImage')} rules={required}>
            <Input placeholder="juicedata/mount:ee-5.4.4" />
          </Form.Item>
          <Form.Item name="replicas" label={t('syncReplicas')}>
            <InputNumber min={1} precision={0} />
          </Form.Item>
          <SyncSinkForm side="from" form={form} t={t} />
          <SyncSinkForm side="to" form={form} t={t} />
          <Form.Item
            name="options"
            label={t('syncOptions')}
            normalize={(value: string) => value.replace(/^([ \t]*)--/gm, '$1')}
            extra={
              <>
                {t('warmupLinesHint')}{' '}
                <a
                  href={t('syncDocsUrl')}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  {t('referenceDocs')}
                </a>
              </>
            }
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="ttlSecondsAfterFinished" label={t('syncTTL')}>
            <InputNumber min={0} precision={0} />
          </Form.Item>
        </Form>
      </div>
    </Drawer>
  )
}
