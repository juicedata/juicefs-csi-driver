/*
 * Copyright 2026 Juicedata Inc
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

import { useState } from 'react'
import { Button, message, Modal, Space } from 'antd'
import { useIntl } from 'react-intl'
import { useNavigate } from 'react-router-dom'

import StorageResourceDrawer, {
  StorageKind,
} from '@/components/storage-resource-drawer'
import { apiFetch } from '@/utils'

type Resource = {
  metadata?: { name?: string; namespace?: string }
  status?: { phase?: string }
}

export default function StorageResourceActions({
  kind,
  resource,
  onSaved,
}: {
  kind: StorageKind
  resource?: Resource
  onSaved: () => Promise<void> | void
}) {
  const intl = useIntl()
  const t = (id: string) => intl.formatMessage({ id })
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [modal, contextHolder] = Modal.useModal()
  const names = { pv: 'PV', pvc: 'PVC', storageclass: 'StorageClass' }
  const lists = { pv: '/pvs', pvc: '/pvcs', storageclass: '/storageclass' }
  const remove = () => {
    modal.confirm({
      title: t('storageDeleteConfirm'),
      content: (
        <>
          {`${names[kind]}: ${resource?.metadata?.namespace ? `${resource.metadata.namespace}/` : ''}${resource?.metadata?.name || ''}`}
          <br />
          {t(`storageDeleteRisk_${kind}`)}
        </>
      ),
      okText: t('warmupDelete'),
      cancelText: t('warmupCancel'),
      okButtonProps: { danger: true },
      onOk: async () => {
        setDeleting(true)
        try {
          const path = `/api/v1/${kind}/${kind === 'pvc' ? `${encodeURIComponent(resource?.metadata?.namespace || '')}/` : ''}${encodeURIComponent(resource?.metadata?.name || '')}/`
          await apiFetch(path, { method: 'DELETE' })
          message.success(t('storageDeleted'))
          navigate(lists[kind])
        } catch (err) {
          const text = (err as Error).message
          try {
            message.error((JSON.parse(text) as { error: string }).error || text)
          } catch {
            message.error(text)
          }
          throw err
        } finally {
          setDeleting(false)
        }
      },
    })
  }
  return (
    <>
      {contextHolder}
      <Space>
        <Button
          type={resource ? 'default' : 'primary'}
          onClick={() => setOpen(true)}
        >
          {t(resource ? 'edit' : 'storageCreate')} {!resource && names[kind]}
        </Button>
        {resource && (
          <Button danger loading={deleting} onClick={remove}>
            {t('warmupDelete')}
          </Button>
        )}
      </Space>
      {open && (
        <StorageResourceDrawer
          kind={kind}
          initial={resource}
          onClose={() => setOpen(false)}
          onSaved={onSaved}
        />
      )}
    </>
  )
}
