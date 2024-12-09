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

import React, { useState } from 'react'
import { PageContainer, ProColumns, ProTable } from '@ant-design/pro-components'
import { Button, message } from 'antd'
import { FormattedMessage } from 'react-intl'
import { Link } from 'react-router-dom'
import YAML from 'yaml'

import { YamlModal } from '@/components'
import { useCacheGroups, useCreateCacheGroup } from '@/hooks/cg-api'
import { CacheGroup } from '@/types/k8s'

const defaultCreateCgTemplate = `
## This is a default template for creating a cache group
## You can modify it to fit your needs
## For more information, please refer to the official documentation

apiVersion: juicefs.io/v1
kind: CacheGroup
metadata:
  name: cachegroup-demo
  namespace: default
spec:
  secretRef:
    name: juicefs-secret
  worker:
    template:
      nodeSelector:
        juicefs.io/cg-worker: "true"
      image: juicedata/mount:ee-5.1.2-59d9736
      dnsPolicy: ClusterFirstWithHostNet
      hostNetwork: true
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
        limits:
          cpu: 1
          memory: 1Gi
`

const columns: ProColumns<CacheGroup>[] = [
  {
    title: <FormattedMessage id="name" />,
    dataIndex: ['metadata', 'name'],
    render: (_, cg) => (
      <Link to={`/cachegroups/${cg.metadata?.namespace}/${cg.metadata?.name}`}>
        {cg.metadata?.name}
      </Link>
    ),
  },
  {
    title: <FormattedMessage id="fileSystem" />,
    dataIndex: ['status', 'fileSystem'],
  },
  {
    title: <FormattedMessage id="phase" />,
    dataIndex: ['status', 'phase'],
  },
  {
    title: <FormattedMessage id="ready" />,
    dataIndex: ['status', 'readyStr'],
  },
  {
    title: <FormattedMessage id="createAt" />,
    key: 'time',
    sorter: 'time',
    search: false,
    render: (_, row) =>
      new Date(row.metadata?.creationTimestamp as string).toLocaleString(),
  },
]

const CgList: React.FC<unknown> = () => {
  const { data, isLoading, mutate } = useCacheGroups()
  const [, createCg] = useCreateCacheGroup()

  const [isModalOpen, setIsModalOpen] = useState(false)

  const showModal = () => {
    setIsModalOpen(true)
  }

  const handleCancel = () => {
    setIsModalOpen(false)
  }

  return (
    <PageContainer
      header={{
        title: 'Cache Groups',
      }}
    >
      <ProTable<CacheGroup>
        rowKey={(record) => record.metadata?.uid || ''}
        loading={isLoading}
        dataSource={data}
        columns={columns}
        search={false}
        toolbar={{
          actions: [
            <>
              <Button key="create" type="primary" onClick={showModal}>
                Create
              </Button>
              <YamlModal
                isOpen={isModalOpen}
                onClose={handleCancel}
                content={defaultCreateCgTemplate}
                editable
                onSave={async (data) => {
                  const resp = await createCg.execute({
                    body: YAML.parse(data),
                  })
                  if (resp.status !== 200) {
                    message.error('error: ' + (await resp.json()).error)
                    return
                  }
                  mutate()
                  handleCancel()
                  message.success('success')
                }}
                saveButtonText="Create"
              />
            </>,
          ],
          settings: undefined,
        }}
      />
    </PageContainer>
  )
}

export default CgList
