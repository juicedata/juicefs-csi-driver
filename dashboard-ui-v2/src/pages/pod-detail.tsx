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

import { memo } from 'react'
import { PageContainer } from '@ant-design/pro-components'
import { union } from 'lodash'
import { FormattedMessage } from 'react-intl'

import { Containers, EventTable, PodBasic, PodsTable } from '@/components'
import VolumeMountsTable from '@/components/volumeMounts-table.tsx'
import { useAppPod } from '@/hooks/use-api'

const PodDetail: React.FC<{
  name?: string
  namespace?: string
}> = memo((props) => {
  const { name, namespace } = props
  const { data, isLoading } = useAppPod(namespace, name)
  if (namespace === '' || name === '' || !data) {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="podNotFound" />,
        }}
      ></PageContainer>
    )
  }

  return (
    <PageContainer
      fixedHeader
      loading={isLoading}
      header={{
        title: name,
        subTitle: namespace,
      }}
    >
      <PodBasic pod={data} />
      <Containers
        pod={data}
        containerStatuses={union(
          data.status?.containerStatuses,
          data.status?.initContainerStatuses,
        )}
      />
      <PodsTable
        title="App Pods"
        source="pod"
        type="apppods"
        namespace={namespace!}
        name={name!}
      />
      <VolumeMountsTable title="Volumes" pod={data} />
      <PodsTable
        title="CSI Node Pod"
        source="pod"
        type="csi-nodes"
        namespace={namespace!}
        name={name!}
      />
      <EventTable source="pod" name={name!} namespace={namespace!} />
    </PageContainer>
  )
})

export default PodDetail
