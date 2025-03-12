/*
 * Copyright 2025 Juicedata Inc
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

import React, { useEffect, useRef, useState } from 'react'
import {
  ProCard,
  ProForm,
  ProFormInstance,
  ProFormList,
} from '@ant-design/pro-components'
import { Collapse, Popover } from 'antd'
import { FormattedMessage } from 'react-intl'
import YAML, { YAMLParseError } from 'yaml'

import MountPodPatchDetail from '@/components/config/mount-pod-patch-detail.tsx'
import MountPodPatchForm from '@/components/config/mount-pod-patch-form.tsx'
import { Config, ToConfig, ToOriginConfig } from '@/types/config.ts'
import { OriginConfig, PVCWithPod } from '@/types/k8s.ts'

const ConfigTablePage: React.FC<{
  configData?: string
  setConfigData: (configData: string) => void
  setUpdate: (updated: boolean) => void
  pvcs?: PVCWithPod[][]
  edit: boolean
  setError: (message: string) => void
}> = (props) => {
  const { configData, setConfigData, setUpdate, pvcs, edit, setError } = props
  const [config, setConfig] = useState<Config>()
  const formRef = useRef<ProFormInstance>()

  useEffect(() => {
    if (configData && configData !== '') {
      try {
        const oc = YAML.parse(configData || '') as OriginConfig
        const c = ToConfig(oc)
        setConfig(c)
        formRef?.current?.setFieldsValue(c)
      } catch (e) {
        setError((e as YAMLParseError).message)
      }
    }
  }, [setError, configData, edit])

  return (
    <ProCard>
      {edit ? (
        <ProForm
          submitter={false}
          onValuesChange={(_, allValues) => {
            const oc = ToOriginConfig(allValues)
            try {
              const ocs = YAML.stringify(oc)
              setConfigData(ocs)
              setUpdate(true)
            } catch (e) {
              setError((e as YAMLParseError).message)
            }
          }}
          formRef={formRef}
          layout={'horizontal'}
          grid={false}
          rowProps={{
            gutter: [16, 0],
          }}
        >
          <ProFormList
            name="mountPodPatches"
            creatorButtonProps={{
              position: 'bottom',
              creatorButtonText: 'New',
            }}
            itemRender={({ action }, { index, record }) => {
              const key = 'Form' + index
              const items = [
                {
                  key: key,
                  label: PvcPop(index, pvcs),
                  children: <MountPodPatchForm patch={record} />,
                  extra: action,
                  headerClass: 'patch-extra',
                },
              ]
              return (
                <Collapse
                  defaultActiveKey={index === 0 ? [key] : []}
                  style={{ marginBottom: 16 }}
                  items={items}
                />
              )
            }}
          ></ProFormList>
        </ProForm>
      ) : (
        <>
          {config?.mountPodPatches?.map((value, index) => (
            <Collapse
              key={index}
              defaultActiveKey={[index]}
              style={{ marginBottom: 16 }}
              items={[
                {
                  key: index,
                  label: PvcPop(index, pvcs),
                  children: (
                    <MountPodPatchDetail
                      patch={value}
                      pvcs={pvcs ? pvcs[index] : []}
                    />
                  ),
                },
              ]}
            />
          ))}
        </>
      )}
    </ProCard>
  )
}

export default ConfigTablePage

export const PvcPop = (index: number, pvcs?: PVCWithPod[][]) => {
  return (
    <Popover
      placement={'bottomLeft'}
      title={<FormattedMessage id="pvcMatched" />}
      content={
        <>
          {pvcs && pvcs[index] ? (
            pvcs[index].map((pvc) => (
              <p key={pvc.PVC.metadata?.uid}>{pvc.PVC.metadata?.name}</p>
            ))
          ) : (
            <FormattedMessage id="allPVC" />
          )}
        </>
      }
    >
      <FormattedMessage id="patch" /> {index + 1}
    </Popover>
  )
}
