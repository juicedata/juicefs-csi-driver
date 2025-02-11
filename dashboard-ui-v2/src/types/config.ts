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
import { Quantity } from 'kubernetes-types/api/resource'
import {
  EnvVar,
  Lifecycle,
  Probe,
  Volume,
  VolumeDevice,
  VolumeMount,
} from 'kubernetes-types/core/v1'
import { LabelSelectorRequirement } from 'kubernetes-types/meta/v1'

import { MountPatchCacheDir, OriginConfig } from '@/types/k8s.ts'

export type Config = {
  enableNodeSelector?: boolean
  mountPodPatches?: mountPodPatch[]
}

export type mountPodPatch = {
  pvcSelector?: pvcSelector
} & MountPatch

export type pvcSelector = {
  matchStorageClassName?: string
  matchName?: string
  matchLabels?: KeyValue[]
  matchExpressions?: KeyValueRequirement[]
}

export type KeyValueRequirement = {
  key: string
  operator: string
  values?: KeyValue[]
}

export type MountPatch = {
  ceMountImage?: string
  eeMountImage?: string
  cacheDirs?: MountPatchCacheDir[]
  labels?: KeyValue[]
  annotations?: KeyValue[]
  hostNetwork?: boolean
  hostPID?: boolean
  livenessProbe?: Probe
  readinessProbe?: Probe
  startupProbe?: Probe
  lifecycle?: Lifecycle
  resources?: resource
  terminationGracePeriodSeconds?: number
  volumes?: Volume[]
  volumeDevices?: VolumeDevice[]
  volumeMounts?: VolumeMount[]
  env?: EnvVar[]
  mountOptions?: KeyValue[]
}

export type resource = {
  limits?: {
    cpu?: string
    memory?: string
  }
  requests?: {
    cpu?: string
    memory?: string
  }
}

export type KeyValue = {
  key: string
  value: string
}

export const ToConfig = (originConfig: OriginConfig): Config => {
  return {
    enableNodeSelector: originConfig.enableNodeSelector,
    mountPodPatches: originConfig.mountPodPatch
      ? originConfig.mountPodPatch?.map((patch) => {
          return {
            ...patch,
            pvcSelector: patch.pvcSelector
              ? {
                  ...patch.pvcSelector,
                  matchLabels: patch.pvcSelector.matchLabels
                    ? Object.keys(patch.pvcSelector.matchLabels).map((key) => {
                        return {
                          key: key,
                          value: patch.pvcSelector?.matchLabels![key] || '',
                        }
                      })
                    : undefined,
                  matchExpressions: patch.pvcSelector.matchExpressions
                    ? patch.pvcSelector.matchExpressions.map((key) => {
                        return {
                          key: key.key,
                          operator: key.operator,
                          values: key.values
                            ? key.values.map((key, index) => {
                                return {
                                  key: `${index}`,
                                  value: key,
                                }
                              })
                            : undefined,
                        }
                      })
                    : undefined,
                }
              : undefined,
            labels: patch.labels
              ? Object.keys(patch.labels).map((key) => {
                  return { key: key, value: patch.labels![key] }
                })
              : undefined,
            annotations: patch.annotations
              ? Object.keys(patch.annotations).map((key) => {
                  return { key: key, value: patch.annotations![key] }
                })
              : undefined,
            mountOptions: patch.mountOptions
              ? patch.mountOptions.map((value) => {
                  return { key: value, value: value }
                })
              : undefined,
            resources: patch.resources
              ? {
                  requests: patch.resources.requests
                    ? {
                        cpu: patch.resources.requests!['cpu'],
                        memory: patch.resources.requests!['memory'],
                      }
                    : undefined,
                  limits: patch.resources.limits
                    ? {
                        cpu: patch.resources.limits!['cpu'],
                        memory: patch.resources.limits!['memory'],
                      }
                    : undefined,
                }
              : undefined,
          }
        })
      : undefined,
  }
}

export const ToOriginConfig = (config: Config): OriginConfig => {
  const convertResource = (input?: {
    cpu?: string
    memory?: string
  }): { [key: string]: Quantity } | undefined => {
    if (!input) {
      return undefined
    }

    const output: { [key: string]: Quantity } = {}

    if (input.cpu) {
      output['cpu'] = input.cpu
    }

    if (input.memory) {
      output['memory'] = input.memory
    }

    return output
  }

  const convertMountOptions = (input?: KeyValue[]): string[] | undefined => {
    if (!input) {
      return undefined
    }
    const output: string[] = []
    input.forEach((value) => {
      value.value ? output.push(`${value.value}`) : output.push('')
    })
    return output
  }

  const convertRequirements = (requirements?: KeyValueRequirement[]) => {
    return requirements
      ? (requirements.map((req) => ({
          key: req.key,
          operator: req.operator,
          values: req.values ? req.values.map((v) => v.value) : undefined,
        })) as Array<LabelSelectorRequirement>)
      : undefined
  }

  return {
    enableNodeSelector: config.enableNodeSelector,
    mountPodPatch: config.mountPodPatches
      ? config.mountPodPatches?.map((patch) => {
          return {
            ...patch,
            pvcSelector: patch.pvcSelector
              ? {
                  ...patch.pvcSelector,
                  matchLabels: patch.pvcSelector.matchLabels
                    ? patch.pvcSelector.matchLabels.reduce(
                        (acc, { key, value }) => {
                          acc[key] = value
                          return acc
                        },
                        {} as { [name: string]: string },
                      )
                    : undefined,
                  matchExpressions: convertRequirements(
                    patch.pvcSelector.matchExpressions,
                  ),
                }
              : undefined,
            labels: patch.labels
              ? patch.labels.reduce(
                  (acc, { key, value }) => {
                    acc[key] = value
                    return acc
                  },
                  {} as { [name: string]: string },
                )
              : undefined,
            annotations: patch.annotations
              ? patch.annotations.reduce(
                  (acc, { key, value }) => {
                    acc[key] = value
                    return acc
                  },
                  {} as { [name: string]: string },
                )
              : undefined,
            mountOptions: convertMountOptions(patch.mountOptions),
            resources: patch.resources
              ? {
                  requests: convertResource(patch.resources.requests),
                  limits: convertResource(patch.resources.limits),
                }
              : undefined,
          }
        })
      : undefined,
  }
}
