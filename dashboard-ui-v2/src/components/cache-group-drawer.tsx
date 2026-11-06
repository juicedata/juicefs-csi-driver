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

import { useRef, useState } from 'react'
import { MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import Editor from '@monaco-editor/react'
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Collapse,
  Drawer,
  Form,
  Input,
  message,
  Radio,
  Select,
  Space,
  Switch,
} from 'antd'
import { EnvVar } from 'kubernetes-types/core/v1'
import { cloneDeep, omit } from 'lodash'
import { useIntl } from 'react-intl'
import YAML from 'yaml'

import { useNodes } from '@/hooks/use-api'
import { CacheGroup } from '@/types/k8s'
import {
  k8sResourceNameRe,
  validateCPUQuantity,
  validateEnvVarName,
  validateMemoryQuantity,
} from '@/utils/k8s-validation'

const defaultCreateCgTemplate = `apiVersion: juicefs.io/v1
kind: CacheGroup
`

type WorkerOverride = {
  nodes?: string[]
  nodeSelector?: Record<string, string>
  image?: string
  cacheDirs?: Array<{ type?: string; path?: string; [key: string]: unknown }>
  opts?: string[]
  env?: EnvVar[]
  resources?: {
    requests?: Record<string, string>
    limits?: Record<string, string>
    [key: string]: unknown
  }
  [key: string]: unknown
}

type OverrideValues = {
  original: WorkerOverride
  baseline?: Omit<OverrideValues, 'original' | 'baseline'>
  matchMode: 'nodes' | 'labels' | 'both'
  nodes: string[]
  selector: string
  useImage: boolean
  image: string
  useCacheDirs: boolean
  cacheDirs: string
  useOptions: boolean
  options: string
  useEnv: boolean
  env: EnvVar[]
  useResources: boolean
  cpuRequest: string
  memoryRequest: string
  cpuLimit: string
  memoryLimit: string
}

const overrideFromYaml = (original: WorkerOverride): OverrideValues => {
  const values = {
    matchMode: (original.nodes?.length && original.nodeSelector
      ? 'both'
      : original.nodeSelector
        ? 'labels'
        : 'nodes') as OverrideValues['matchMode'],
    nodes: original.nodes || [],
    selector: Object.entries(original.nodeSelector || {})
      .map(([key, value]) => `${key}=${value}`)
      .join('\n'),
    useImage: !!original.image,
    image: original.image || '',
    useCacheDirs: original.cacheDirs != null,
    cacheDirs: (original.cacheDirs || [])
      .filter((dir) => dir.type === 'HostPath')
      .map((dir) => dir.path || '')
      .join('\n'),
    useOptions: original.opts != null,
    options: original.opts?.join('\n') || '',
    useEnv: original.env != null,
    env: original.env || [],
    useResources: original.resources != null,
    cpuRequest: original.resources?.requests?.cpu || '',
    memoryRequest: original.resources?.requests?.memory || '',
    cpuLimit: original.resources?.limits?.cpu || '',
    memoryLimit: original.resources?.limits?.memory || '',
  }
  return { ...values, original, baseline: cloneDeep(values) }
}

type CreateValues = {
  name: string
  namespace: string
  cacheGroup?: string
  cleanCache?: boolean
  updateStrategy?: 'RollingUpdate' | 'OnDelete'
  maxUnavailable?: string
  secret: string
  image: string
  nodeSelector: string
  cacheDirs?: string
  options?: string
  replicas?: number
  backupDuration?: string
  waitingDuration?: string
  hostNetwork: boolean
  env?: EnvVar[]
  cpuRequest?: string
  memoryRequest?: string
  cpuLimit?: string
  memoryLimit?: string
  overwrite?: OverrideValues[]
}

