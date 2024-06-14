export type Params = {
  resources: 'pods' | 'mountpods' | 'syspods' | 'pvs' | 'pvcs' | 'storageclass'
}

export type DetailParams = {
  namespace: string
  name: string
} & Params

export type SortOrder = 'descend' | 'ascend' | null

export interface AppPagingListArgs {
  pageSize?: number
  current?: number
  namespace?: string
  name?: string
  pv?: string
  mountPod?: string
  csiNode?: string
  sort?: Record<string, SortOrder>
  filter?: Record<string, (string | number)[] | null>
}

export interface SysPagingListArgs {
  pageSize?: number
  current?: number
  namespace?: string
  name?: string
  node?: string
  sort?: Record<string, SortOrder>
  filter?: Record<string, (string | number)[] | null>
}
