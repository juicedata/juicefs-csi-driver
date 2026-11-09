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

import { ComponentProps } from 'react'
import { AutoComplete } from 'antd'
import { useIntl } from 'react-intl'
import useSWR from 'swr'

import { useVersion } from '@/hooks/use-version'

type SecretSummary = {
  metadata: { name: string; namespace: string; labels?: Record<string, string> }
}

export default function SecretNameInput({
  namespace,
  allSecrets = false,
  ...props
}: ComponentProps<typeof AutoComplete> & {
  namespace?: string
  allSecrets?: boolean
}) {
  const intl = useIntl()
  const { data: version } = useVersion()
  const enabled = version?.secretManagementEnabled === true
  const { data: juiceSecrets } = useSWR<SecretSummary[]>(
    enabled && !allSecrets && namespace ? '/api/v1/secrets' : null,
  )
  const { data: allNames } = useSWR<string[]>(
    enabled && allSecrets && namespace
      ? `/api/v1/secrets/basic?namespace=${encodeURIComponent(namespace)}`
      : null,
  )
  const names = allSecrets
    ? allNames || []
    : (juiceSecrets || [])
        .filter(
          ({ metadata }) =>
            metadata.namespace === namespace &&
            !Object.prototype.hasOwnProperty.call(
              metadata.labels || {},
              'juicefs/secret',
            ),
        )
        .map(({ metadata }) => metadata.name)

  return (
    <AutoComplete
      {...props}
      placeholder={intl.formatMessage({
        id: enabled ? 'storageSecretSelect' : 'name',
      })}
      options={names.map((name) => ({ value: name }))}
      filterOption={(input, option) =>
        String(option?.value ?? '').toLowerCase().includes(input.toLowerCase())
      }
    />
  )
}