const valuesFromYaml = (
  content: string,
  invalidMessage: string,
): CreateValues => {
  const doc = YAML.parseDocument(content)
  if (doc.errors.length) throw doc.errors[0]
  const body = doc.toJS() as {
    apiVersion?: string
    kind?: string
    metadata?: { name?: string; namespace?: string }
    spec?: {
      cacheGroup?: string
      cleanCache?: boolean
      updateStrategy?: {
        type?: 'RollingUpdate' | 'OnDelete'
        rollingUpdate?: { maxUnavailable?: number | string }
      }
      secretRef?: { name?: string }
      replicas?: number
      backupDuration?: string
      waitingDeletedMaxDuration?: string
      worker?: {
        overwrite?: WorkerOverride[]
        template?: {
          image?: string
          nodeSelector?: Record<string, string>
          cacheDirs?: Array<{ type?: string; path?: string }>
          opts?: string[]
          hostNetwork?: boolean
          env?: EnvVar[]
          resources?: {
            requests?: { cpu?: string; memory?: string }
            limits?: { cpu?: string; memory?: string }
          }
        }
      }
    }
  }
  if (
    !body ||
    body.apiVersion !== 'juicefs.io/v1' ||
    body.kind !== 'CacheGroup'
  )
    throw new Error(invalidMessage)
  const spec = body.spec
  const template = spec?.worker?.template
  const values: CreateValues = {
    name: body.metadata?.name || '',
    namespace: body.metadata?.namespace || '',
    cacheGroup: spec?.cacheGroup || '',
    cleanCache: spec?.cleanCache ?? false,
    updateStrategy: spec?.updateStrategy?.type || 'RollingUpdate',
    maxUnavailable: String(
      spec?.updateStrategy?.rollingUpdate?.maxUnavailable ?? '',
    ),
    secret: spec?.secretRef?.name || '',
    image: template?.image || '',
    nodeSelector: Object.entries(template?.nodeSelector || {})
      .map(([key, value]) => `${key}=${value}`)
      .join('\n'),
    cacheDirs: (template?.cacheDirs || [])
      .filter((dir) => dir.type === 'HostPath')
      .map((dir) => dir.path || '')
      .join('\n'),
    options: template?.opts?.join('\n') || '',
    replicas: spec?.replicas,
    backupDuration: spec?.backupDuration || '',
    waitingDuration: spec?.waitingDeletedMaxDuration || '',
    hostNetwork: template?.hostNetwork ?? false,
    env: template?.env || [],
    cpuRequest: template?.resources?.requests?.cpu || '',
    memoryRequest: template?.resources?.requests?.memory || '',
    cpuLimit: template?.resources?.limits?.cpu || '',
    memoryLimit: template?.resources?.limits?.memory || '',
    overwrite: spec?.worker?.overwrite?.map(overrideFromYaml),
  }
  return values
}

