import { useParams } from 'react-router-dom'

import { DetailParams } from '@/types'

export default function ResourcesDetail() {
  const { resources, namespace, name } = useParams<DetailParams>()

  return (
    <>
      hi {resources}/{namespace}/{name}
    </>
  )
}
