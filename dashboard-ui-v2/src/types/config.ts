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

import {
  MountPatchCacheDir,
  OriginConfig,
  OriginPVCSelector,
} from '@/types/k8s.ts'

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
  matchExpressions?: Array<LabelSelectorRequirement>
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
  const convertMap = (input?: {
    [name: string]: string
  }): KeyValue[] | undefined => {
    if (!input) {
      return []
    }

    const output: KeyValue[] = []
    for (const key in input) {
      const value = input[key]
      output.push({
        key: key,
        value: value,
      })
    }

    return output
  }

  const convertOptions = (input?: string[]): KeyValue[] | undefined => {
    if (!input) {
      return []
    }

    const output: KeyValue[] = []
    input.forEach((option) => {
      if (option) {
        output.push({
          key: option,
          value: option,
        })
      }
    })

    return output
  }

  const convertEnvs = (
    input?: EnvVar[],
  ): (EnvVar & { valueType?: string })[] | undefined => {
    if (!input) {
      return []
    }

    const output: (EnvVar & { valueType?: string })[] = []
    input.forEach((env) => {
      if (env && env.name) {
        let valueType = 'value'
        if (env.valueFrom?.configMapKeyRef) {
          valueType = 'configMapKeyRef'
        } else if (env.valueFrom?.secretKeyRef) {
          valueType = 'secretKeyRef'
        } else if (env.valueFrom?.fieldRef) {
          valueType = 'fieldRef'
        }
        output.push({ ...env, valueType })
      }
    })

    return output
  }

  const convertPVCSelector = (
    input?: OriginPVCSelector,
  ): pvcSelector | undefined => {
    if (!input) {
      return {}
    }

    const output: pvcSelector = {}
    if (input.matchLabels) {
      output.matchLabels = convertMap(input.matchLabels)
    }
    if (input.matchExpressions) {
      output.matchExpressions = input.matchExpressions
    }
    if (input.matchName) {
      output.matchName = input.matchName
    }
    if (input.matchStorageClassName) {
      output.matchStorageClassName = input.matchStorageClassName
    }
    return output
  }

  const convertCacheDir = (
    input?: MountPatchCacheDir[] | undefined,
  ): MountPatchCacheDir[] | undefined => {
    if (!input) {
      return []
    }
    return input
  }

  if (!originConfig) {
    return {}
  }

  return {
    enableNodeSelector: originConfig.enableNodeSelector,
    mountPodPatches: originConfig.mountPodPatch
      ? originConfig.mountPodPatch?.map((patch) => {
          return {
            ...patch,
            pvcSelector: convertPVCSelector(patch.pvcSelector),
            labels: convertMap(patch.labels),
            annotations: convertMap(patch.annotations),
            mountOptions: convertOptions(patch.mountOptions),
            env: convertEnvs(patch.env),
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
            cacheDirs: convertCacheDir(patch.cacheDirs),
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
    if (!input || input.length === 0) {
      return undefined
    }
    const output: string[] = []
    input.forEach((value) => {
      if (value.value) {
        output.push(value.value)
      }
    })
    return output.length > 0 ? output : undefined
  }

  const convertKeyValue = (
    input?: KeyValue[],
  ): { [name: string]: string } | undefined => {
    if (!input || input.length === 0) {
      return undefined
    }
    const output: { [key: string]: string } = {}
    input.forEach((value) => {
      if (value.value) {
        output[value.key] = value.value
      }
    })
    return Object.keys(output).length > 0 ? output : undefined
  }

  const convertEnvs = (
    envs?: (EnvVar & { valueType?: string })[],
  ): EnvVar[] | undefined => {
    if (!envs || !envs.length) return undefined

    const output: EnvVar[] = []
    envs.forEach((env) => {
      if (env.name) {
        const { valueType, ...envWithoutValueType } = env
        if (valueType === 'value' || !valueType) {
          delete envWithoutValueType.valueFrom
          output.push(envWithoutValueType)
        } else {
          delete envWithoutValueType.value
          output.push(envWithoutValueType)
        }
      }
    })
    return output.length > 0 ? output : undefined
  }

  const convertPVCSelector = (
    input?: pvcSelector,
  ): OriginPVCSelector | undefined => {
    if (!input) {
      return undefined
    }

    const output: OriginPVCSelector = {}
    let noMatch = true
    if (input.matchLabels) {
      output.matchLabels = convertKeyValue(input.matchLabels)
      noMatch = output.matchLabels === undefined
    }
    if (input.matchExpressions) {
      output.matchExpressions = input.matchExpressions
      noMatch = output.matchExpressions === undefined
    }
    if (input.matchName) {
      output.matchName = input.matchName
      noMatch = false
    }
    if (input.matchStorageClassName) {
      output.matchStorageClassName = input.matchStorageClassName
      noMatch = false
    }
    return noMatch ? undefined : output
  }

  const convertCacheDir = (
    input?: MountPatchCacheDir[] | undefined,
  ): MountPatchCacheDir[] | undefined => {
    if (!input || input.length === 0) {
      return undefined
    }
    return Object.keys(input).length > 0 ? input : undefined
  }

  return {
    enableNodeSelector: config.enableNodeSelector,
    mountPodPatch: config.mountPodPatches
      ? config.mountPodPatches?.map((patch) => {
          return {
            ...patch,
            pvcSelector: convertPVCSelector(patch.pvcSelector),
            labels: convertKeyValue(patch.labels),
            annotations: convertKeyValue(patch.annotations),
            mountOptions: convertMountOptions(patch.mountOptions),
            env: convertEnvs(patch.env),
            resources: patch.resources
              ? {
                  requests: convertResource(patch.resources.requests),
                  limits: convertResource(patch.resources.limits),
                }
              : undefined,
            cacheDirs: convertCacheDir(patch.cacheDirs),
          }
        })
      : undefined,
  }
}
