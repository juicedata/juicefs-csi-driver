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
import ConfigTablePage from '@/pages/config-table-page.tsx'
import ConfigYamlPage from '@/pages/config-yaml-page.tsx'
import { OriginConfig } from '@/types/k8s.ts'

const ConfigDetail = () => {
  const [updated, setUpdated] = useState(false)

  const { data, isLoading, mutate } = useConfig()
  const { data: pvcs } = useConfigPVC()
  const [configData, setConfigData] = useState('')
  const { data: diffPods, mutate: diffMutate } = useConfigDiff('', '')
  const [diff, setDiff] = useState(false)
  const [error, setError] = useState('')
  const navigate = useNavigate()
  const [edit, setEdit] = useState(false)
  const [activeTabKey, setActiveTabKey] = useState('1')

  const handleTabChange = (key: SetStateAction<string>) => {
    setActiveTabKey(key)
  }

  const [api, contextHolder] = notification.useNotification()

  const [isModalVisible, setIsModalVisible] = useState(false)

  useEffect(() => {
    if (error) {
      api['error']({
        message: <FormattedMessage id="updateConfigError" />,
        description: error,
        placement: 'top',
      })
    }
  }, [api, error])

  useEffect(() => {
    try {
      const raw = data?.data?.['config.yaml']
      const d = raw ? YAML.stringify(YAML.parse(raw)) : ''
      setConfigData(d)
    } catch (e) {
      setError((e as YAMLParseError).message)
      setConfigData(data?.data?.['config.yaml'] || '')
    }
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
                  YAML.stringify(YAML.parse(configData) as OriginConfig)
                  return setIsModalVisible(true)
                } catch (e) {
                  setError((e as YAMLParseError).message)
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
              setError={setError}
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
              disabled={!diff}
              onClick={() => {
                navigate('/jobs?modalOpen=true')
                setDiff(false)
              }}
            >
              <FormattedMessage id="apply" />
            </Button>
          </Popover>
        ) : (
          <Button key="apply" type="primary" disabled={true}>
            <FormattedMessage id="apply" />
          </Button>
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
          setError={setError}
        />
      )}
      {activeTabKey === '2' && (
        <ConfigYamlPage
          setError={setError}
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
