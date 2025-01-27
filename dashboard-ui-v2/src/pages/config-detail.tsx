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

import { useEffect, useState } from 'react'
import { PageContainer } from '@ant-design/pro-components'
import { Button, Popover, Tabs, TabsProps } from 'antd'
import { FormattedMessage } from 'react-intl'
import { useNavigate } from 'react-router-dom'
import YAML, { YAMLParseError } from 'yaml'

import {
  useConfig,
  useConfigDiff,
  useConfigPVC,
  useUpdateConfig,
} from '@/hooks/cm-api'
import ConfigTablePage from '@/pages/config-table-page.tsx'
import ConfigYamlPage from '@/pages/config-yaml-page.tsx'

const ConfigDetail = () => {
  const [updated, setUpdated] = useState(false)

  const { data, isLoading, mutate } = useConfig()
  const { data: pvcs } = useConfigPVC()
  const [state, actions] = useUpdateConfig()
  const [configData, setConfigData] = useState('')
  const { data: diffPods, mutate: diffMutate } = useConfigDiff('', '')
  const [diff, setDiff] = useState(false)
  const [error, setError] = useState('')
  const navigate = useNavigate()
  const [edit, setEdit] = useState(false)

  useEffect(() => {
    setConfigData(data?.data?.['config.yaml'] || '')
  }, [data])

  useEffect(() => {
    if (diffPods && diffPods.pods.length > 0) {
      setDiff(true)
    }
  }, [diffPods])

  useEffect(() => {
    if (!updated) {
      diffMutate()
      mutate()
    }
  }, [diffMutate, mutate, updated])

  const items: TabsProps['items'] = [
    {
      key: '1',
      label: 'Detail',
      children: (
        <ConfigTablePage
          configData={configData}
          setConfigData={setConfigData}
          setUpdate={setUpdated}
          pvcs={pvcs}
          edit={edit}
        />
      ),
    },
    {
      key: '2',
      label: 'Yaml',
      children: (
        <ConfigYamlPage
          error={error}
          setError={setError}
          setUpdated={setUpdated}
          setConfigData={setConfigData}
          configData={configData}
          edit={edit}
        />
      ),
    },
  ]

  return (
    <PageContainer
      fixedHeader
      className="config-page-header"
      header={{
        title: <FormattedMessage id="config" />,
        ghost: true,
      }}
      extra={[
        <Button
          key="docs"
          onClick={() => {
            window.open(
              'https://juicefs.com/docs/zh/csi/guide/configurations',
              '_blank',
            )
          }}
        >
          <FormattedMessage id="docs" />
        </Button>,
        edit ? (
          <Button
            key="finish docs"
            loading={isLoading}
            disabled={!edit}
            onClick={() => {
              setEdit(false)
            }}
          >
            <FormattedMessage id="finish" />
          </Button>
        ) : (
          <Button
            key="edit docs"
            loading={isLoading}
            disabled={edit}
            onClick={() => {
              setEdit(true)
            }}
          >
            <FormattedMessage id="edit" />
          </Button>
        ),
        updated ? (
          <Button
            key="reset docs"
            loading={isLoading}
            disabled={!updated}
            onClick={() => {
              mutate()
              if (configData) {
                try {
                  setConfigData(data?.data?.['config.yaml'] || '')
                } catch (e) {
                  setConfigData((e as YAMLParseError).message)
                }
                setUpdated(false)
              }
            }}
          >
            <FormattedMessage id="reset" />
          </Button>
        ) : null,
        <Button
          key="update docs"
          type="primary"
          disabled={!updated}
          loading={state.status === 'loading'}
          onClick={() => {
            try {
              YAML.stringify(YAML.parse(configData))
              actions
                .execute({
                  ...data,
                  data: {
                    'config.yaml': configData || '',
                  },
                })
                .catch((error) => {
                  setError(error.toString())
                })
                .then(() => {
                  setUpdated(false)
                  setEdit(false)
                })
            } catch (e) {
              setConfigData((e as YAMLParseError).message)
            }
          }}
        >
          <FormattedMessage id="save" />
        </Button>,

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
    >
      <Tabs items={items} />
    </PageContainer>
  )
}

export default ConfigDetail
