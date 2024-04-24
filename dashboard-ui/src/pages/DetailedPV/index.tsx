/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import { EventTable, getPodTableContent } from '@/pages/DetailedPod'
import { PVStatusEnum } from '@/services/common'
import { getMountPodOfPV, getPV, getPVEvents } from '@/services/pv'
import {
  PageContainer,
  PageLoading,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components'
import { useLocation, useParams, useSearchParams } from '@umijs/max'
import { List } from 'antd'
import {
  Event,
  PersistentVolume,
  Pod as RawPod,
} from 'kubernetes-types/core/v1'
import React, { useEffect, useState } from 'react'
import { FormattedMessage, Link } from 'umi'
import { formatData } from '../utils'

const DetailedPV: React.FC<unknown> = () => {
  const location = useLocation()
  const params = useParams()
  const [searchParams] = useSearchParams()
  const pvName = params['pvName'] || ''
  const format = searchParams.get('raw')
  const [pv, setPV] = useState<PersistentVolume>()
  const [mountpods, setMountPods] = useState<RawPod[]>()
  const [events, setEvents] = useState<Event[]>()
  const pvcNamespace = pv?.spec?.claimRef?.namespace || ''
  const pvcName = pv?.spec?.claimRef?.name || ''

  useEffect(() => {
    getPV(pvName).then(setPV)
    getMountPodOfPV(pvName).then(setMountPods)
  }, [setPV, setMountPods])
  useEffect(() => {
    getPVEvents(pvName).then(setEvents)
  }, [setEvents])

  if (pvName === '') {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="pvNotFound" />,
        }}
      ></PageContainer>
    )
  }

  const getPVTabsContent = (pv: PersistentVolume, pvcNamespace: string) => {
    const p = {
      metadata: pv.metadata,
      spec: pv.spec,
      status: pv.status,
    }

    p.metadata?.managedFields?.forEach((managedField) => {
      managedField.fieldsV1 = undefined
    })
    const accessModeMap: { [key: string]: string } = {
      ReadWriteOnce: 'RWO',
      ReadWriteMany: 'RWX',
      ReadOnlyMany: 'ROX',
      ReadWriteOncePod: 'RWOP',
    }

    let content: any
    let parameters: string[] = []
    const volumeAttributes = pv.spec?.csi?.volumeAttributes
    if (volumeAttributes) {
      for (const key in volumeAttributes) {
        if (volumeAttributes.hasOwnProperty(key)) {
          const value = volumeAttributes[key]
          parameters.push(`${key}: ${value}`)
        }
      }
    }
    content = (
      <div>
        <ProCard title={<FormattedMessage id="basic" />}>
          <ProDescriptions
            column={2}
            dataSource={{
              pvc: `${pvcNamespace}/${pvcName}`,
              capacity: pv.spec?.capacity?.storage,
              accessMode: pv.spec?.accessModes
                ?.map((accessMode) => accessModeMap[accessMode] || 'Unknown')
                .join(','),
              reclaimPolicy: pv.spec?.persistentVolumeReclaimPolicy,
              storageClass: pv.spec?.storageClassName,
              volumeHandle: pv.spec?.csi?.volumeHandle,
              status: pv.status?.phase,
              time: pv.metadata?.creationTimestamp,
            }}
            columns={[
              {
                title: 'PVC',
                key: 'pvc',
                render: (_, record) => {
                  if (record.pvc === '/') {
                    return '-'
                  }
                  const [namespace, name] = record.pvc.split('/')
                  return (
                    <Link to={`/pvc/${namespace}/${name}`}>{record.pvc}</Link>
                  )
                },
              },
              {
                title: <FormattedMessage id="capacity" />,
                key: 'capacity',
                dataIndex: 'capacity',
              },
              {
                title: <FormattedMessage id="accessMode" />,
                key: 'accessMode',
                dataIndex: 'accessMode',
              },
              {
                title: <FormattedMessage id="reclaimPolicy" />,
                key: 'reclaimPolicy',
                dataIndex: 'reclaimPolicy',
              },
              {
                title: 'StorageClass',
                key: 'storageClass',
                render: (_, record) => {
                  if (!record.storageClass) {
                    return '-'
                  }
                  return (
                    <Link to={`/storageclass/${record.storageClass}`}>
                      {record.storageClass}
                    </Link>
                  )
                },
              },
              {
                title: 'volumeHandle',
                key: 'volumeHandle',
                dataIndex: 'volumeHandle',
              },
              {
                title: <FormattedMessage id="status" />,
                key: 'status',
                dataIndex: 'status',
                valueType: 'select',
                valueEnum: PVStatusEnum,
              },
              {
                title: <FormattedMessage id="createAt" />,
                key: 'time',
                dataIndex: 'time',
              },
              {
                title: 'Yaml',
                key: 'yaml',
                render: () => (
                  <Link to={`${location.pathname}?raw=yaml`}>
                    {<FormattedMessage id="clickToView" />}
                  </Link>
                ),
              },
            ]}
          />
        </ProCard>
        <ProCard title={<FormattedMessage id="volumeAttributes" />}>
          <List
            dataSource={parameters}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>
        <ProCard title={<FormattedMessage id="mountOptions" />}>
          <List
            dataSource={pv.spec?.mountOptions}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>

        {getPodTableContent(mountpods || [], 'Mount Pods')}

        {EventTable(events || [])}
      </div>
    )
    return content
  }

  let contents
  if (!pv) {
    return <PageLoading />
  } else if (
    typeof format === 'string' &&
    (format === 'json' || format === 'yaml')
  ) {
    contents = formatData(pv, format)
  } else {
    contents = (
      <ProCard direction="column">{getPVTabsContent(pv, pvcNamespace)}</ProCard>
    )
  }
  return (
    <PageContainer
      fixedHeader
      header={{
        title: (
          <Link to={`/pv/${pv?.metadata?.name}`}>{pv?.metadata?.name}</Link>
        ),
      }}
    >
      {contents}
    </PageContainer>
  )
}

export default DetailedPV
