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

import React from 'react'
import { ProCard } from '@ant-design/pro-components'
import { Table } from 'antd'
import { Badge } from 'antd/lib'
import {
  PersistentVolume,
  PersistentVolumeClaim,
  VolumeMount,
} from 'kubernetes-types/core/v1'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'

import { usePods, usePVCsOfPod, usePVsOfPod } from '@/hooks/use-api'
import { Pod } from '@/types/k8s.ts'
import { getPodStatusBadge, getPVCStatusBadge, podStatus } from '@/utils'

interface VolumeMountType {
  pvc?: PersistentVolumeClaim
  pv?: PersistentVolume
  volumeMount?: VolumeMount
  mountPod?: Pod
}

const VolumeMountsTable: React.FC<{
  title: string
  pod: Pod
}> = ({ title, pod }) => {
  const { data } = usePods(
    pod.metadata?.namespace,
    pod.metadata?.name,
    'pod',
    'mountpods',
  )
  const { data: pvcs } = usePVCsOfPod(
    pod.metadata?.namespace,
    pod.metadata?.name,
  )
  const { data: pvs } = usePVsOfPod(pod.metadata?.namespace, pod.metadata?.name)

  if (!data || data.length === 0) {
    return null
  }

  const dataSource = (): VolumeMountType[] => {
    const sources: VolumeMountType[] = []
    const volumeMounts: Map<string, VolumeMount> = new Map()
    pod.spec?.containers.forEach((container) => {
      container.volumeMounts?.forEach((volumeMount) => {
        volumeMounts.set(volumeMount.name, volumeMount)
      })
    })
    const pvcMap: Map<string, PersistentVolumeClaim> = new Map()
    const pvMap: Map<string, PersistentVolume> = new Map()
    pvcs?.forEach((pvc) => {
      if (pvc.metadata && pvc.metadata.name) {
        pvcMap.set(pvc.metadata?.name, pvc)
      }
    })
    pvs?.forEach((pv) => {
      if (pv.spec && pv.spec.claimRef && pv.spec.claimRef.name) {
        pvMap.set(pv.spec.claimRef.name, pv)
      }
    })
    const mountPods: Map<string, Pod> = new Map()
    data.forEach((pod) => {
      if (
        pod.metadata &&
        pod.metadata.labels &&
        pod.metadata.labels['volume-id']
      ) {
        mountPods.set(pod.metadata.labels['volume-id'], pod)
      }
    })

    pod.spec?.volumes?.forEach((volume) => {
      if (volume.persistentVolumeClaim) {
        const volumeMount = volumeMounts.get(volume.name)
        const pvc = pvcMap.get(volume.persistentVolumeClaim.claimName)
        if (pvc) {
          const pv = pvMap.get(pvc.metadata?.name || '')
          const uniqueId = pv?.spec?.csi?.volumeHandle || ''
          let mountPod = mountPods.get(uniqueId)
          if (!mountPod) {
            mountPod = mountPods.get(pvc?.spec?.storageClassName || '')
          }
          sources.push({
            pvc: pvc,
            pv: pv,
            volumeMount: volumeMount,
            mountPod: mountPod,
          })
        }
      }
    })

    return sources
  }

  return (
    <ProCard title={title}>
      <Table
        columns={[
          {
            title: 'PVC',
            render: (_, record) => {
              if (!record.pvc) {
                return '-'
              }
              return (
                <Badge
                  color={getPVCStatusBadge(record.pvc || '')}
                  text={
                    <Link
                      to={`/pvcs/${record.pvc.metadata?.namespace}/${record.pvc.metadata?.name}`}
                    >
                      {record.pvc.metadata?.namespace}/
                      {record.pvc.metadata?.name}
                    </Link>
                  }
                />
              )
            },
          },
          {
            title: <FormattedMessage id="containerPath" />,
            render: (_, record) => {
              return record.volumeMount?.mountPath
            },
          },
          {
            title: 'subPath',
            render: (_, record) => {
              return record.volumeMount?.subPath || '-'
            },
          },
          {
            title: 'Mount Pod',
            render: (_, record) => {
              if (!record.mountPod) {
                return <span>-</span>
              } else {
                const mountPod = record.mountPod
                return (
                  <Badge
                    color={getPodStatusBadge(podStatus(mountPod) || '')}
                    text={
                      <Link
                        to={`/syspods/${mountPod?.metadata?.namespace}/${mountPod?.metadata?.name}/`}
                      >
                        {mountPod?.metadata?.name}
                      </Link>
                    }
                  />
                )
              }
            },
          },
        ]}
        dataSource={dataSource()}
        rowKey={(c) => c.volumeMount?.name || ''}
        pagination={false}
      />
    </ProCard>
  )
}

export default VolumeMountsTable
