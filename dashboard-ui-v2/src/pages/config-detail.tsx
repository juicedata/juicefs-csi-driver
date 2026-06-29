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

import { SetStateAction, useEffect, useState } from 'react'
import { QuestionCircleOutlined } from '@ant-design/icons'
import { PageContainer, ProCard } from '@ant-design/pro-components'
import { Button, notification, Popover, Tooltip } from 'antd'
import { FormattedMessage } from 'react-intl'
import { useNavigate } from 'react-router-dom'
import YAML, { YAMLParseError } from 'yaml'

import ConfigUpdateConfirmModal from '@/components/config/config-update-modal.tsx'
import { useConfig, useConfigDiff, useConfigPVC } from '@/hooks/cm-api'
import { useVersion } from '@/hooks/use-version.ts'
import ConfigTablePage from '@/pages/config-table-page.tsx'
import ConfigYamlPage from '@/pages/config-yaml-page.tsx'
import { OriginConfig } from '@/types/k8s.ts'
import {
  envVarNameRe,
  isValidK8sKey,
  isValidLabelValue,
  isValidQuantity,
  validCacheDirTypes,
  validDnsPolicies,
} from '@/utils/k8s-validation.ts'

function validateConfigData(config: OriginConfig): string | null {
  for (let i = 0; i < (config.mountPodPatch?.length ?? 0); i++) {
    const patch = config.mountPodPatch![i]

    // labels
    for (const [k, v] of Object.entries(patch.labels ?? {})) {
      if (!isValidK8sKey(k))
        return `mountPodPatch[${i}].labels: invalid key "${k}"`
      if (!isValidLabelValue(v))
        return `mountPodPatch[${i}].labels: invalid value "${v}" for key "${k}"`
    }

    // annotations
    for (const k of Object.keys(patch.annotations ?? {})) {
      if (!isValidK8sKey(k))
        return `mountPodPatch[${i}].annotations: invalid key "${k}"`
    }

    // env
    for (const env of patch.env ?? []) {
      if (!env.name || !envVarNameRe.test(env.name))
        return `mountPodPatch[${i}].env: invalid environment variable name "${env.name}"`
    }

    // resources
    const res = patch.resources
    if (res) {
      for (const [name, qty] of Object.entries(res.requests ?? {})) {
        if (!isValidQuantity(qty as string))
          return `mountPodPatch[${i}].resources.requests.${name}: invalid quantity "${qty}"`
      }
      for (const [name, qty] of Object.entries(res.limits ?? {})) {
        if (!isValidQuantity(qty as string))
          return `mountPodPatch[${i}].resources.limits.${name}: invalid quantity "${qty}"`
      }
    }

    const volumeNames = new Set((patch.volumes ?? []).map((v) => v.name))
    const usedVolumes = new Set<string>()
    for (let j = 0; j < (patch.volumeMounts?.length ?? 0); j++) {
      const volumeMount = patch.volumeMounts![j]
      if (!volumeNames.has(volumeMount.name))
        return `mountPodPatch[${i}].volumeMounts[${j}]: volume "${volumeMount.name}" not found in volumes`
      usedVolumes.add(volumeMount.name)
    }
    for (let j = 0; j < (patch.volumeDevices?.length ?? 0); j++) {
      const volumeDevice = patch.volumeDevices![j]
      if (!volumeNames.has(volumeDevice.name))
        return `mountPodPatch[${i}].volumeDevices[${j}]: volume "${volumeDevice.name}" not found in volumes`
      usedVolumes.add(volumeDevice.name)
    }
    for (let j = 0; j < (patch.volumes?.length ?? 0); j++) {
      const volume = patch.volumes![j]
      if (!usedVolumes.has(volume.name))
        return `mountPodPatch[${i}].volumes[${j}]: volume "${volume.name}" not found in volumeMounts or volumeDevices`
    }

    // cacheDirs
    for (let j = 0; j < (patch.cacheDirs?.length ?? 0); j++) {
      const cd = patch.cacheDirs![j]
      if (!validCacheDirTypes.has(cd.type))
        return `mountPodPatch[${i}].cacheDirs[${j}]: invalid type "${cd.type}", must be one of HostPath, PVC, EmptyDir, Ephemeral`
      if (cd.type === 'HostPath' && !cd.path)
        return `mountPodPatch[${i}].cacheDirs[${j}]: path is required for HostPath type`
      if (cd.type === 'PVC' && !cd.name)
        return `mountPodPatch[${i}].cacheDirs[${j}]: name is required for PVC type`
    }

    // dnsPolicy
    if (patch.dnsPolicy && !validDnsPolicies.has(patch.dnsPolicy))
      return `mountPodPatch[${i}].dnsPolicy: invalid value "${patch.dnsPolicy}", must be one of ClusterFirst, ClusterFirstWithHostNet, Default, None`
  }
  return null
}

