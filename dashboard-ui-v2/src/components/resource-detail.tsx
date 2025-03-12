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

import { useParams } from 'react-router-dom'

import BatchUpgradeJobDetail from '@/pages/batch-upgrade-job-detail.tsx'
import CgDetail from '@/pages/cg-detail'
import PodDetail from '@/pages/pod-detail'
import PVDetail from '@/pages/pv-detail'
import PVCDetail from '@/pages/pvc-detail'
import SCDetail from '@/pages/sc-detail.tsx'
import { DetailParams } from '@/types'

export default function ResourcesDetail() {
  const { resources, namespace, name } = useParams<DetailParams>()

  switch (resources) {
    case 'syspods':
    case 'pods':
      return <PodDetail namespace={namespace} name={name} />
    case 'storageclass':
      return <SCDetail name={name} />
    case 'pvs':
      return <PVDetail name={name} />
    case 'pvcs':
      return <PVCDetail name={name} namespace={namespace} />
    case 'cachegroups':
      return <CgDetail name={name} namespace={namespace} />
    case 'jobs':
      return <BatchUpgradeJobDetail jobName={name} />
    default:
      return <div>Not Found</div>
  }
}
