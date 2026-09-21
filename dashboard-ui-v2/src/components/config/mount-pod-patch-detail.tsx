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

import React from 'react'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { Checkbox } from 'antd'
import { FormattedMessage } from 'react-intl'

import PVCWithSelector from '@/components/config/pvc-with-selector.tsx'
import { KeyValue, mountPodPatch } from '@/types/config.ts'
import { MountPatchCacheDir, PVCWithPod } from '@/types/k8s.ts'

const describeEnv = (env: NonNullable<mountPodPatch['env']>[number]) => {
  if (env.valueFrom?.configMapKeyRef) {
    const { name, key } = env.valueFrom.configMapKeyRef
    return ['ConfigMap', name, key].filter(Boolean).join('/')
  }
  if (env.valueFrom?.secretKeyRef) {
    const { name, key } = env.valueFrom.secretKeyRef
    return ['Secret', name, key].filter(Boolean).join('/')
  }
  if (env.valueFrom?.fieldRef) {
    return env.valueFrom.fieldRef.fieldPath ?? ''
  }
  return env.value ?? ''
}

const describeCacheDir = (cacheDir: MountPatchCacheDir) => {
  switch (cacheDir.type) {
    case 'HostPath':
      return cacheDir.path || '""'
    case 'PVC':
      return cacheDir.name || '""'
    case 'EmptyDir': {
      const fields = [
        cacheDir.medium && `medium: ${cacheDir.medium}`,
        cacheDir.sizeLimit && `sizeLimit: ${cacheDir.sizeLimit}`,
      ].filter(Boolean)
      return fields.join(', ') || '{}'
    }
    default:
      return '{}'
  }
}

