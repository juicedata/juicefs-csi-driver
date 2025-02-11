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
import { FormattedMessage } from 'react-intl'

import PVCWithSelector from '@/components/config/pvc-with-selector.tsx'
import { KeyValue, mountPodPatch } from '@/types/config.ts'
import { PVCWithPod } from '@/types/k8s.ts'

const MountPodPatchDetail: React.FC<{
  patch: mountPodPatch
  pvcs?: PVCWithPod[]
}> = (props) => {
  const { patch, pvcs } = props
  const kvDescribe = (kv?: KeyValue[]) => {
    if (!kv || kv.length === 0) return null
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
              render: () => patch.ceMountImage || '-',
            },
            {
              title: <FormattedMessage id="eeImage" />,
              key: 'eeMountImage',
              render: () => patch.eeMountImage || '-',
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
              render: () => (
                <div>
                  {patch.mountOptions?.map((value, index) => (
                    <div key={index} className="inlinecode">
                      {value.value}
                    </div>
                  )) || '-'}
                </div>
              ),
            },
            {
              title: <FormattedMessage id="envs" />,
              key: 'envs',
              render: () => (
                <div>
                  {patch.env?.map((value, index) => (
                    <div key={index} className="inlinecode">
                      {value.name}: {value.value}
                    </div>
                  )) || '-'}
                </div>
              ),
            },
            {
              title: <FormattedMessage id="resourceRequests" />,
              key: 'resourceRequests',
              render: () =>
                patch.resources?.requests ? (
                  <div style={{ display: 'flex', gap: '12px' }}>
                    {patch.resources.requests.cpu && (
                      <span>
                        <FormattedMessage id="cpu" />:{' '}
                        {patch.resources?.requests?.cpu}
                      </span>
                    )}
                    {patch.resources.requests.memory && (
                      <span>
                        <FormattedMessage id="memory" />:{' '}
                        {patch.resources?.requests?.memory || '-'}
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
                  <div style={{ display: 'flex', gap: '12px' }}>
                    {patch.resources.limits.cpu && (
                      <span>
                        <FormattedMessage id="cpu" />:{' '}
                        {patch.resources?.limits?.cpu || '-'}
                      </span>
                    )}
                    {patch.resources.limits.memory && (
                      <span>
                        <FormattedMessage id="memory" />:{' '}
                        {patch.resources?.limits?.memory || '-'}
                      </span>
                    )}
                  </div>
                ) : (
                  '-'
                ),
            },
          ]}
        />
      </ProCard>

      <PVCWithSelector pvcSelector={patch.pvcSelector} pvcs={pvcs} />
    </>
  )
}

export default MountPodPatchDetail