export default function CacheGroupDrawer({
  initialGroup,
  onClose,
  onSave,
}: {
  initialGroup?: CacheGroup
  onClose: () => void
  onSave: (body: CacheGroup) => Promise<void>
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const [initialYaml] = useState(() =>
    initialGroup
      ? YAML.stringify(omit(initialGroup, ['metadata.managedFields']))
      : defaultCreateCgTemplate,
  )
  const [initialValues] = useState(() =>
    initialGroup
      ? valuesFromYaml(initialYaml, t('cacheGroupYamlInvalid'))
      : undefined,
  )
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [yaml, setYaml] = useState(initialYaml)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm<CreateValues>()
  const updateStrategy = Form.useWatch('updateStrategy', form)
  const cacheGroupCustomized = useRef(false)
  const { data: nodes } = useNodes()
  const syncedValues = useRef<Partial<CreateValues>>(
    initialValues ? cloneDeep(initialValues) : {},
  )
  const required = [
    { required: true, whitespace: true, message: t('warmupRequired') },
  ]

  const save = async (body: CacheGroup) => {
    setSubmitting(true)
    try {
      if (
        initialGroup &&
        (body.metadata?.name !== initialGroup.metadata?.name ||
          body.metadata?.namespace !== initialGroup.metadata?.namespace)
      ) {
        throw new Error(t('cacheGroupIdentityImmutable'))
      }
      await onSave(body)
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

  const syncFormToYaml = (values: CreateValues) => {
    const doc = YAML.parseDocument(yaml)
    if (doc.errors.length) throw doc.errors[0]
    const body = doc.toJS() as {
      spec?: {
        worker?: {
          template?: { cacheDirs?: Array<{ type?: string; path?: string }> }
        }
      }
    }
    const changed = (key: keyof CreateValues) =>
      JSON.stringify(values[key]) !== JSON.stringify(syncedValues.current[key])
    const remove = (path: string[]) => {
      if (doc.hasIn(path.slice(0, -1))) doc.deleteIn(path)
    }
    const update = (path: string[], value: unknown) => {
      if (value === undefined || value === '') remove(path)
      else doc.setIn(path, value)
    }
    const entries = (value?: string) =>
      value
        ?.split('\n')
        .map((line) => line.trim())
        .filter(Boolean)
    if (changed('name')) update(['metadata', 'name'], values.name?.trim())
    if (changed('namespace'))
      update(['metadata', 'namespace'], values.namespace?.trim())
    if (changed('cacheGroup'))
      update(['spec', 'cacheGroup'], values.cacheGroup?.trim())
    if (changed('cleanCache'))
      update(['spec', 'cleanCache'], values.cleanCache)
    if (changed('updateStrategy'))
      update(['spec', 'updateStrategy', 'type'], values.updateStrategy)
    if (changed('maxUnavailable')) {
      const value = values.maxUnavailable?.trim()
      update(
        ['spec', 'updateStrategy', 'rollingUpdate', 'maxUnavailable'],
        value ? (/^\d+$/.test(value) ? Number(value) : value) : undefined,
      )
    }
    if (changed('secret'))
      update(['spec', 'secretRef', 'name'], values.secret?.trim())
    if (changed('image'))
      update(['spec', 'worker', 'template', 'image'], values.image?.trim())
    if (changed('nodeSelector')) {
      const selector = entries(values.nodeSelector)?.map((line) => {
        const index = line.indexOf('=')
        if (index < 1 || !line.slice(0, index).trim())
          throw new Error(t('cacheGroupNodeSelectorInvalid'))
        return [line.slice(0, index).trim(), line.slice(index + 1).trim()]
      })
      update(
        ['spec', 'worker', 'template', 'nodeSelector'],
        selector?.length ? Object.fromEntries(selector) : undefined,
      )
    }
    if (changed('cacheDirs')) {
      const old = body?.spec?.worker?.template?.cacheDirs || []
      const dirs = (entries(values.cacheDirs) || []).map(
        (path) =>
          old.find((dir) => dir.type === 'HostPath' && dir.path === path) || {
            type: 'HostPath',
            path,
          },
      )
      dirs.push(...old.filter((dir) => dir.type !== 'HostPath'))
      update(
        ['spec', 'worker', 'template', 'cacheDirs'],
        dirs.length ? dirs : undefined,
      )
    }
    if (changed('options'))
      update(
        ['spec', 'worker', 'template', 'opts'],
        entries(values.options)?.length ? entries(values.options) : undefined,
      )
    if (changed('env'))
      update(
        ['spec', 'worker', 'template', 'env'],
        values.env?.length ? values.env : undefined,
      )
    if (changed('cpuRequest'))
      update(
        ['spec', 'worker', 'template', 'resources', 'requests', 'cpu'],
        values.cpuRequest?.trim(),
      )
    if (changed('memoryRequest'))
      update(
        ['spec', 'worker', 'template', 'resources', 'requests', 'memory'],
        values.memoryRequest?.trim(),
      )
    if (changed('cpuLimit'))
      update(
        ['spec', 'worker', 'template', 'resources', 'limits', 'cpu'],
        values.cpuLimit?.trim(),
      )
    if (changed('memoryLimit'))
      update(
        ['spec', 'worker', 'template', 'resources', 'limits', 'memory'],
        values.memoryLimit?.trim(),
      )
    if (changed('overwrite')) {
      const rules = (values.overwrite || []).map((rule) => {
        const result = cloneDeep(rule.original || {})
        const ruleChanged = (key: keyof OverrideValues) =>
          !rule.baseline ||
          JSON.stringify(rule[key]) !==
            JSON.stringify(rule.baseline[key as keyof typeof rule.baseline])
        if (
          ruleChanged('matchMode') ||
          ruleChanged('nodes') ||
          ruleChanged('selector')
        ) {
          if (rule.matchMode !== 'labels') result.nodes = rule.nodes || []
          else delete result.nodes
          if (rule.matchMode !== 'nodes') {
            const labels = entries(rule.selector)?.map((line) => {
              const index = line.indexOf('=')
              if (index < 1 || !line.slice(0, index).trim())
                throw new Error(t('cacheGroupNodeSelectorInvalid'))
              return [line.slice(0, index).trim(), line.slice(index + 1).trim()]
            })
            if (labels?.length) result.nodeSelector = Object.fromEntries(labels)
            else delete result.nodeSelector
          } else delete result.nodeSelector
        }
        if (ruleChanged('useImage') || ruleChanged('image')) {
          if (rule.useImage) result.image = rule.image?.trim()
          else delete result.image
        }
        if (ruleChanged('useCacheDirs') || ruleChanged('cacheDirs')) {
          if (rule.useCacheDirs) {
            const old = result.cacheDirs || []
            result.cacheDirs = (entries(rule.cacheDirs) || []).map(
              (path) =>
                old.find(
                  (dir) => dir.type === 'HostPath' && dir.path === path,
                ) || { type: 'HostPath', path },
            )
            result.cacheDirs.push(
              ...old.filter((dir) => dir.type !== 'HostPath'),
            )
            if (!result.cacheDirs.length) throw new Error(t('cgOverwriteEmpty'))
          } else delete result.cacheDirs
        }
        if (ruleChanged('useOptions') || ruleChanged('options')) {
          if (rule.useOptions) {
            result.opts = entries(rule.options) || []
            if (!result.opts.length) throw new Error(t('cgOverwriteEmpty'))
          } else delete result.opts
        }
        if (ruleChanged('useEnv') || ruleChanged('env')) {
          if (rule.useEnv) {
            result.env = rule.env || []
            if (!result.env.length) throw new Error(t('cgOverwriteEmpty'))
          } else delete result.env
        }
        if (
          ruleChanged('useResources') ||
          ['cpuRequest', 'memoryRequest', 'cpuLimit', 'memoryLimit'].some(
            (key) => ruleChanged(key as keyof OverrideValues),
          )
        ) {
          if (rule.useResources) {
            const resources = cloneDeep(result.resources || {})
            const requests = { ...resources.requests }
            const limits = { ...resources.limits }
            if (rule.cpuRequest) requests.cpu = rule.cpuRequest.trim()
            else delete requests.cpu
            if (rule.memoryRequest) requests.memory = rule.memoryRequest.trim()
            else delete requests.memory
            if (rule.cpuLimit) limits.cpu = rule.cpuLimit.trim()
            else delete limits.cpu
            if (rule.memoryLimit) limits.memory = rule.memoryLimit.trim()
            else delete limits.memory
            if (Object.keys(requests).length) resources.requests = requests
            else delete resources.requests
            if (Object.keys(limits).length) resources.limits = limits
            else delete resources.limits
            result.resources = resources
          } else delete result.resources
        }
        return result
      })
      update(['spec', 'worker', 'overwrite'], rules.length ? rules : undefined)
    }
    if (changed('replicas')) update(['spec', 'replicas'], values.replicas)
    if (changed('backupDuration'))
      update(['spec', 'backupDuration'], values.backupDuration?.trim())
    if (changed('waitingDuration'))
      update(
        ['spec', 'waitingDeletedMaxDuration'],
        values.waitingDuration?.trim(),
      )
    if (changed('hostNetwork')) {
      update(['spec', 'worker', 'template', 'hostNetwork'], values.hostNetwork)
      if (
        values.hostNetwork &&
        !doc.getIn(['spec', 'worker', 'template', 'dnsPolicy'])
      )
        doc.setIn(
          ['spec', 'worker', 'template', 'dnsPolicy'],
          'ClusterFirstWithHostNet',
        )
    }
    syncedValues.current = cloneDeep(values)
    const next = doc.toString()
    setYaml(next)
    return doc.toJS() as CacheGroup
  }

  const switchMode = (next: 'form' | 'yaml') => {
    try {
      if (next === 'yaml') {
        syncFormToYaml(form.getFieldsValue(true))
      } else {
        const values = valuesFromYaml(yaml, t('cacheGroupYamlInvalid'))
        form.setFieldsValue(values as unknown as Parameters<typeof form.setFieldsValue>[0])
        syncedValues.current = cloneDeep(values)
        if (!initialGroup)
          cacheGroupCustomized.current =
            cacheGroupCustomized.current || values.cacheGroup !== values.name
      }
      setMode(next)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const submitForm = (values: CreateValues) => {
    try {
      void save(syncFormToYaml(values))
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const submitYaml = () => {
    try {
      const body = YAML.parse(yaml) as CacheGroup & {
        apiVersion?: string
        kind?: string
      }
      if (
        body?.apiVersion !== 'juicefs.io/v1' ||
        body.kind !== 'CacheGroup' ||
        !body.metadata?.name ||
        !body.metadata.namespace ||
        !body.spec
      ) {
        throw new Error(t('cacheGroupYamlInvalid'))
      }
      void save(body)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  return (
    <Drawer
      title={t(initialGroup ? 'cacheGroupEdit' : 'cacheGroupCreate')}
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
            {t(initialGroup ? 'save' : 'cacheGroupCreate')}
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
          initialValues={
            initialValues || {
              namespace: 'default',
              cleanCache: false,
              updateStrategy: 'RollingUpdate',
              nodeSelector: 'juicefs.io/cg-worker=true',
              hostNetwork: true,
            }
          }
          onValuesChange={(changed: Partial<CreateValues>) => {
            if (initialGroup) return
            if ('cacheGroup' in changed) cacheGroupCustomized.current = true
            if ('name' in changed && !cacheGroupCustomized.current)
              form.setFieldValue('cacheGroup', changed.name)
          }}
          onFinish={() => submitForm(form.getFieldsValue(true))}
        >
          <Form.Item
            name="name"
            label={t('name')}
            rules={[
              ...required,
              { pattern: k8sResourceNameRe, max: 253, message: t('resourceNameInvalid') },
            ]}
          >
            <Input disabled={!!initialGroup} />
          </Form.Item>
          <Form.Item name="namespace" label={t('namespace')} rules={required}>
            <Input disabled={!!initialGroup} />
          </Form.Item>
          <Form.Item
            name="cacheGroup"
            label={t('cacheGroupActualName')}
            extra={t('cacheGroupActualNameHint')}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="secret"
            label={t('warmupSecret')}
            rules={required}
            extra={t('cacheGroupSecretHint')}
          >
            <Input />
          </Form.Item>
          <Form.Item name="image" label={t('cacheGroupImage')} rules={required}>
            <Input placeholder="juicedata/mount:ee-5.4.4" />
          </Form.Item>
          <Form.Item
            name="nodeSelector"
            label={t('cacheGroupNodeSelector')}
            extra={t('cacheGroupNodeSelectorHint')}
            rules={[
              ...required,
              {
                validator: (_, value: string) =>
                  !value ||
                  value
                    .split('\n')
                    .filter((line) => line.trim())
                    .every(
                      (line) => line.includes('=') && line.split('=')[0].trim(),
                    )
                    ? Promise.resolve()
                    : Promise.reject(
                        new Error(t('cacheGroupNodeSelectorInvalid')),
                      ),
              },
            ]}
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item
            name="cacheDirs"
            label={t('cacheGroupCacheDirs')}
            extra={t('warmupLinesHint')}
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item
            name="options"
            label={t('mountOptions')}
            extra={t('warmupLinesHint')}
          >
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item label={t('envs')} extra={t('cacheGroupEnvHint')}>
            <Form.List name="env">
              {(fields, { add, remove }) => (
                <>
                  {fields.map((field) => (
                    <Space
                      key={field.key}
                      align="start"
                      style={{ display: 'flex', marginBottom: 8 }}
                    >
                      <Form.Item
                        name={[field.name, 'name']}
                        rules={[...required, { validator: validateEnvVarName }]}
                        style={{ marginBottom: 0 }}
                      >
                        <Input placeholder={t('cacheGroupEnvName')} />
                      </Form.Item>
                      {form.getFieldValue(['env', field.name, 'valueFrom']) ? (
                        <span>{t('cacheGroupEnvValueFrom')}</span>
                      ) : (
                        <Form.Item
                          name={[field.name, 'value']}
                          style={{ marginBottom: 0 }}
                        >
                          <Input placeholder={t('cacheGroupEnvValue')} />
                        </Form.Item>
                      )}
                      <Button
                        type="text"
                        icon={<MinusCircleOutlined />}
                        aria-label={t('cacheGroupRemoveEnv')}
                        onClick={() => remove(field.name)}
                      />
                    </Space>
                  ))}
                  <Button
                    type="dashed"
                    icon={<PlusOutlined />}
                    onClick={() => add({ name: '', value: '' })}
                  >
                    {t('cacheGroupAddEnv')}
                  </Button>
                </>
              )}
            </Form.List>
          </Form.Item>
          <Form.Item label={t('resourceRequests')}>
            <Space align="start">
              <Form.Item
                name="cpuRequest"
                rules={[{ validator: validateCPUQuantity }]}
                style={{ marginBottom: 0 }}
              >
                <Input
                  aria-label={t('cacheGroupCpuRequest')}
                  placeholder="CPU (100m)"
                />
              </Form.Item>
              <Form.Item
                name="memoryRequest"
                rules={[{ validator: validateMemoryQuantity }]}
                style={{ marginBottom: 0 }}
              >
                <Input
                  aria-label={t('cacheGroupMemoryRequest')}
                  placeholder="Memory (128Mi)"
                />
              </Form.Item>
            </Space>
          </Form.Item>
          <Form.Item label={t('resourceLimits')}>
            <Space align="start">
              <Form.Item
                name="cpuLimit"
                rules={[{ validator: validateCPUQuantity }]}
                style={{ marginBottom: 0 }}
              >
                <Input
                  aria-label={t('cacheGroupCpuLimit')}
                  placeholder="CPU (1)"
                />
              </Form.Item>
              <Form.Item
                name="memoryLimit"
                rules={[{ validator: validateMemoryQuantity }]}
                style={{ marginBottom: 0 }}
              >
                <Input
                  aria-label={t('cacheGroupMemoryLimit')}
                  placeholder="Memory (2Gi)"
                />
              </Form.Item>
            </Space>
          </Form.Item>
          <Collapse
            items={[
              {
                key: 'overwrite',
                label: t('cgOverwriteTitle'),
                children: (
                  <>
                    <p>{t('cgOverwriteHint')}</p>
                    {form.getFieldValue('replicas') != null && (
                      <Alert
                        type="warning"
                        showIcon
                        message={t('cgOverwriteReplicas')}
                        style={{ marginBottom: 16 }}
                      />
                    )}
                    <Form.List name="overwrite">
                      {(fields, { add, remove, move }) => (
                        <>
                          {fields.map((field, index) => (
                            <Card
                              key={field.key}
                              size="small"
                              title={`${t('cgOverwriteRule')} ${index + 1}`}
                              style={{ marginBottom: 16 }}
                              extra={
                                <Space>
                                  <Button
                                    size="small"
                                    disabled={index === 0}
                                    onClick={() => move(index, index - 1)}
                                  >
                                    {t('cgOverwriteUp')}
                                  </Button>
                                  <Button
                                    size="small"
                                    disabled={index === fields.length - 1}
                                    onClick={() => move(index, index + 1)}
                                  >
                                    {t('cgOverwriteDown')}
                                  </Button>
                                  <Button
                                    size="small"
                                    danger
                                    onClick={() => remove(field.name)}
                                  >
                                    {t('cgOverwriteRemove')}
                                  </Button>
                                </Space>
                              }
                            >
                              <Form.Item noStyle shouldUpdate>
                                {() => {
                                  const rule = form.getFieldValue([
                                    'overwrite',
                                    field.name,
                                  ]) as OverrideValues
                                  const set = (
                                    key: keyof OverrideValues,
                                    value: unknown,
                                  ) =>
                                    form.setFieldValue(
                                      ['overwrite', field.name, key],
                                      value,
                                    )
                                  return (
                                    <>
                                      <Form.Item
                                        name={[field.name, 'matchMode']}
                                        label={t('cgOverwriteMatch')}
                                      >
                                        <Radio.Group
                                          options={[
                                            {
                                              label: t('cgOverwriteNodes'),
                                              value: 'nodes',
                                            },
                                            {
                                              label: t('cgOverwriteLabels'),
                                              value: 'labels',
                                            },
                                            ...(rule?.matchMode === 'both'
                                              ? [
                                                  {
                                                    label: t('cgOverwriteBoth'),
                                                    value: 'both',
                                                    disabled: true,
                                                  },
                                                ]
                                              : []),
                                          ]}
                                        />
                                      </Form.Item>
                                      {rule?.matchMode === 'both' && (
                                        <Alert
                                          type="info"
                                          showIcon
                                          message={t('cgOverwriteBothHint')}
                                          style={{ marginBottom: 16 }}
                                        />
                                      )}
                                      {rule?.matchMode !== 'labels' && (
                                        <Form.Item
                                          name={[field.name, 'nodes']}
                                          label={t('cgOverwriteNodes')}
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
                                            mode="tags"
                                            options={(nodes || []).map(
                                              (node) => ({
                                                label:
                                                  node.metadata?.name || '',
                                                value:
                                                  node.metadata?.name || '',
                                              }),
                                            )}
                                          />
                                        </Form.Item>
                                      )}
                                      {rule?.matchMode !== 'nodes' && (
                                        <Form.Item
                                          name={[field.name, 'selector']}
                                          label={t('cgOverwriteLabels')}
                                          extra={t(
                                            'cacheGroupNodeSelectorHint',
                                          )}
                                          rules={[
                                            ...required,
                                            {
                                              validator: (_, value: string) =>
                                                !value ||
                                                value
                                                  .split('\n')
                                                  .filter((line) => line.trim())
                                                  .every(
                                                    (line) =>
                                                      line.includes('=') &&
                                                      line.split('=')[0].trim(),
                                                  )
                                                  ? Promise.resolve()
                                                  : Promise.reject(
                                                      new Error(
                                                        t(
                                                          'cacheGroupNodeSelectorInvalid',
                                                        ),
                                                      ),
                                                    ),
                                            },
                                          ]}
                                        >
                                          <Input.TextArea rows={2} />
                                        </Form.Item>
                                      )}
                                      <p>{t('cgOverwriteReplaceHint')}</p>
                                      <Form.Item
                                        name={[field.name, 'useImage']}
                                        valuePropName="checked"
                                      >
                                        <Checkbox
                                          onChange={(event) => {
                                            if (
                                              event.target.checked &&
                                              !rule.image
                                            )
                                              set(
                                                'image',
                                                form.getFieldValue('image') ||
                                                  '',
                                              )
                                          }}
                                        >
                                          {t('cacheGroupImage')}
                                        </Checkbox>
                                      </Form.Item>
                                      {rule?.useImage && (
                                        <Form.Item
                                          name={[field.name, 'image']}
                                          rules={required}
                                        >
                                          <Input />
                                        </Form.Item>
                                      )}
                                      <Form.Item
                                        name={[field.name, 'useCacheDirs']}
                                        valuePropName="checked"
                                      >
                                        <Checkbox
                                          onChange={(event) => {
                                            if (
                                              event.target.checked &&
                                              !rule.cacheDirs
                                            )
                                              set(
                                                'cacheDirs',
                                                form.getFieldValue(
                                                  'cacheDirs',
                                                ) || '',
                                              )
                                          }}
                                        >
                                          {t('cacheGroupCacheDirs')}
                                        </Checkbox>
                                      </Form.Item>
                                      {rule?.useCacheDirs && (
                                        <Form.Item
                                          name={[field.name, 'cacheDirs']}
                                          extra={t('warmupLinesHint')}
                                        >
                                          <Input.TextArea rows={2} />
                                        </Form.Item>
                                      )}
                                      <Form.Item
                                        name={[field.name, 'useOptions']}
                                        valuePropName="checked"
                                      >
                                        <Checkbox
                                          onChange={(event) => {
                                            if (
                                              event.target.checked &&
                                              !rule.options
                                            )
                                              set(
                                                'options',
                                                form.getFieldValue('options') ||
                                                  '',
                                              )
                                          }}
                                        >
                                          {t('mountOptions')}
                                        </Checkbox>
                                      </Form.Item>
                                      {rule?.useOptions && (
                                        <Form.Item
                                          name={[field.name, 'options']}
                                          extra={t('warmupLinesHint')}
                                        >
                                          <Input.TextArea rows={2} />
                                        </Form.Item>
                                      )}
                                      <Form.Item
                                        name={[field.name, 'useEnv']}
                                        valuePropName="checked"
                                      >
                                        <Checkbox
                                          onChange={(event) => {
                                            if (
                                              event.target.checked &&
                                              !rule.env?.length
                                            )
                                              set(
                                                'env',
                                                cloneDeep(
                                                  form.getFieldValue('env') ||
                                                    [],
                                                ),
                                              )
                                          }}
                                        >
                                          {t('envs')}
                                        </Checkbox>
                                      </Form.Item>
                                      {rule?.useEnv && (
                                        <Form.Item
                                          extra={t('cacheGroupEnvHint')}
                                        >
                                          <Form.List name={[field.name, 'env']}>
                                            {(envFields, envActions) => (
                                              <>
                                                {envFields.map((envField) => (
                                                  <Space
                                                    key={envField.key}
                                                    align="start"
                                                    style={{
                                                      display: 'flex',
                                                      marginBottom: 8,
                                                    }}
                                                  >
                                                    <Form.Item
                                                      name={[
                                                        envField.name,
                                                        'name',
                                                      ]}
                                                      rules={[
                                                        ...required,
                                                        {
                                                          validator:
                                                            validateEnvVarName,
                                                        },
                                                      ]}
                                                      style={{
                                                        marginBottom: 0,
                                                      }}
                                                    >
                                                      <Input
                                                        placeholder={t(
                                                          'cacheGroupEnvName',
                                                        )}
                                                      />
                                                    </Form.Item>
                                                    {form.getFieldValue([
                                                      'overwrite',
                                                      field.name,
                                                      'env',
                                                      envField.name,
                                                      'valueFrom',
                                                    ]) ? (
                                                      <span>
                                                        {t(
                                                          'cacheGroupEnvValueFrom',
                                                        )}
                                                      </span>
                                                    ) : (
                                                      <Form.Item
                                                        name={[
                                                          envField.name,
                                                          'value',
                                                        ]}
                                                        style={{
                                                          marginBottom: 0,
                                                        }}
                                                      >
                                                        <Input
                                                          placeholder={t(
                                                            'cacheGroupEnvValue',
                                                          )}
                                                        />
                                                      </Form.Item>
                                                    )}
                                                    <Button
                                                      type="text"
                                                      icon={
                                                        <MinusCircleOutlined />
                                                      }
                                                      aria-label={t(
                                                        'cacheGroupRemoveEnv',
                                                      )}
                                                      onClick={() =>
                                                        envActions.remove(
                                                          envField.name,
                                                        )
                                                      }
                                                    />
                                                  </Space>
                                                ))}
                                                <Button
                                                  type="dashed"
                                                  icon={<PlusOutlined />}
                                                  onClick={() =>
                                                    envActions.add({
                                                      name: '',
                                                      value: '',
                                                    })
                                                  }
                                                >
                                                  {t('cacheGroupAddEnv')}
                                                </Button>
                                              </>
                                            )}
                                          </Form.List>
                                        </Form.Item>
                                      )}
                                      <Form.Item
                                        name={[field.name, 'useResources']}
                                        valuePropName="checked"
                                      >
                                        <Checkbox
                                          onChange={(event) => {
                                            if (
                                              event.target.checked &&
                                              !rule.useResources
                                            ) {
                                              for (const key of [
                                                'cpuRequest',
                                                'memoryRequest',
                                                'cpuLimit',
                                                'memoryLimit',
                                              ] as const)
                                                if (!rule[key])
                                                  set(
                                                    key,
                                                    form.getFieldValue(key) ||
                                                      '',
                                                  )
                                            }
                                          }}
                                        >
                                          {t('resources')}
                                        </Checkbox>
                                      </Form.Item>
                                      {rule?.useResources && (
                                        <>
                                          <Form.Item
                                            label={t('resourceRequests')}
                                          >
                                            <Space align="start">
                                              <Form.Item
                                                name={[
                                                  field.name,
                                                  'cpuRequest',
                                                ]}
                                                rules={[
                                                  {
                                                    validator:
                                                      validateCPUQuantity,
                                                  },
                                                ]}
                                                style={{ marginBottom: 0 }}
                                              >
                                                <Input
                                                  aria-label={t(
                                                    'cacheGroupCpuRequest',
                                                  )}
                                                  placeholder="CPU (100m)"
                                                />
                                              </Form.Item>
                                              <Form.Item
                                                name={[
                                                  field.name,
                                                  'memoryRequest',
                                                ]}
                                                rules={[
                                                  {
                                                    validator:
                                                      validateMemoryQuantity,
                                                  },
                                                ]}
                                                style={{ marginBottom: 0 }}
                                              >
                                                <Input
                                                  aria-label={t(
                                                    'cacheGroupMemoryRequest',
                                                  )}
                                                  placeholder="Memory (128Mi)"
                                                />
                                              </Form.Item>
                                            </Space>
                                          </Form.Item>
                                          <Form.Item
                                            label={t('resourceLimits')}
                                          >
                                            <Space align="start">
                                              <Form.Item
                                                name={[field.name, 'cpuLimit']}
                                                rules={[
                                                  {
                                                    validator:
                                                      validateCPUQuantity,
                                                  },
                                                ]}
                                                style={{ marginBottom: 0 }}
                                              >
                                                <Input
                                                  aria-label={t(
                                                    'cacheGroupCpuLimit',
                                                  )}
                                                  placeholder="CPU (1)"
                                                />
                                              </Form.Item>
                                              <Form.Item
                                                name={[
                                                  field.name,
                                                  'memoryLimit',
                                                ]}
                                                rules={[
                                                  {
                                                    validator:
                                                      validateMemoryQuantity,
                                                  },
                                                ]}
                                                style={{ marginBottom: 0 }}
                                              >
                                                <Input
                                                  aria-label={t(
                                                    'cacheGroupMemoryLimit',
                                                  )}
                                                  placeholder="Memory (2Gi)"
                                                />
                                              </Form.Item>
                                            </Space>
                                          </Form.Item>
                                        </>
                                      )}
                                    </>
                                  )
                                }}
                              </Form.Item>
                            </Card>
                          ))}
                          <Button
                            type="dashed"
                            icon={<PlusOutlined />}
                            onClick={() => add(overrideFromYaml({}))}
                          >
                            {t('cgOverwriteAdd')}
                          </Button>
                        </>
                      )}
                    </Form.List>
                  </>
                ),
              },
            ]}
            style={{ marginBottom: 24 }}
          />
          <Form.Item
            name="backupDuration"
            label={t('cacheGroupBackupDuration')}
          >
            <Input placeholder="1h" />
          </Form.Item>
          <Form.Item
            name="waitingDuration"
            label={t('cacheGroupWaitingDuration')}
          >
            <Input placeholder="1h" />
          </Form.Item>
          <Form.Item
            name="updateStrategy"
            label={t('cacheGroupUpdateStrategy')}
          >
            <Select
              options={[
                { label: 'RollingUpdate', value: 'RollingUpdate' },
                { label: 'OnDelete', value: 'OnDelete' },
              ]}
            />
          </Form.Item>
          {updateStrategy === 'RollingUpdate' && (
            <Form.Item
              name="maxUnavailable"
              label={t('cacheGroupMaxUnavailable')}
              extra={t('cacheGroupMaxUnavailableHint')}
              rules={[
                {
                  validator: (_, value?: string) =>
                    !value ||
                    /^\d+$/.test(value) ||
                    (/^\d+%$/.test(value) && parseInt(value, 10) <= 100)
                      ? Promise.resolve()
                      : Promise.reject(
                          new Error(t('cacheGroupMaxUnavailableInvalid')),
                        ),
                },
              ]}
            >
              <Input placeholder="1 / 25%" />
            </Form.Item>
          )}
          <Form.Item
            name="cleanCache"
            label={t('cacheGroupCleanCache')}
            extra={t('cacheGroupCleanCacheHint')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="hostNetwork"
            label={t('cacheGroupHostNetwork')}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>
        </Form>
      </div>
    </Drawer>
  )
}