const MountPodPatchDetail: React.FC<{
  patch: mountPodPatch
  pvcs?: PVCWithPod[]
}> = (props) => {
  const { patch, pvcs } = props
  const kvDescribe = (kv?: KeyValue[]) => {
    if (!kv || !kv.length) return null
    return (
      <div>
        {kv.map((value, index) => (
          <div key={index} className="inlinecode">
            {value.key}: {value.value}
          </div>
        ))}
      </div>
    )
  }

  return (
    <>
      {patch.pvcSelector && (
        <ProCard title={<FormattedMessage id="selector" />}>
          <ProDescriptions
            column={3}
            dataSource={patch.pvcSelector}
            columns={[
              {
                title: <FormattedMessage id="pvcLabelMatch" />,
                key: 'pvcLabelMatch',
                render: () => kvDescribe(patch.pvcSelector?.matchLabels) || '-',
              },
              {
                title: <FormattedMessage id="pvcName" />,
                key: 'pvcName',
                dataIndex: ['matchName'],
              },
              {
                title: <FormattedMessage id="scName" />,
                key: 'scName',
                dataIndex: ['matchStorageClassName'],
              },
            ]}
          />
        </ProCard>
      )}

      <ProCard title={<FormattedMessage id="basicPatch" />}>
        <ProDescriptions
          column={2}
          dataSource={patch}
          columns={[
            {
              title: <FormattedMessage id="ceImage" />,
              key: 'ceMountImage',
              render: () =>
                patch.ceMountImage ? (
                  <span className="inlinecode">{patch.ceMountImage}</span>
                ) : (
                  '-'
                ),
            },
            {
              title: <FormattedMessage id="eeImage" />,
              key: 'eeMountImage',
              render: () =>
                patch.eeMountImage ? (
                  <span className="inlinecode">{patch.eeMountImage}</span>
                ) : (
                  '-'
                ),
            },
            {
              title: <FormattedMessage id="labels" />,
              key: 'labels',
              render: () => kvDescribe(patch.labels) || '-',
            },
            {
              title: <FormattedMessage id="annotations" />,
              key: 'annotations',
              render: () => kvDescribe(patch.annotations) || '-',
            },
            {
              title: <FormattedMessage id="mountOptions" />,
              key: 'mountOptions',
              render: () => {
                if (!patch.mountOptions || !patch.mountOptions.length) {
                  return '-'
                }
                return (
                  <div>
                    {patch.mountOptions?.map((value, index) => (
                      <div key={index} className="inlinecode">
                        {value.value}
                      </div>
                    )) || '-'}
                  </div>
                )
              },
            },
            {
              title: <FormattedMessage id="envs" />,
              key: 'envs',
              render: () => {
                if (!patch.env || !patch.env.length) {
                  return '-'
                }
                return (
                  <div>
                    {patch.env?.map((value, index) => (
                      <div key={index} className="inlinecode">
                        {value.name}: {describeEnv(value)}
                      </div>
                    )) || '-'}
                  </div>
                )
              },
            },
            {
              title: <FormattedMessage id="resourceRequests" />,
              key: 'resourceRequests',
              render: () =>
                patch.resources?.requests ? (
                  <div className="config-detail-item-container">
                    {patch.resources.requests.cpu && (
                      <span>
                        <FormattedMessage id="cpu" />:{' '}
                        <span className="inlinecode">
                          {' '}
                          {patch.resources?.requests?.cpu}{' '}
                        </span>
                      </span>
                    )}
                    {patch.resources.requests.memory && (
                      <span>
                        <FormattedMessage id="memory" />:{' '}
                        <span className="inlinecode">
                          {' '}
                          {patch.resources?.requests?.memory || '-'}{' '}
                        </span>
                      </span>
                    )}
                  </div>
                ) : (
                  '-'
                ),
            },
            {
              title: <FormattedMessage id="resourceLimits" />,
              key: 'resourceLimits',
              render: () =>
                patch.resources?.limits ? (
                  <div className="config-detail-item-container">
                    {patch.resources.limits.cpu && (
                      <span>
                        <FormattedMessage id="cpu" />:{' '}
                        <span className="inlinecode">
                          {' '}
                          {patch.resources?.limits?.cpu || '-'}{' '}
                        </span>
                      </span>
                    )}
                    {patch.resources.limits.memory && (
                      <span>
                        <FormattedMessage id="memory" />:{' '}
                        <span className="inlinecode">
                          {' '}
                          {patch.resources?.limits?.memory || '-'}{' '}
                        </span>
                      </span>
                    )}
                  </div>
                ) : (
                  '-'
                ),
            },
            {
              title: 'hostNetwork',
              key: 'hostNetwork',
              render: () =>
                patch.hostNetwork !== undefined ? (
                  <Checkbox checked={patch.hostNetwork} />
                ) : (
                  '-'
                ),
            },
            {
              title: 'hostPID',
              key: 'hostPID',
              render: () =>
                patch.hostPID !== undefined ? (
                  <Checkbox checked={patch.hostPID} />
                ) : (
                  '-'
                ),
            },
            {
              title: 'terminationGracePeriodSeconds',
              key: 'terminationGracePeriodSeconds',
              render: () =>
                patch.terminationGracePeriodSeconds !== undefined ? (
                  <span className="inlinecode">
                    {patch.terminationGracePeriodSeconds}
                  </span>
                ) : (
                  '-'
                ),
            },
            {
              title: <FormattedMessage id="cacheDir" />,
              key: 'cache',
              render: () => {
                return patch.cacheDirs && patch.cacheDirs.length != 0 ? (
                  <div>
                    {patch.cacheDirs?.map((value, index) => {
                      return (
                        <div
                          key={index}
                          className="config-detail-item-container"
                        >
                          <span style={{ marginBottom: '6px' }}>
                            {value.type}:{' '}
                            <span className="inlinecode">
                              {describeCacheDir(value)}
                            </span>
                          </span>
                        </div>
                      )
                    })}
                  </div>
                ) : (
                  '-'
                )
              },
            },
          ]}
        />
      </ProCard>

      <PVCWithSelector pvcs={pvcs} />
    </>
  )
}

export default MountPodPatchDetail
