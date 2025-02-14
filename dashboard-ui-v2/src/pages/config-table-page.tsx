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
import { Card, Collapse, Popover } from 'antd'
import { FormattedMessage } from 'react-intl'
import YAML from 'yaml'

import MountPodPatchForm from '@/components/config/mount-pod-patch-form.tsx'
import {
  Config,
  pvcSelector,
  ToConfig,
  ToOriginConfig,
} from '@/types/config.ts'
import { OriginConfig, PVCWithPod } from '@/types/k8s.ts'

const ConfigTablePage: React.FC<{
  configData?: string
  setConfigData: (configData: string) => void
  setUpdate: (updated: boolean) => void
  pvcs?: PVCWithPod[][]
}> = (props) => {
  const { configData, setConfigData, setUpdate, pvcs } = props
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
        console.log(e)
      }
    }
  }, [configData])

  return (
    <ProCard>
      <ProForm
        submitter={false}
        onValuesChange={(_, allValues) => {
          const oc = ToOriginConfig(allValues)
          try {
            const ocs = YAML.stringify(oc)
            setConfigData(ocs)
            setUpdate(true)
          } catch (e) {
            console.log(e)
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
          itemRender={({ listDom, action }, { index }) => {
            const key = 'Form' + index
            const items = [
              {
                key: key,
                label: pvcPop(
                  index,
                  config?.mountPodPatches
                    ? config?.mountPodPatches[index]?.pvcSelector
                    : undefined,
                  pvcs,
                ),
                children: (
                  <>
                    {listDom}
                    <Card bordered={false}>
                      <MountPodPatchForm
                        patch={
                          config?.mountPodPatches
                            ? config?.mountPodPatches[index]
                            : undefined
                        }
                        pvcs={pvcs ? pvcs[index] : undefined}
                      />
                    </Card>
                  </>
                ),
                extra: action,
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
    </ProCard>
  )
}

export default ConfigTablePage

const pvcPop = (index: number, pvcSt?: pvcSelector, pvcs?: PVCWithPod[][]) => {
  return (
    <Popover
      placement={'bottomLeft'}
      title={<FormattedMessage id="pvcMatched" />}
      content={
        <>
          {pvcSt ? (
            pvcs && pvcs[index] ? (
              pvcs[index].map((pvc) => (
                <p key={pvc.PVC.metadata?.uid}>{pvc.PVC.metadata?.name}</p>
              ))
            ) : null
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
