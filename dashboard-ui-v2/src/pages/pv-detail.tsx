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
import { PageContainer } from '@ant-design/pro-components'

import { EventTable, PodsTable, PVBasic } from '@/components'
import { usePV } from '@/hooks/pv-api'

const PVDetail: React.FC<{
  name?: string
}> = ({ name }) => {
  const { data, isLoading } = usePV(name)

  if (!data) {
    return null
  }

  return (
    <PageContainer
      loading={!data || isLoading}
      fixedHeader
      header={{
        title: data?.metadata?.name,
      }}
    >
      <PVBasic pv={data} />
      <PodsTable title="Mount Pods" source="pv" type="mountpods" name={name!} />
      <EventTable source="pv" name={name!} />
    </PageContainer>
  )
}

export default PVDetail
