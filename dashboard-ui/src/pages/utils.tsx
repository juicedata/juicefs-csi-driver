/*
 Copyright 2023 Juicedata Inc

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
 */

import * as jsyaml from 'js-yaml'
import { Node, PersistentVolume } from 'kubernetes-types/core/v1'
import { ObjectMeta } from 'kubernetes-types/meta/v1'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'

export interface Source {
  metadata?: ObjectMeta
}

export const formatData = (src: Source, format: string) => {
  src.metadata?.managedFields?.forEach((managedField) => {
    managedField.fieldsV1 = undefined
  })
  const data =
    format === 'json' ? JSON.stringify(src, null, '\t') : jsyaml.dump(src)
  return (
    <SyntaxHighlighter language={format} showLineNumbers>
      {data.trim()}
    </SyntaxHighlighter>
  )
}

export const getNodeStatusBadge = (node: Node) => {
  const ready = node.status?.conditions?.find((condition) => {
    if (condition.type === 'Ready' && condition.status === 'True') {
      return true
    }
    return false
  })
  return ready ? 'green' : 'red'
}

export const getPodStatusBadge = (finalStatus: string) => {
  switch (finalStatus) {
    case 'Pending':
    case 'ContainerCreating':
    case 'PodInitializing':
      return 'yellow'
    case 'Running':
      return 'green'
    case 'Succeed':
      return 'blue'
    case 'Failed':
    case 'Error':
      return 'red'
    case 'Unknown':
    case 'Terminating':
    default:
      return 'grey'
  }
}

export const getPVStatusBadge = (pv: PersistentVolume) => {
  if (pv.status === undefined || pv.status.phase === undefined) {
    return 'grey'
  }
  switch (pv.status.phase) {
    case 'Bound':
      return 'green'
    case 'Available':
      return 'blue'
    case 'Pending':
      return 'yellow'
    case 'Failed':
      return 'red'
    case 'Released':
    default:
      return 'grey'
  }
}
