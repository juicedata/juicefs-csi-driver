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

import useSWR from 'swr'

export interface VersionResponse {
  version: string
  fullImage?: string
}

export function useVersion() {
  return useSWR<VersionResponse>('/api/v1/version', {
    // Cache for 5 minutes to reduce unnecessary requests
    refreshInterval: 5 * 60 * 1000,
    // Don't revalidate on window focus since version doesn't change frequently
    revalidateOnFocus: false,
  })
}