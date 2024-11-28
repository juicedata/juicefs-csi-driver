/*
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


import { Pod } from 'kubernetes-types/core/v1'
import { ProCard } from '@ant-design/pro-components'
import { Badge } from 'antd/lib'
import { Link } from 'react-router-dom'
import { FormattedMessage } from 'react-intl'

const PodDiff: React.FC<{
  diffPods: [Pod],
  diffStatus: Map<string, string>
}> = (props) => {
  const { diffPods, diffStatus } = props

  return (
    <ProCard
      title={<FormattedMessage id="diffPods" />}
      key="diffPods"
      style={{ marginBlockStart: 4 }}
      gutter={4}
      wrap
    >
      {diffPods?.map(pod =>
        <ProCard key={pod.metadata?.uid || ''} colSpan={6}>
          <Badge status={getUpgradeStatusBadge(diffStatus.get(pod?.metadata?.name || '') || '')}
                 text={
                   <Link to={`/syspods/${pod?.metadata?.namespace}/${pod?.metadata?.name}/`}>
                     {pod?.metadata?.name}
                   </Link>
                 }
          />
        </ProCard>,
      )}
    </ProCard>
  )
}

export default PodDiff
const getUpgradeStatusBadge = (finalStatus: string) => {
  switch (finalStatus) {
    case 'pending':
      return 'default'
    case 'running':
    case 'start':
      return 'processing'
    case 'success':
      return 'success'
    case 'fail':
      return 'error'
    default:
      return 'default'
  }
}
