import { Pod as RawPod } from 'kubernetes-types/core/v1'

export type Pod = RawPod & {
    mountPods?: RawPod[]
}

export type SortOrder = 'descend' | 'ascend' | null;
export interface PagingListArgs {
    pageSize?: number;
    current?: number;
    keyword?: string;
    sort: Record<string, SortOrder>;
    filter: Record<string, (string | number)[] | null>
}
export interface PodListResult {
    data?: Pod[];
    success: boolean;
}

export const listAppPods = async (args: PagingListArgs) => {
    let data: Pod[]
    try {
        let pods = await fetch('http://localhost:8088/api/v1/pods')
        data = JSON.parse(await pods.text())
    } catch (e) {
        console.log(`fail to list pods: ${e}`)
        return { data: null, success: false }
    }
    for (let pod of data) {
        try {
            let mountPods = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/mountpods`)
            pod.mountPods = JSON.parse(await mountPods.text())
        } catch (e) {
            console.log(`fail to list mount pods of pod(${pod.metadata?.namespace}/${pod.metadata?.name}): ${e}`)
        }
    }
    return {
        data,
        success: true,
    }
}