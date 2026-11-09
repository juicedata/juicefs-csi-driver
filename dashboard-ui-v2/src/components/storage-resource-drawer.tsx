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
  AutoComplete,
  Button,
  Checkbox,
  Drawer,
  Form,
  Input,
  message,
  Radio,
  Select,
  Space,
  Switch,
} from 'antd'
import { cloneDeep, get, omit } from 'lodash'
import { useIntl } from 'react-intl'
import useSWR from 'swr'
import YAML from 'yaml'

import { useVersion } from '@/hooks/use-version'
import { apiFetch } from '@/utils'
import {
  k8sResourceNameRe,
  validateMemoryQuantity,
} from '@/utils/k8s-validation'

export type StorageKind = 'pv' | 'pvc' | 'storageclass'

type Values = {
  name: string
  namespace: string
  capacity: string
  accessModes: string[]
  storageClass: string
  noStorageClass: boolean
  volumeName: string
  volumeMode: string
  reclaimPolicy: string
  volumeHandle: string
  mountOptions: string
  parameters: string
  provisionerSecret: string
  nodePublishSecret: string
  expansionSecret: string
  allowExpansion: boolean
  bindingMode: string
}

type Resource = {
  apiVersion?: string
  kind?: string
  metadata?: { name?: string; namespace?: string }
  status?: { phase?: string }
}

const descriptions: Record<
  StorageKind,
  { apiVersion: string; kind: string; plural: string }
> = {
  pv: { apiVersion: 'v1', kind: 'PersistentVolume', plural: 'pvs' },
  pvc: { apiVersion: 'v1', kind: 'PersistentVolumeClaim', plural: 'pvcs' },
  storageclass: {
    apiVersion: 'storage.k8s.io/v1',
    kind: 'StorageClass',
    plural: 'storageclasses',
  },
}

const secretParameters = {
  provisionerSecret: [
    'csi.storage.k8s.io/provisioner-secret-namespace',
    'csi.storage.k8s.io/provisioner-secret-name',
  ],
  nodePublishSecret: [
    'csi.storage.k8s.io/node-publish-secret-namespace',
    'csi.storage.k8s.io/node-publish-secret-name',
  ],
  expansionSecret: [
    'csi.storage.k8s.io/controller-expand-secret-namespace',
    'csi.storage.k8s.io/controller-expand-secret-name',
  ],
} as const
const secretParameterKeys: string[] = Object.values(secretParameters).flat()

function valuesFromYaml(content: string): Values {
  const doc = YAML.parseDocument(content)
  if (doc.errors.length) throw doc.errors[0]
  const body = doc.toJS() as Resource
  const getString = (path: string[]) => String(doc.getIn(path) ?? '')
  const params = get(body, 'parameters') as Record<string, string> | undefined
  const pvSecretName = getString([
    'spec',
    'csi',
    'nodePublishSecretRef',
    'name',
  ])
  const pvSecretNamespace = getString([
    'spec',
    'csi',
    'nodePublishSecretRef',
    'namespace',
  ])
  const secretRef = (keys: readonly [string, string]) => {
    const namespace = params?.[keys[0]] || ''
    const name = params?.[keys[1]] || ''
    return namespace || name ? `${namespace}/${name}` : ''
  }
  return {
    name: getString(['metadata', 'name']),
    namespace: getString(['metadata', 'namespace']),
    capacity: getString(
      doc.getIn(['kind']) === 'PersistentVolume'
        ? ['spec', 'capacity', 'storage']
        : ['spec', 'resources', 'requests', 'storage'],
    ),
    accessModes: (get(body, 'spec.accessModes') as string[] | undefined) || [],
    storageClass: getString(['spec', 'storageClassName']),
    noStorageClass:
      doc.hasIn(['spec', 'storageClassName']) &&
      getString(['spec', 'storageClassName']) === '',
    volumeName: getString(['spec', 'volumeName']),
    volumeMode: getString(['spec', 'volumeMode']) || 'Filesystem',
    reclaimPolicy:
      getString(
        doc.getIn(['kind']) === 'StorageClass'
          ? ['reclaimPolicy']
          : ['spec', 'persistentVolumeReclaimPolicy'],
      ) || (doc.getIn(['kind']) === 'StorageClass' ? 'Delete' : 'Retain'),
    volumeHandle: getString(['spec', 'csi', 'volumeHandle']),
    mountOptions: (
      (get(
        body,
        doc.getIn(['kind']) === 'StorageClass'
          ? 'mountOptions'
          : 'spec.mountOptions',
      ) as string[] | undefined) || []
    ).join('\n'),
    parameters: params
      ? Object.entries(params)
          .filter(([key]) => !secretParameterKeys.includes(key))
          .map(([key, value]) => `${key}=${value}`)
          .join('\n')
      : '',
    provisionerSecret: secretRef(secretParameters.provisionerSecret),
    nodePublishSecret:
      doc.getIn(['kind']) === 'PersistentVolume'
        ? pvSecretName || pvSecretNamespace
          ? `${pvSecretNamespace}/${pvSecretName}`
          : ''
        : secretRef(secretParameters.nodePublishSecret),
    expansionSecret: secretRef(secretParameters.expansionSecret),
    allowExpansion: Boolean(doc.getIn(['allowVolumeExpansion'])),
    bindingMode: getString(['volumeBindingMode']) || 'Immediate',
  }
}

