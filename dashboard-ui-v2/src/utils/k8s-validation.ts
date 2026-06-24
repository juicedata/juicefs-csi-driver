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

/**
 * Kubernetes validation helpers shared between config-detail.tsx and
 * mount-pod-patch-form.tsx.
 *
 * NOTE: These rules mirror the backend Config.Validate() in
 * pkg/config/config.go. Keep them in sync when updating validation logic.
 */

// name part: start/end alphanum, middle may contain -, _, .  (max 63 chars)
const k8sKeyRe = /^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?$/
// DNS subdomain prefix: lowercase alphanum + hyphen + dot  (max 253 chars)
const dnsPrefixRe = /^[a-z0-9]([a-z0-9\-.]*[a-z0-9])?$/
// label value: same character set as key name (empty is also valid)
const labelValueRe = /^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?$/
// env var name: printable ASCII except '='
export const envVarNameRe = /^[!-<>-~]+$/
// K8s resource quantity: plain number, SI suffixes or binary suffixes
export const k8sQuantityRe =
  /^(\+|-)?([0-9]+\.?[0-9]*|\.[0-9]+)([eE][+-]?[0-9]+|Ki|Mi|Gi|Ti|Pi|Ei|[numkMGTPE])?$/

export const validDnsPolicies = new Set([
  'ClusterFirst',
  'ClusterFirstWithHostNet',
  'Default',
  'None',
])

export const validCacheDirTypes = new Set([
  'HostPath',
  'PVC',
  'EmptyDir',
  'Ephemeral',
])

/** Returns true if `key` is a valid Kubernetes qualified name ([prefix/]name). */
export function isValidK8sKey(key: string): boolean {
  const parts = key.split('/')
  if (parts.length > 2) return false
  let name = key
  if (parts.length === 2) {
    const prefix = parts[0]
    name = parts[1]
    if (!prefix || prefix.length > 253 || !dnsPrefixRe.test(prefix))
      return false
  }
  return !!name && name.length <= 63 && k8sKeyRe.test(name)
}

/** Returns true if `value` is a valid Kubernetes label value (empty is ok). */
export function isValidLabelValue(value: string): boolean {
  if (!value) return true
  return value.length <= 63 && labelValueRe.test(value)
}

/** Returns true if `v` is a valid Kubernetes resource quantity (empty is ok). */
export function isValidQuantity(v: string): boolean {
  if (!v) return true
  return k8sQuantityRe.test(v.trim())
}

// ---------------------------------------------------------------------------
// Ant Design ProForm async validators
// ---------------------------------------------------------------------------

/** Validates a Kubernetes qualified name key: [prefix/]name */
export const validateK8sKey = (_: unknown, value: string): Promise<void> => {
  if (!value) return Promise.resolve()
  const parts = value.split('/')
  if (parts.length > 2)
    return Promise.reject(new Error('Key must be in format [prefix/]name'))
  let name = value
  if (parts.length === 2) {
    const prefix = parts[0]
    name = parts[1]
    if (!prefix || prefix.length > 253)
      return Promise.reject(
        new Error('Prefix must be a valid DNS subdomain (max 253 chars)'),
      )
    if (!dnsPrefixRe.test(prefix))
      return Promise.reject(
        new Error(
          'Prefix must be a lowercase DNS subdomain (a-z, 0-9, hyphen, dot)',
        ),
      )
  }
  if (!name || name.length > 63)
    return Promise.reject(
      new Error('Name part must be non-empty and at most 63 chars'),
    )
  if (!k8sKeyRe.test(name))
    return Promise.reject(
      new Error(
        'Name must start and end with an alphanumeric character, and may contain -, _, .',
      ),
    )
  return Promise.resolve()
}

/** Validates a Kubernetes label value: empty OR max 63 chars, start/end alphanum */
export const validateLabelValue = (
  _: unknown,
  value: string,
): Promise<void> => {
  if (!value) return Promise.resolve()
  if (value.length > 63)
    return Promise.reject(new Error('Label value must be at most 63 chars'))
  if (!labelValueRe.test(value))
    return Promise.reject(
      new Error(
        'Label value must start and end with an alphanumeric character, and may contain -, _, .',
      ),
    )
  return Promise.resolve()
}

/** Validates a Kubernetes env var name (printable ASCII, no '='). */
export const validateEnvVarName = (
  _: unknown,
  value: string,
): Promise<void> => {
  if (!value) return Promise.resolve()
  if (!envVarNameRe.test(value))
    return Promise.reject(
      new Error(
        "A valid environment variable name must consist only of printable ASCII characters other than '='",
      ),
    )
  return Promise.resolve()
}

/** Validates a CPU resource quantity, e.g. 100m, 500m, 1, 1.5 */
export const validateCPUQuantity = (
  _: unknown,
  value: string,
): Promise<void> => {
  if (!value) return Promise.resolve()
  if (!k8sQuantityRe.test(value.trim()))
    return Promise.reject(
      new Error('Invalid CPU quantity, examples: 100m, 500m, 1, 1.5, 2'),
    )
  return Promise.resolve()
}

/** Validates a memory resource quantity, e.g. 128Mi, 1Gi */
export const validateMemoryQuantity = (
  _: unknown,
  value: string,
): Promise<void> => {
  if (!value) return Promise.resolve()
  if (!k8sQuantityRe.test(value.trim()))
    return Promise.reject(
      new Error(
        'Invalid memory quantity, examples: 128Mi, 512Mi, 1Gi, 2Gi',
      ),
    )
  return Promise.resolve()
}
