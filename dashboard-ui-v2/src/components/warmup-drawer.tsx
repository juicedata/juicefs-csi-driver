/*
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

import { useRef, useState } from 'react'
import Editor from '@monaco-editor/react'
import {
  Button,
  Drawer,
  Form,
  Input,
  InputNumber,
  message,
  Radio,
  Select,
  Space,
} from 'antd'
import { useIntl } from 'react-intl'
import { useSWRConfig } from 'swr'
import YAML from 'yaml'

import { useCacheGroups } from '@/hooks/cg-api'
import { apiFetch } from '@/utils'

const defaultWarmupYaml = `apiVersion: juicefs.io/v1
kind: WarmUp
`

type FormValues = {
  name: string
  namespace: string
  groupType: 'managed' | 'external'
  cacheGroupName: string
  secret: string
  image: string
  targetType: 'all' | 'files' | 'configMap' | 'filePath'
  files: string
  configMapName: string
  configMapKey: string
  filePath: string
  options: string
  mountOptions: string
  policy: 'Once' | 'Cron'
  schedule: string
  backoffLimit?: number
  ttl?: number
}

const lines = (value?: string) =>
  value
    ?.split('\n')
    .map((s) => s.trim())
    .filter(Boolean)

export default function WarmupDrawer({
  initialNamespace,
  initialCacheGroup,
  onClose,
}: {
  initialNamespace?: string
  initialCacheGroup?: string
  onClose: () => void
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const { data: groups, error: groupsError } = useCacheGroups()
  const { mutate } = useSWRConfig()
  const [submitting, setSubmitting] = useState(false)
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [yaml, setYaml] = useState(defaultWarmupYaml)
  const [form] = Form.useForm<FormValues>()
  const syncedValues = useRef<Partial<FormValues>>({})
  const namespace = Form.useWatch('namespace', form)
  const groupType = Form.useWatch('groupType', form)
  const targetType = Form.useWatch('targetType', form)
  const policy = Form.useWatch('policy', form)
  const required = [
    { required: true, whitespace: true, message: t('warmupRequired') },
  ]

  const syncFormToYaml = (values: FormValues) => {
    const doc = YAML.parseDocument(yaml)
    if (doc.errors.length) throw doc.errors[0]
    const changed = (key: keyof FormValues) =>
      JSON.stringify(values[key]) !== JSON.stringify(syncedValues.current[key])
    const remove = (path: string[]) => {
      if (doc.hasIn(path.slice(0, -1))) doc.deleteIn(path)
    }
    const update = (path: string[], value: unknown) => {
      if (
        value === undefined ||
        value === '' ||
        (Array.isArray(value) && !value.length)
      )
        remove(path)
      else doc.setIn(path, value)
    }
    if (changed('name')) update(['metadata', 'name'], values.name?.trim())
    if (changed('namespace'))
      update(['metadata', 'namespace'], values.namespace?.trim())
    if (changed('cacheGroupName'))
      update(['spec', 'cacheGroupName'], values.cacheGroupName?.trim())
    if (changed('groupType') || changed('secret')) {
      if (values.groupType === 'external')
        update(['spec', 'secretRef', 'name'], values.secret?.trim())
      else remove(['spec', 'secretRef'])
    }
    if (
      values.groupType === 'external' &&
      (changed('image') || changed('groupType'))
    )
      update(['spec', 'image'], values.image?.trim())
    if (changed('targetType')) {
      remove(['spec', 'targetsFrom', 'files'])
      remove(['spec', 'targetsFrom', 'configMap'])
      remove(['spec', 'targetsFrom', 'filePath'])
    }
    if (
      values.targetType === 'files' &&
      (changed('targetType') || changed('files'))
    )
      update(['spec', 'targetsFrom', 'files'], lines(values.files))
    if (values.targetType === 'configMap') {
      if (changed('targetType') || changed('configMapName'))
        update(
          ['spec', 'targetsFrom', 'configMap', 'name'],
          values.configMapName?.trim(),
        )
      if (changed('targetType') || changed('configMapKey'))
        update(
          ['spec', 'targetsFrom', 'configMap', 'key'],
          values.configMapKey?.trim(),
        )
    }
    if (
      values.targetType === 'filePath' &&
      (changed('targetType') || changed('filePath'))
    )
      update(['spec', 'targetsFrom', 'filePath'], values.filePath?.trim())
    const targets = doc.getIn(['spec', 'targetsFrom'])
    if (YAML.isMap(targets) && targets.items.length === 0)
      remove(['spec', 'targetsFrom'])
    if (changed('options')) update(['spec', 'options'], lines(values.options))
    if (changed('mountOptions'))
      update(['spec', 'mountOptions'], lines(values.mountOptions))
    if (changed('policy')) update(['spec', 'policy', 'type'], values.policy)
    if (values.policy === 'Cron' && (changed('schedule') || changed('policy')))
      update(['spec', 'policy', 'cron', 'schedule'], values.schedule?.trim())
    if (changed('backoffLimit'))
      update(['spec', 'backoffLimit'], values.backoffLimit)
    if (changed('ttl')) update(['spec', 'ttlSecondsAfterFinished'], values.ttl)
    syncedValues.current = { ...values }
    setYaml(doc.toString())
    return doc.toJS()
  }

  const switchMode = (next: 'form' | 'yaml') => {
    try {
      if (next === 'yaml') {
        syncFormToYaml(form.getFieldsValue(true))
      } else {
        const doc = YAML.parseDocument(yaml)
        if (doc.errors.length) throw doc.errors[0]
        const body = doc.toJS() as {
          apiVersion?: string
          kind?: string
          metadata?: { name?: string; namespace?: string }
          spec?: {
            cacheGroupName?: string
            secretRef?: { name?: string }
            image?: string
            targetsFrom?: {
              files?: string[]
              configMap?: { name?: string; key?: string }
              filePath?: string
            }
            options?: string[]
            mountOptions?: string[]
            policy?: { type?: 'Once' | 'Cron'; cron?: { schedule?: string } }
            backoffLimit?: number
            ttlSecondsAfterFinished?: number
          }
        }
        if (
          !body ||
          body.apiVersion !== 'juicefs.io/v1' ||
          body.kind !== 'WarmUp'
        )
          throw new Error(t('warmupYamlInvalid'))
        const spec = body.spec
        const target = spec?.targetsFrom
        const values: FormValues = {
          name: body.metadata?.name || '',
          namespace: body.metadata?.namespace || '',
          cacheGroupName: spec?.cacheGroupName || '',
          groupType: spec?.secretRef ? 'external' : 'managed',
          secret: spec?.secretRef?.name || '',
          image: spec?.image || '',
          targetType: target?.files
            ? 'files'
            : target?.configMap
              ? 'configMap'
              : target?.filePath
                ? 'filePath'
                : 'all',
          files: target?.files?.join('\n') || '',
          configMapName: target?.configMap?.name || '',
          configMapKey: target?.configMap?.key || '',
          filePath: target?.filePath || '',
          options: spec?.options?.join('\n') || '',
          mountOptions: spec?.mountOptions?.join('\n') || '',
          policy: spec?.policy?.type || 'Once',
          schedule: spec?.policy?.cron?.schedule || '',
          backoffLimit: spec?.backoffLimit,
          ttl: spec?.ttlSecondsAfterFinished,
        }
        form.setFieldsValue(values)
        syncedValues.current = { ...values }
      }
      setMode(next)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const submit = async (values: FormValues) => {
    try {
      await createWarmup(syncFormToYaml(values))
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const createWarmup = async (body: unknown) => {
    setSubmitting(true)
    try {
      await apiFetch('/api/v1/warmups', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      onClose()
      await mutate('/api/v1/warmups')
      message.success(t('warmupCreated'))
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
      } | null
      if (
        body?.apiVersion !== 'juicefs.io/v1' ||
        body.kind !== 'WarmUp' ||
        !body.metadata?.name ||
        !body.metadata.namespace ||
        !body.spec
      ) {
        throw new Error(t('warmupYamlInvalid'))
      }
      void createWarmup(body)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  return (
    <Drawer
      title={t('warmupCreate')}
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
            {t('warmupCreate')}
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
          options={{
            minimap: { enabled: false },
            scrollBeyondLastLine: false,
          }}
        />
      )}
      <div style={{ display: mode === 'form' ? undefined : 'none' }}>
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            namespace: initialNamespace || 'default',
            cacheGroupName: initialCacheGroup,
            groupType: 'managed',
            targetType: 'all',
            policy: 'Once',
          }}
          onFinish={() => submit(form.getFieldsValue(true))}
          onValuesChange={(changed) => {
            if ('namespace' in changed || 'groupType' in changed)
              form.setFieldValue('cacheGroupName', undefined)
          }}
        >
          <Form.Item name="name" label={t('name')} rules={required}>
            <Input />
          </Form.Item>
          <Form.Item name="namespace" label={t('namespace')} rules={required}>
            <Input />
          </Form.Item>
          <Form.Item name="groupType" label={t('warmupGroupType')}>
            <Radio.Group
              options={[
                { label: t('warmupManaged'), value: 'managed' },
                { label: t('warmupExternal'), value: 'external' },
              ]}
            />
          </Form.Item>
          {groupType === 'external' ? (
            <>
              <Form.Item
                name="cacheGroupName"
                label={t('cacheGroupName')}
                rules={required}
              >
                <Input />
              </Form.Item>
              <Form.Item
                name="secret"
                label={t('warmupSecret')}
                rules={required}
                extra={t('warmupSecretHint')}
              >
                <Input />
              </Form.Item>
              <Form.Item name="image" label={t('warmupImage')} rules={required}>
                <Input />
              </Form.Item>
            </>
          ) : (
            <Form.Item
              name="cacheGroupName"
              label={t('cacheGroup')}
              rules={required}
              extra={
                groupsError ? t('warmupGroupsError') : t('warmupGroupsHint')
              }
            >
              <Select
                showSearch
                optionFilterProp="label"
                options={(groups || [])
                  .filter((g) => g.metadata?.namespace === namespace)
                  .map((g) => ({
                    label: g.metadata!.name!,
                    value: g.metadata!.name!,
                  }))}
              />
            </Form.Item>
          )}
          <Form.Item name="targetType" label={t('warmupTargetType')}>
            <Select
              options={['all', 'files', 'configMap', 'filePath'].map(
                (value) => ({ value, label: t(`warmupTarget_${value}`) }),
              )}
            />
          </Form.Item>
          {targetType === 'files' && (
            <Form.Item
              name="files"
              label={t('warmupPaths')}
              rules={required}
              extra={t('warmupLinesHint')}
            >
              <Input.TextArea rows={3} />
            </Form.Item>
          )}
          {targetType === 'configMap' && (
            <>
              <Form.Item
                name="configMapName"
                label={t('warmupConfigMap')}
                rules={required}
              >
                <Input />
              </Form.Item>
              <Form.Item
                name="configMapKey"
                label={t('warmupConfigMapKey')}
                rules={required}
              >
                <Input />
              </Form.Item>
            </>
          )}
          {targetType === 'filePath' && (
            <Form.Item
              name="filePath"
              label={t('warmupFilePath')}
              rules={required}
            >
              <Input />
            </Form.Item>
          )}
          <Form.Item
            name="options"
            label={t('warmupOptions')}
            extra={t('warmupOptionsHint')}
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item
            name="mountOptions"
            label={t('mountOptions')}
            extra={t('warmupLinesHint')}
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="policy" label={t('warmupPolicy')}>
            <Radio.Group
              options={[
                { label: t('warmupOnce'), value: 'Once' },
                { label: t('warmupCron'), value: 'Cron' },
              ]}
            />
          </Form.Item>
          {policy === 'Cron' && (
            <Form.Item
              name="schedule"
              label={t('warmupSchedule')}
              rules={required}
            >
              <Input placeholder="0 2 * * *" />
            </Form.Item>
          )}
          <Form.Item name="backoffLimit" label={t('warmupBackoff')}>
            <InputNumber min={0} precision={0} />
          </Form.Item>
          {policy === 'Once' && (
            <Form.Item name="ttl" label={t('warmupTTL')}>
              <InputNumber min={0} precision={0} />
            </Form.Item>
          )}
        </Form>
      </div>
    </Drawer>
  )
}
