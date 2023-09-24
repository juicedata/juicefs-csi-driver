import {Pod as RawPod} from 'kubernetes-types/core/v1'

export type Pod = RawPod & {
    mountPods?: Map<string, RawPod>
    csiNode?: RawPod,
    log?: string,
    events?: string[],
}

export type SortOrder = 'descend' | 'ascend' | null;

export interface PagingListArgs {
    pageSize?: number;
    current?: number;
    namespace?: string;
    name?: string;
    pv?: string;
    csiNode?: string;
    sort: Record<string, SortOrder>;
    filter: Record<string, (string | number)[] | null>
}

export interface PodListResult {
    data?: Pod[];
    success: boolean;
}

export const listAppPods = async (args: PagingListArgs) => {
    console.log(`list pods with args: ${JSON.stringify(args)}`)
    let data: Pod[]
    try {
        const pods = await fetch('http://localhost:8088/api/v1/pods')
        data = JSON.parse(await pods.text())
    } catch (e) {
        console.log(`fail to list pods: ${e}`)
        return {data: null, success: false}
    }
    data = data.filter(pod =>
        pod.metadata?.namespace?.includes(args.namespace || "") &&
        pod.metadata?.name?.includes(args.name || "")
    )
    const getMore = async (pod: Pod) => {
        try {
            const rawMountPods = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/mountpods`)
            const mountPods = new Map(Object.entries(JSON.parse(await rawMountPods.text())))
            const rawCSINode = await fetch(`http://localhost:8088/api/v1/csi-node/${pod.spec?.nodeName}`)
            const csiNode = JSON.parse(await rawCSINode.text())
            return {mountPods, csiNode}
        } catch (e) {
            console.log(`fail to get mount pods or csi node by pod(${pod.metadata?.namespace}/${pod.metadata?.name}): ${e}`)
            return {mountPods: null, csiNode: null}
        }
    }

    const tasks = []
    for (const pod of data) {
        tasks.push(getMore(pod))
    }
    const results = await Promise.all(tasks)
    for (const i in results) {
        const {mountPods, csiNode} = results[i]
        data[i].mountPods = mountPods || new Map()
        data[i].csiNode = csiNode || null
    }
    data = data.filter(pod => pod.csiNode?.metadata?.name?.includes(args.csiNode || ""))
    if (args.pv) {
        data = data.filter(pod => {
            for (const name of pod.mountPods!.keys()) {
                if (name.includes(args.pv || "")) {
                    return true
                }
            }
            return false
        })
    }
    const timeOrder = args.sort['time']
    if (timeOrder) {
        data.sort((a, b) => {
            const aTime = new Date(a.metadata?.creationTimestamp || 0).getTime()
            const bTime = new Date(b.metadata?.creationTimestamp || 0).getTime()
            if (timeOrder === 'descend') {
                return bTime - aTime
            } else {
                return aTime - bTime
            }
        })
    } else {
        data.sort((a, b) => {
            const aName = a.metadata?.namespace + "/" + a.metadata?.name || ''
            const bName = b.metadata?.namespace + "/" + b.metadata?.name || ''
            return aName > bName ? 1 : -1
        })
    }
    return {
        data,
        success: true,
    }
}
export const getPod = async (namespace: string, podName: string) => {
    try {
        const rawPod = await fetch(`http://localhost:8088/api/v1/pod/${namespace}/${podName}/`)
        const pod: Pod = JSON.parse(await rawPod.text())
        const mountPods = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/mountpods`)
        pod.mountPods = new Map(Object.entries(JSON.parse(await mountPods.text())))
        const csiNode = await fetch(`http://localhost:8088/api/v1/csi-node/${pod.spec?.nodeName}`)
        pod.csiNode = JSON.parse(await csiNode.text())
        const events = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/events`)
        pod.events = JSON.parse(await events.text())
        return pod
    } catch (e) {
        console.log(`fail to get pod(${namespace}/${podName}): ${e}`)
        return null
    }
}

export const getLog = async (pod: Pod) => {
    try {
        const log = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/logs/${pod.spec?.containers[0].name}`)
        return await log.text()
    } catch (e) {
        console.log(`fail to get log of pod(${pod.metadata?.namespace}/${pod.metadata?.name}): ${e}`)
        return ""
    }
}