export default function StorageResourceDrawer({
  kind,
  initial,
  onClose,
  onSaved,
}: {
  kind: StorageKind
  initial?: Resource
  onClose: () => void
  onSaved: () => Promise<void> | void
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const desc = descriptions[kind]
  const { data: version } = useVersion()
  const secretManagementEnabled = version?.secretManagementEnabled === true
  const { data: driver } = useSWR<{ name: string }>('/api/v1/storage-driver')
  const {
    data: secrets,
    error: secretsError,
    isLoading: secretsLoading,
  } = useSWR<
    Array<{
      metadata: {
        name: string
        namespace: string
        labels?: Record<string, string>
      }
    }>
  >(
    (kind === 'storageclass' || kind === 'pv') && secretManagementEnabled
      ? '/api/v1/secrets'
      : null,
  )
  const {
    data: storageClasses,
    error: storageClassesError,
    isLoading: storageClassesLoading,
  } = useSWR<string[]>(kind === 'pvc' ? '/api/v1/storageclasses/basic' : null)
  const [form] = Form.useForm<Values>()
  const noStorageClass = Form.useWatch('noStorageClass', form)
  const allowExpansion = Form.useWatch('allowExpansion', form)
  const volumeHandleCustomized = useRef(false)
  const nodeSecretCustomized = useRef(false)
  const expansionSecretCustomized = useRef(false)
  const [initialYaml] = useState(() =>
    initial
      ? YAML.stringify({
          ...omit(initial, ['metadata.managedFields']),
          apiVersion: desc.apiVersion,
          kind: desc.kind,
        })
      : `apiVersion: ${desc.apiVersion}\nkind: ${desc.kind}\n`,
  )
  const [initialValues] = useState(() =>
    initial ? valuesFromYaml(initialYaml) : undefined,
  )
  const [yaml, setYaml] = useState(initialYaml)
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [saving, setSaving] = useState(false)
  const synced = useRef<Partial<Values>>(
    initialValues ? cloneDeep(initialValues) : {},
  )
  const required = [
    { required: true, whitespace: true, message: t('warmupRequired') },
  ]
  const secretRefRules = [
    ...required,
    {
      pattern: /^[^/\s]+\/[^/\s]+$/,
      message: t('storageSecretReferenceInvalid'),
    },
  ]
  const secretOptions = (secrets || [])
    .filter(
      ({ metadata }) =>
        !Object.prototype.hasOwnProperty.call(
          metadata.labels || {},
          'juicefs/secret',
        ),
    )
    .map(({ metadata }) => ({
      value: `${metadata.namespace}/${metadata.name}`,
    }))
  const parseEntries = (value?: string) =>
    value
      ?.split('\n')
      .map((s) => s.trim())
      .filter(Boolean) || []

  const syncFormToYaml = (values: Values) => {
    const doc = YAML.parseDocument(yaml)
    if (doc.errors.length) throw doc.errors[0]
    const changed = (key: keyof Values) =>
      JSON.stringify(values[key]) !== JSON.stringify(synced.current[key])
    const update = (path: string[], value: unknown) => {
      if (
        value === undefined ||
        value === '' ||
        (Array.isArray(value) && value.length === 0)
      ) {
        if (doc.hasIn(path.slice(0, -1))) doc.deleteIn(path)
      } else doc.setIn(path, value)
    }
    if (changed('name')) update(['metadata', 'name'], values.name?.trim())
    if (kind === 'pvc' && changed('namespace'))
      update(['metadata', 'namespace'], values.namespace?.trim())
    if (kind !== 'storageclass') {
      if (changed('capacity'))
        update(
          kind === 'pv'
            ? ['spec', 'capacity', 'storage']
            : ['spec', 'resources', 'requests', 'storage'],
          values.capacity?.trim(),
        )
      if (changed('accessModes'))
        update(['spec', 'accessModes'], values.accessModes)
      if (
        changed('storageClass') ||
        (kind === 'pvc' && changed('noStorageClass'))
      ) {
        if (kind === 'pvc' && values.noStorageClass)
          doc.setIn(['spec', 'storageClassName'], '')
        else update(['spec', 'storageClassName'], values.storageClass?.trim())
      }
      if (changed('volumeMode'))
        update(['spec', 'volumeMode'], values.volumeMode)
    }
    if (kind === 'pvc' && changed('volumeName'))
      update(['spec', 'volumeName'], values.volumeName?.trim())
    if (kind === 'pv') {
      if (!initial && !doc.getIn(['spec', 'csi', 'driver']))
        update(['spec', 'csi', 'driver'], driver?.name)
      if (!initial && !doc.getIn(['spec', 'csi', 'fsType']))
        update(['spec', 'csi', 'fsType'], 'juicefs')
      if (changed('volumeHandle'))
        update(['spec', 'csi', 'volumeHandle'], values.volumeHandle?.trim())
      if (changed('nodePublishSecret')) {
        const value = values.nodePublishSecret?.trim()
        if (value && !/^[^/\s]+\/[^/\s]+$/.test(value))
          throw new Error(t('storageSecretReferenceInvalid'))
        const [namespace, name] = value ? value.split('/') : []
        update(['spec', 'csi', 'nodePublishSecretRef', 'name'], name)
        update(['spec', 'csi', 'nodePublishSecretRef', 'namespace'], namespace)
      }
    }
    if (kind !== 'pvc') {
      if (changed('reclaimPolicy'))
        update(
          kind === 'pv'
            ? ['spec', 'persistentVolumeReclaimPolicy']
            : ['reclaimPolicy'],
          values.reclaimPolicy,
        )
      if (changed('mountOptions'))
        update(
          kind === 'pv' ? ['spec', 'mountOptions'] : ['mountOptions'],
          parseEntries(values.mountOptions),
        )
    }
    if (kind === 'storageclass') {
      if (!initial && !doc.getIn(['provisioner']))
        update(['provisioner'], driver?.name)
      if (changed('allowExpansion'))
        update(['allowVolumeExpansion'], values.allowExpansion)
      if (changed('bindingMode'))
        update(['volumeBindingMode'], values.bindingMode)
      if (changed('parameters')) {
        const entries = parseEntries(values.parameters).map((line) => {
          const index = line.indexOf('=')
          if (index < 1) throw new Error(t('storageParametersInvalid'))
          const key = line.slice(0, index).trim()
          if (secretParameterKeys.includes(key))
            throw new Error(t('storageSecretParameterReserved'))
          return [key, line.slice(index + 1).trim()]
        })
        const current = (get(doc.toJS(), 'parameters') || {}) as Record<
          string,
          string
        >
        for (const key of Object.keys(current))
          if (!secretParameterKeys.includes(key))
            update(['parameters', key], undefined)
        for (const [key, value] of entries)
          doc.setIn(['parameters', key], value)
      }
      for (const field of [
        'provisionerSecret',
        'nodePublishSecret',
        'expansionSecret',
      ] as const) {
        if (field === 'expansionSecret' && !values.allowExpansion) continue
        if (
          !changed(field) &&
          !(field === 'expansionSecret' && changed('allowExpansion'))
        )
          continue
        const value = values[field]?.trim()
        if (value && !/^[^/\s]+\/[^/\s]+$/.test(value))
          throw new Error(t('storageSecretReferenceInvalid'))
        const [namespace, name] = value ? value.split('/') : []
        const [namespaceKey, nameKey] = secretParameters[field]
        update(['parameters', namespaceKey], namespace)
        update(['parameters', nameKey], name)
      }
    }
    synced.current = cloneDeep(values)
    setYaml(doc.toString())
    return doc.toJS() as Resource
  }

  const switchMode = (next: 'form' | 'yaml') => {
    try {
      if (next === 'yaml') syncFormToYaml(form.getFieldsValue(true))
      else {
        const parsed = YAML.parse(yaml) as Resource
        if (parsed?.apiVersion !== desc.apiVersion || parsed.kind !== desc.kind)
          throw new Error(t('storageYamlInvalid'))
        const values = valuesFromYaml(yaml)
        form.setFieldsValue(values)
        synced.current = cloneDeep(values)
        if (!initial && kind === 'pv') {
          if (values.volumeHandle && values.volumeHandle !== values.name)
            volumeHandleCustomized.current = true
          if (!values.volumeHandle && !volumeHandleCustomized.current)
            form.setFieldValue('volumeHandle', values.name)
        }
        if (!initial && kind === 'storageclass') {
          nodeSecretCustomized.current =
            nodeSecretCustomized.current ||
            (!!values.nodePublishSecret &&
              values.nodePublishSecret !== values.provisionerSecret)
          expansionSecretCustomized.current =
            expansionSecretCustomized.current ||
            (!!values.expansionSecret &&
              values.expansionSecret !== values.provisionerSecret)
        }
      }
      setMode(next)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const save = async (body: Resource) => {
    if (
      body?.apiVersion !== desc.apiVersion ||
      body.kind !== desc.kind ||
      !body.metadata?.name ||
      (kind === 'pvc' && !body.metadata.namespace)
    ) {
      message.error(t('storageYamlInvalid'))
      return
    }
    if (
      initial &&
      (body.metadata.name !== initial.metadata?.name ||
        body.metadata.namespace !== initial.metadata?.namespace)
    ) {
      message.error(t('storageIdentityImmutable'))
      return
    }
    setSaving(true)
    try {
      const path = initial
        ? `/api/v1/${kind}/${kind === 'pvc' ? `${encodeURIComponent(initial.metadata?.namespace || '')}/` : ''}${encodeURIComponent(initial.metadata?.name || '')}/`
        : `/api/v1/${desc.plural}`
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
      title={`${t(initial ? 'edit' : 'storageCreate')} ${desc.kind}`}
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
            disabled={!initial && kind !== 'pvc' && !driver}
            onClick={() => {
              if (mode === 'form') form.submit()
              else {
                try {
                  void save(YAML.parse(yaml) as Resource)
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
          initialValues={
            initialValues || {
              namespace: 'default',
              capacity: '1Gi',
              accessModes: ['ReadWriteMany'],
              volumeMode: 'Filesystem',
              reclaimPolicy: kind === 'storageclass' ? 'Delete' : 'Retain',
              allowExpansion: false,
              bindingMode: 'Immediate',
            }
          }
          onFinish={() => {
            try {
              void save(syncFormToYaml(form.getFieldsValue(true)))
            } catch (err) {
              message.error((err as Error).message)
            }
          }}
          onValuesChange={(changed: Partial<Values>) => {
            if (!initial && kind === 'pv') {
              if ('volumeHandle' in changed) volumeHandleCustomized.current = true
              if ('name' in changed && !volumeHandleCustomized.current)
                form.setFieldValue('volumeHandle', changed.name)
            }
            if (initial || kind !== 'storageclass') return
            if ('nodePublishSecret' in changed)
              nodeSecretCustomized.current = true
            if ('expansionSecret' in changed)
              expansionSecretCustomized.current = true
            if ('provisionerSecret' in changed) {
              if (!nodeSecretCustomized.current)
                form.setFieldValue(
                  'nodePublishSecret',
                  changed.provisionerSecret,
                )
              if (!expansionSecretCustomized.current)
                form.setFieldValue('expansionSecret', changed.provisionerSecret)
            }
            if (
              changed.allowExpansion &&
              !expansionSecretCustomized.current &&
              !form.getFieldValue('expansionSecret')
            )
              form.setFieldValue(
                'expansionSecret',
                form.getFieldValue('provisionerSecret'),
              )
          }}
        >
          {initial && <p>{t('storageImmutableHint')}</p>}
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
          {kind === 'pvc' && (
            <Form.Item name="namespace" label={t('namespace')} rules={required}>
              <Input disabled={!!initial} />
            </Form.Item>
          )}
          {kind !== 'storageclass' && (
            <>
              <Form.Item
                name="capacity"
                label={t('capacity')}
                rules={[...required, { validator: validateMemoryQuantity }]}
                extra={
                  kind === 'pvc' && initial ? t('pvcResizeHint') : undefined
                }
              >
                <Input
                  disabled={
                    kind === 'pvc' &&
                    !!initial &&
                    initial.status?.phase !== 'Bound'
                  }
                  placeholder="1Gi"
                />
              </Form.Item>
              <Form.Item
                name="accessModes"
                label={t('accessMode')}
                rules={[
                  {
                    required: true,
                    type: 'array',
                    min: 1,
                    message: t('warmupRequired'),
                  },
                ]}
              >
                <Select
                  mode="multiple"
                  disabled={kind === 'pvc' && !!initial}
                  options={[
                    'ReadWriteOnce',
                    'ReadWriteMany',
                    'ReadOnlyMany',
                    'ReadWriteOncePod',
                  ].map((value) => ({ value, label: value }))}
                />
              </Form.Item>
              <Form.Item
                name="storageClass"
                label="StorageClass"
                extra={
                  kind === 'pvc'
                    ? t(
                        storageClassesError
                          ? 'storageClassesLoadError'
                          : 'pvcStorageClassHint',
                      )
                    : undefined
                }
              >
                {kind === 'pvc' ? (
                  <AutoComplete
                    disabled={!!initial || noStorageClass}
                    options={(storageClasses || []).map((value) => ({ value }))}
                    filterOption={(input, option) =>
                      String(option?.value || '')
                        .toLowerCase()
                        .includes(input.toLowerCase())
                    }
                    placeholder={t(
                      storageClassesLoading
                        ? 'storageClassesLoading'
                        : 'storageClassSelect',
                    )}
                    allowClear
                  />
                ) : (
                  <Input />
                )}
              </Form.Item>
              {kind === 'pvc' && (
                <Form.Item name="noStorageClass" valuePropName="checked">
                  <Checkbox disabled={!!initial}>
                    {t('pvcNoStorageClass')}
                  </Checkbox>
                </Form.Item>
              )}
              <Form.Item
                name="volumeMode"
                label={t('storageVolumeMode')}
                hidden={kind === 'pv'}
              >
                <Select
                  disabled={!!initial || kind === 'pv'}
                  options={[
                    { value: 'Filesystem', label: 'Filesystem' },
                    ...(kind === 'pvc' ? [{ value: 'Block', label: 'Block' }] : []),
                  ]}
                />
              </Form.Item>
            </>
          )}
          {kind === 'pvc' && (
            <Form.Item name="volumeName" label={t('storagePVName')}>
              <Input disabled={!!initial} />
            </Form.Item>
          )}
          {kind === 'pv' && (
            <>
              <Form.Item
                name="volumeHandle"
                label="Volume Handle"
                rules={required}
              >
                <Input disabled={!!initial} />
              </Form.Item>
              <Form.Item
                name="nodePublishSecret"
                label={t('storage_nodePublishSecret')}
                rules={initial ? undefined : secretRefRules}
                extra={
                  !secretManagementEnabled
                    ? t('storageSecretReferenceManualHint')
                    : secretsError
                      ? t('secretListError')
                      : t('storageSecretReferenceHint')
                }
              >
                <AutoComplete
                  disabled={!!initial}
                  options={secretOptions}
                  filterOption={(input, option) =>
                    String(option?.value || '')
                      .toLowerCase()
                      .includes(input.toLowerCase())
                  }
                  placeholder={t(
                    !secretManagementEnabled
                      ? 'storageSecretReferenceManualHint'
                      : secretsLoading
                        ? 'storageSecretsLoading'
                        : 'storageSecretSelect',
                  )}
                  allowClear
                />
              </Form.Item>
            </>
          )}
          {kind !== 'pvc' && (
            <>
              <Form.Item name="reclaimPolicy" label={t('reclaimPolicy')}>
                <Select
                  disabled={kind === 'storageclass' && !!initial}
                  options={['Retain', 'Delete'].map((value) => ({
                    value,
                    label: value,
                  }))}
                />
              </Form.Item>
              <Form.Item
                name="mountOptions"
                label={t('mountOptions')}
                extra={t('warmupLinesHint')}
                normalize={(value: string) =>
                  value.replace(/^([ \t]*)--/gm, '$1')
                }
              >
                <Input.TextArea rows={3} />
              </Form.Item>
            </>
          )}
          {kind === 'storageclass' && (
            <>
              {(['provisionerSecret', 'nodePublishSecret'] as const).map(
                (field) => (
                  <Form.Item
                    key={field}
                    name={field}
                    label={t(`storage_${field}`)}
                    rules={initial ? undefined : secretRefRules}
                    extra={
                      !secretManagementEnabled
                        ? t('storageSecretReferenceManualHint')
                        : secretsError
                        ? t('secretListError')
                        : t('storageSecretReferenceHint')
                    }
                  >
                    <AutoComplete
                      disabled={!!initial}
                      options={secretOptions}
                      filterOption={(input, option) =>
                        String(option?.value || '')
                          .toLowerCase()
                          .includes(input.toLowerCase())
                      }
                      placeholder={t(
                        !secretManagementEnabled
                          ? 'storageSecretReferenceManualHint'
                          : secretsLoading
                          ? 'storageSecretsLoading'
                          : 'storageSecretSelect',
                      )}
                      allowClear
                    />
                  </Form.Item>
                ),
              )}
              <Form.Item
                name="parameters"
                label={t('storageExtraParameters')}
                extra={t('storageParametersHint')}
                rules={[
                  {
                    validator: (_, value?: string) =>
                      !value ||
                      parseEntries(value).every((line) => line.indexOf('=') > 0)
                        ? Promise.resolve()
                        : Promise.reject(
                            new Error(t('storageParametersInvalid')),
                          ),
                  },
                ]}
              >
                <Input.TextArea
                  disabled={!!initial}
                  rows={4}
                  placeholder={'pathPattern=${.pvc.namespace}-${.pvc.name}'}
                />
              </Form.Item>
              <Form.Item name="bindingMode" label={t('storageBindingMode')}>
                <Select
                  disabled={!!initial}
                  options={['Immediate', 'WaitForFirstConsumer'].map(
                    (value) => ({ value, label: value }),
                  )}
                />
              </Form.Item>
              <Form.Item
                name="allowExpansion"
                label={t('allowVolumeExpansion')}
                valuePropName="checked"
                extra={
                  initial && !initialValues?.expansionSecret
                    ? t('storageExpansionImmutableHint')
                    : undefined
                }
              >
                <Switch />
              </Form.Item>
              {allowExpansion && (
                <Form.Item
                  name="expansionSecret"
                  label={t('storage_expansionSecret')}
                  rules={initial ? undefined : secretRefRules}
                  extra={
                    !secretManagementEnabled
                      ? t('storageSecretReferenceManualHint')
                      : secretsError
                      ? t('secretListError')
                      : t('storageSecretReferenceHint')
                  }
                >
                  <AutoComplete
                    disabled={!!initial}
                    options={secretOptions}
                    filterOption={(input, option) =>
                      String(option?.value || '')
                        .toLowerCase()
                        .includes(input.toLowerCase())
                    }
                    placeholder={t(
                      !secretManagementEnabled
                        ? 'storageSecretReferenceManualHint'
                        : secretsLoading
                        ? 'storageSecretsLoading'
                        : 'storageSecretSelect',
                    )}
                    allowClear
                  />
                </Form.Item>
              )}
            </>
          )}
        </Form>
      </div>
    </Drawer>
  )
}
