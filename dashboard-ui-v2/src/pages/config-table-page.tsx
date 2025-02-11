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
import { Card, Collapse, Popover, Tooltip } from 'antd'
import YAML from 'yaml'

import MountPodPatchDetail from '@/components/config/mount-pod-patch-detail.tsx'
import MountPodPatchForm from '@/components/config/mount-pod-patch-form.tsx'
import { Config, pvcSelector, ToConfig, ToOriginConfig } from '@/types/config.ts'
import { OriginConfig, PVCWithPod } from '@/types/k8s.ts'
import { FormattedMessage } from 'react-intl'

const ConfigTablePage: React.FC<{
  configData?: string
  setConfigData: (configData: string) => void
  setUpdate: (updated: boolean) => void
  pvcs?: PVCWithPod[][]
  edit: boolean
}> = (props) => {
  const { configData, setConfigData, setUpdate, pvcs, edit } = props
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
  }, [configData, edit])

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
            creatorRecord={{
              useMode: 'none',
            }}
            itemRender={({ listDom, action }, { index }) => {
              const items = [
                {
                  key: 'Form' + index,
                  label: pvcPop(index, config?.mountPodPatches ? config?.mountPodPatches[index].pvcSelector: undefined, pvcs),
                  children: <Card bordered={false}>{listDom}</Card>,
                  extra: action,
                },
              ]
              return (
                <Collapse
                  defaultActiveKey={['0']}
                  style={{ marginBottom: 16 }}
                  items={items}
                />
              )
            }}
          >
            <MountPodPatchForm />
          </ProFormList>
        </ProForm>
      ) : (
        <Collapse
          defaultActiveKey={['0']}
          style={{ marginBottom: 16 }}
          items={config?.mountPodPatches?.map((value, index) => {
            return {
              key: index,
              label: pvcPop(index, value.pvcSelector, pvcs),
              children: (
                <MountPodPatchDetail
                  patch={value}
                  pvcs={pvcs ? pvcs[index] : []}
                />
              ),
            }
          })}
        />
      )}
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
          {pvcSt ?
            (pvcs && pvcs[index]) ? pvcs[index].map((pvc) => (
              <p key={pvc.PVC.metadata?.uid}>
                {pvc.PVC.metadata?.name}
              </p>
            )) : null
            : <FormattedMessage id="allPVC" />}
        </>
      }
    >
      <FormattedMessage id="patch" /> {index + 1}
    </Popover>
  )
}