const ConfigDetail = () => {
  const [updated, setUpdated] = useState(false)

  const { data, isLoading, mutate } = useConfig()
  const { data: pvcs } = useConfigPVC()
  const [configData, setConfigData] = useState('')
  const { data: diffPods, mutate: diffMutate } = useConfigDiff('', '')
  const [diff, setDiff] = useState(false)
  const navigate = useNavigate()
  const [edit, setEdit] = useState(false)
  const [activeTabKey, setActiveTabKey] = useState('1')
  const { data: versionData } = useVersion()

  const handleTabChange = (key: SetStateAction<string>) => {
    setActiveTabKey(key)
  }

  const [api, contextHolder] = notification.useNotification()

  const [isModalVisible, setIsModalVisible] = useState(false)

  const showError = (msg: string) => {
    api['error']({
      message: <FormattedMessage id="updateConfigError" />,
      description: msg,
      placement: 'top',
    })
  }

  useEffect(() => {
    try {
      const raw = data?.data?.['config.yaml']
      const d = raw ? YAML.stringify(YAML.parse(raw)) : ''
      setConfigData(d)
    } catch (e) {
      showError((e as YAMLParseError).message)
      setConfigData(data?.data?.['config.yaml'] || '')
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  useEffect(() => {
    setDiff((diffPods?.pods?.length || 0) > 0)
  }, [diffPods])

  useEffect(() => {
    if (!updated) {
      diffMutate()
      mutate()
    }
  }, [diffMutate, mutate, updated])

  if (!data) {
    return (
      <PageContainer
        fixedHeader
        className="config-page-header"
        header={{
          title: <FormattedMessage id="config" />,
          subTitle: (
            <Tooltip title="Docs">
              <Button
                icon={<QuestionCircleOutlined />}
                className="header-subtitle-button"
                onClick={() => {
                  window.open(
                    'https://juicefs.com/docs/zh/csi/guide/configurations',
                    '_blank',
                  )
                }}
              />
            </Tooltip>
          ),
          ghost: true,
        }}
      >
        <ProCard>
          <FormattedMessage id="configNotFound" />
        </ProCard>
      </PageContainer>
    )
  }

  return (
    <PageContainer
      fixedHeader
      className="config-page-header"
      header={{
        title: <FormattedMessage id="config" />,
        subTitle: (
          <Tooltip title="Docs">
            <Button
              icon={<QuestionCircleOutlined />}
              className="header-subtitle-button"
              onClick={() => {
                window.open(
                  'https://juicefs.com/docs/zh/csi/guide/configurations',
                  '_blank',
                )
              }}
            />
          </Tooltip>
        ),
        ghost: true,
      }}
      extra={[
        !edit && (
          <Button
            key="edit docs"
            loading={isLoading}
            onClick={() => {
              setEdit(true)
            }}
          >
            <FormattedMessage id="edit" />
          </Button>
        ),
        edit && (
          <Button
            key="reset docs"
            loading={isLoading}
            onClick={() => {
              mutate()
              if (data) {
                setConfigData(data.data?.['config.yaml'] || '')
                setEdit(false)
              }
            }}
          >
            <FormattedMessage id="reset" />
          </Button>
        ),
        edit && (
          <>
            <Button
              key="update docs"
              type="primary"
              disabled={
                YAML.stringify(configData) ==
                YAML.stringify(data?.data?.['config.yaml'] || '')
              }
              onClick={() => {
                try {
                  const parsed = YAML.parse(configData) as OriginConfig
                  YAML.stringify(parsed)
                  const validationError = validateConfigData(parsed)
                  if (validationError) {
                    showError(validationError)
                    return
                  }
                  setIsModalVisible(true)
                } catch (e) {
                  showError((e as YAMLParseError).message)
                }
              }}
            >
              <FormattedMessage id="save" />
            </Button>
            <ConfigUpdateConfirmModal
              modalOpen={isModalVisible}
              onOk={() => {
                setIsModalVisible(false)
                setUpdated(false)
              }}
              onCancel={() => setIsModalVisible(false)}
              setUpdated={setUpdated}
              setEdit={setEdit}
              setError={showError}
              data={data}
              configData={configData}
            />
          </>
        ),

        diff ? (
          <Popover
            key="diff pods"
            placement="bottomRight"
            title={<FormattedMessage id="diffPods" />}
            content={
              <div>
                {diffPods?.pods?.map((poddiff) => (
                  <p key={poddiff?.pod.metadata?.uid || ''}>
                    {poddiff?.pod.metadata?.name}
                  </p>
                ))}
              </div>
            }
          >
            <Button
              key="apply"
              type="primary"
              disabled={!diff || versionData?.disableGraceUpgrade}
              onClick={() => {
                navigate('/jobs?modalOpen=true')
                setDiff(false)
              }}
            >
              <FormattedMessage id="apply" />
            </Button>
          </Popover>
        ) : (
          <Tooltip
            title={
              versionData?.disableGraceUpgrade ? (
                <FormattedMessage id="smoothUpgradeDisabled" />
              ) : null
            }
          >
            <Button key="apply" type="primary" disabled={true}>
              <FormattedMessage id="apply" />
            </Button>
          </Tooltip>
        ),
      ]}
      tabActiveKey={activeTabKey}
      onTabChange={handleTabChange}
      tabList={[
        {
          key: '1',
          tab: 'Detail',
        },
        {
          key: '2',
          tab: 'Yaml',
        },
      ]}
    >
      {contextHolder}
      {activeTabKey === '1' && (
        <ConfigTablePage
          configData={configData}
          setConfigData={setConfigData}
          setUpdate={setUpdated}
          pvcs={pvcs}
          edit={edit}
          setError={showError}
        />
      )}
      {activeTabKey === '2' && (
        <ConfigYamlPage
          setUpdated={setUpdated}
          setConfigData={setConfigData}
          configData={configData}
          edit={edit}
        />
      )}
    </PageContainer>
  )
}

export default ConfigDetail
