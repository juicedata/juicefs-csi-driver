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

import PodList from '@/pages/app-pod-list'
import BatchJobList from '@/pages/batch-job-list.tsx'
import CgList from '@/pages/cg-list'
import PVList from '@/pages/pv-list'
import PVCList from '@/pages/pvc-list'
import SCList from '@/pages/sc-list'
import SecretList from '@/pages/juicefs-secret-list'
import SysPodList from '@/pages/sys-pod-list'
import SyncList from '@/pages/sync-list'
import WarmupList from '@/pages/warmup-list'
import { useVersion } from '@/hooks/use-version'
import { Params } from '@/types'

export default function ResourcesList() {
  const { resources } = useParams<Params>()
  const { data: version } = useVersion()

  switch (resources) {
    case 'pods':
      return <PodList />
    case 'syspods':
      return <SysPodList />
    case 'pvs':
      return <PVList />
    case 'pvcs':
      return <PVCList />
    case 'storageclass':
      return <SCList />
    case 'secrets':
      return version?.secretManagementEnabled ? <SecretList /> : null
    case 'cachegroups':
      return <CgList />
    case 'warmups':
      return <WarmupList />
    case 'syncs':
      return <SyncList />
    case 'jobs':
      return <BatchJobList />
    default:
      return <div>Not Found</div>
  }
}
