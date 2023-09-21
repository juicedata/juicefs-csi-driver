import { Pod as RawPod } from 'kubernetes-types/core/v1'

export type Pod = RawPod & {
    mountPods?: Map<string, RawPod>
    csiNode?: RawPod,
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
        const pods = await fetch('http://localhost:8088/api/v1/pods')
        data = JSON.parse(await pods.text())
    } catch (e) {
        console.log(`fail to list pods: ${e}`)
        return { data: null, success: false }
    }
    for (const pod of data) {
        try {
            const mountPods = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/mountpods`)
            pod.mountPods = new Map(Object.entries(JSON.parse(await mountPods.text())))
            const csiNode = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/csi-node`)
            pod.csiNode = JSON.parse(await csiNode.text())
        } catch (e) {
            console.log(`fail to list mount pods of pod(${pod.metadata?.namespace}/${pod.metadata?.name}): ${e}`)
        }
    }
    return {
        data,
        success: true,
    }
}
export const getPod = async (namespace: string, podName: string) => {
    try {
        const rawPod = await fetch(`http://localhost:8088/api/v1/pod/${namespace}/${podName}/`)
        const pod = JSON.parse(await rawPod.text())
        const mountPods = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/mountpods`)
        pod.mountPods = new Map(Object.entries(JSON.parse(await mountPods.text())))
        const csiNode = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/csi-node`)
        pod.csiNode = JSON.parse(await csiNode.text())
        return pod
    } catch (e) {
        console.log(`fail to get pod(${namespace}/${podName}): ${e}`)
        return null
    }
}
