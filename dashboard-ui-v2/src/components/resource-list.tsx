import { useParams } from 'react-router-dom'

import PodList from '@/pages/app-pod-list'
import PVList from '@/pages/pv-list'
import PVCList from '@/pages/pvc-list'
import SCList from '@/pages/sc-list'
import SysPodList from '@/pages/sys-pod-list'
import { Params } from '@/types'

export default function ResourcesList() {
  const { resources } = useParams<Params>()

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
    default:
      return <div>Not Found</div>
  }
}
