import { Pod } from 'kubernetes-types/core/v1'


export type SortOrder = 'descend' | 'ascend' | null;
export interface PagingListArgs {
    pageSize?: number;
    current?: number;
    keyword?: string;
    sort: Record<string, SortOrder>;
    filter: Record<string, (string | number)[] | null>
}
export interface PodListResult {
    data: Pod[];
    success: boolean;
}

export const listAppPods = async (args: PagingListArgs) => {
    return {
        data: [],
        success: true,
    }
}