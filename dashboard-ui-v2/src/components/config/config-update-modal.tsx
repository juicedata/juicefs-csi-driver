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

import React, { useEffect, useState } from 'react'
import { ProCard } from '@ant-design/pro-components'
import { Button, Collapse, Modal } from 'antd'
import { ConfigMap } from 'kubernetes-types/core/v1'
import ReactDiffViewer from 'react-diff-viewer'
import { FormattedMessage } from 'react-intl'
import YAML, { YAMLParseError } from 'yaml'

import PVCWithSelector from '@/components/config/pvc-with-selector.tsx'
import { useConfigPVCSelector, useUpdateConfig } from '@/hooks/cm-api.ts'
import { PvcPop } from '@/pages/config-table-page.tsx'
import { OriginConfig, PVCWithPod } from '@/types/k8s.ts'

const ConfigUpdateConfirmModal: React.FC<{
  modalOpen: boolean
  onOk: () => void
  onCancel: () => void
  setError: (message: string) => void
  setUpdated: (updated: boolean) => void
  setEdit: (edit: boolean) => void
  data?: ConfigMap
  configData: string
}> = (props) => {
  const {
    modalOpen,
    onOk,
    onCancel,
    setError,
    setUpdated,
    setEdit,
    data,
    configData,
  } = props
  const [state, actions] = useUpdateConfig()

  const [, selectorActions] = useConfigPVCSelector()
  const [pvcs, setPVCs] = useState<PVCWithPod[][]>([])

  const [oldConfig, setOldConfig] = useState<OriginConfig>()
  const [newConfig, setNewConfig] = useState<OriginConfig>()

  useEffect(() => {
    if (modalOpen) {
      try {
        setOldConfig(
          YAML.parse(data?.data?.['config.yaml'] || '') as OriginConfig,
        )
        setNewConfig(YAML.parse(configData) as OriginConfig)
        selectorActions
          .execute({
            data: {
              'config.yaml': configData || '',
            },
          })
          .then((data: PVCWithPod[][]) => {
            setPVCs(data)
          })
          .catch((error) => {
            setError(error.toString())
          })
      } catch (e) {
        setError((e as YAMLParseError).message)
      }
    }
  }, [modalOpen, data, setError, configData, selectorActions])

  const handleSave = () => {
    try {
      YAML.stringify(YAML.parse(configData) as OriginConfig)
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
          setEdit(false)
          setUpdated(false)
          onOk()
        })
    } catch (e) {
      setError((e as YAMLParseError).message)
    }
  }

  const configDiff = (
    oldConfig?: OriginConfig,
    newConfig?: OriginConfig,
    pvcs?: PVCWithPod[][],
  ) => {
    if (!oldConfig || !newConfig || !pvcs) {
      return []
    }
    const result = []
    let j = 0
    for (let i = 0; i < pvcs.length; i++) {
      const oldData = oldConfig.mountPodPatch
        ? oldConfig.mountPodPatch[i]
        : undefined
      const newData = newConfig.mountPodPatch
        ? newConfig.mountPodPatch[i]
        : undefined
      if (YAML.stringify(oldData) !== YAML.stringify(newData)) {
        result.push({
          key: j,
          label: PvcPop(i, pvcs),
          children: (
            <>
              <ProCard title={<FormattedMessage id="updatePatch" />}>
                <ReactDiffViewer
                  oldValue={YAML.stringify(oldData)}
                  newValue={YAML.stringify(newData)}
                  splitView={true}
                />
              </ProCard>
              <PVCWithSelector pvcs={pvcs[i]} />
            </>
          ),
        })
        j++
      }
    }
    return result
  }

  return (
    <Modal
      title={<FormattedMessage id="configUpdateConfirm" />}
      open={modalOpen}
      onOk={onOk}
      onCancel={onCancel}
      className="batch-upgrade-modal"
      footer={() => (
        <>
          <Button type="default" key="back" onClick={onCancel}>
            <FormattedMessage id="backToUpdate" />
          </Button>
          <Button
            type="primary"
            key="save"
            loading={state.status === 'loading'}
            onClick={handleSave}
          >
            <FormattedMessage id="save" />
          </Button>
        </>
      )}
    >
      <Collapse
        key={'config-update'}
        style={{ marginBottom: 16 }}
        defaultActiveKey={[0]}
        items={configDiff(oldConfig, newConfig, pvcs)}
      />
    </Modal>
  )
}

export default ConfigUpdateConfirmModal
