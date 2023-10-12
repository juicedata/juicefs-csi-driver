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

import {Pod as RawPod, Event} from 'kubernetes-types/core/v1'
import {PersistentVolume, PersistentVolumeClaim} from 'kubernetes-types/core/v1'

export type Pod = RawPod & {
    mountPods?: RawPod[],
    appPods?: RawPod[],
    pvcs?: PersistentVolumeClaim[],
    pvs?: PersistentVolume[],
    csiNode?: RawPod,
    logs: Map<string, string>,
    events?: Event[],
    failedReason?: string,
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
            const mountPods = JSON.parse(await rawMountPods.text())
            let csiNode
            if (pod.spec?.nodeName) {
                const rawCSINode = await fetch(`http://localhost:8088/api/v1/csi-node/${pod.spec?.nodeName}`)
                csiNode = JSON.parse(await rawCSINode.text())
            }
            const rawPVCs = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/pvcs`)
            const pvcs = JSON.parse(await rawPVCs.text())
            const rawPVs = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/pvs`)
            const pvs = JSON.parse(await rawPVs.text())
            const failedReason = failedReasonOfAppPod(pod, mountPods, pvcs, csiNode)
            return {mountPods, csiNode, pvcs, pvs, failedReason}
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
        const {mountPods, csiNode, pvcs, pvs, failedReason} = results[i]
        data[i].mountPods = mountPods || []
        data[i].csiNode = csiNode || null
        data[i].pvcs = pvcs || []
        data[i].pvs = pvs || []
        data[i].failedReason = failedReason || ""
    }
    if (args.pv) {
        data = data.filter(pod => {
            return !!pod.metadata?.name?.includes(args.pv || "");
        })
    }
    if (args.csiNode) {
        data = data.filter(pod => pod.csiNode?.metadata?.name?.includes(args.csiNode || ""))
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
        pod.mountPods = JSON.parse(await mountPods.text())
        const appPods = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/apppods`)
        pod.appPods = JSON.parse(await appPods.text())
        if (pod.spec?.nodeName) {
            const csiNode = await fetch(`http://localhost:8088/api/v1/csi-node/${pod.spec?.nodeName}`)
            pod.csiNode = JSON.parse(await csiNode.text())
        }
        const events = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/events`)
        let podEvents: Event[] = JSON.parse(await events.text()) || []
        podEvents.sort((a, b) => {
            const aTime = new Date(a.firstTimestamp || a.eventTime || 0).getTime()
            const bTime = new Date(b.firstTimestamp || b.eventTime || 0).getTime()
            return bTime - aTime
        })
        pod.events = podEvents
        pod.logs = new Map()
        return pod
    } catch (e) {
        console.log(`fail to get pod(${namespace}/${podName}): ${e}`)
        return null
    }
}

export const getLog = async (pod: Pod, container: string) => {
    try {
        const log = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/logs/${container}`)
        return await log.text()
    } catch (e) {
        console.log(`fail to get log of pod(${pod.metadata?.namespace}/${pod.metadata?.name}/${container}): ${e}`)
        return ""
    }
}

export const listSystemPods = async (args: PagingListArgs) => {
    console.log(`list system pods with args: ${JSON.stringify(args)}`)
    let data: Pod[]
    let mountPods: Pod[] = [],
        csiNodes: Pod[] = [],
        csiControllers: Pod[] = []
    try {
        const mountPodList = await fetch('http://localhost:8088/api/v1/mountpods')
        const csiNodeList = await fetch('http://localhost:8088/api/v1/csi-nodes')
        const csiControllerList = await fetch('http://localhost:8088/api/v1/controllers')
        mountPods = JSON.parse(await mountPodList.text())
        csiNodes = JSON.parse(await csiNodeList.text())
        csiControllers = JSON.parse(await csiControllerList.text())
    } catch (e) {
        console.log(`fail to list mount pods: ${e}`)
        return {data: null, success: false}
    }
    data = mountPods.concat(csiNodes, csiControllers)
    const getMore = async (pod: RawPod) => {
        const failedReason = failedReasonOfSysPod(pod)
        return {failedReason}
    }
    const tasks = []
    for (const pv of data) {
        tasks.push(getMore(pv))
    }
    const results = await Promise.all(tasks)
    for (const i in results) {
        const {failedReason} = results[i]
        data[i].failedReason = failedReason || ""
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

const failedReasonOfAppPod = (pod: RawPod, mountPods: RawPod[], pvcs: PersistentVolumeClaim[], csiNode: RawPod | undefined) => {
    // check if pod is ready
    if (isPodReady(pod)) {
        return ""
    }

    let reason = ""
    // 1. PVC pending
    pvcs.forEach(pvc => {
        if (pvc.status?.phase !== "Bound") {
            reason = `PVC "${pvc.metadata?.name}" 未成功绑定，请点击「PVC」查看详情。`
        }
    })
    if (reason !== "") {
        return reason
    }

    // 2. not scheduled
    pod.status?.conditions?.forEach(condition => {
        if (condition.type === "PodScheduled" && condition.status != "True") {
            reason = "未调度成功，请点击 Pod 详情查看调度失败的具体原因。"
            return
        }
    })
    if (reason !== "") {
        return reason
    }

    if (pod.metadata?.labels != undefined && pod.metadata?.labels["done.sidecar.juicefs.com/inject"] === "true") {
        // sidecar mode
        let reason = ""
        pod.status?.initContainerStatuses?.forEach(containerStatus => {
            if (!containerStatus.ready) {
                reason = `${containerStatus.name} 容器异常，请点击 Pod 详情查看容器状态及日志。`
            }
        })
        pod.status?.containerStatuses?.forEach(containerStatus => {
            if (!containerStatus.ready) {
                reason = `${containerStatus.name} 容器异常，请点击 Pod 详情查看容器状态及日志。`
            }
        })
        return reason
    }

    // mount pod mode
    // 2. check csi node
    if (csiNode === undefined) {
        return "所在节点 CSI Node 未启动成功，请检查：1. 若是 sidecar 模式，请查看其所在 namespace 是否打上需要的 label 或查看 CSI Controller 日志以确认为何 sidecar 未注入；2. 若是 Mount Pod 模式，请检查 CSI Node DaemonSet 是否未调度到该节点上。"
    }
    if (!isPodReady(csiNode)) {
        return "所在节点 CSI Node 未启动成功，请点击右方「CSI Node」查看其状态及日志。"
    }
    // 3. check mount pod
    if (mountPods?.length == 0) {
        return "Mount Pod 未启动，请点击右方「CSI Node」检查其日志。"
    }
    mountPods?.forEach(mountPod => {
        if (!isPodReady(mountPod)) {
            reason = "Mount Pod 未启动成功，请点击右方「Mount Pods」检查其状态及日志。"
            return
        }
    })
    if (reason !== "") {
        return reason
    }

    return "pod 异常，请点击详情查看其 event 或日志。"
}

const failedReasonOfSysPod = (pod: RawPod) => {
    // check if pod is ready
    if (isPodReady(pod)) {
        return ""
    }

    let reason = ""
    // 1. not scheduled
    pod.status?.conditions?.forEach(condition => {
        if (condition.type === "PodScheduled" && condition.status != "True") {
            reason = "未调度成功，请点击 Pod 详情查看调度失败的具体原因。"
            return
        }
    })
    if (reason !== "") {
        return reason
    }

    pod.status?.initContainerStatuses?.forEach(containerStatus => {
        if (!containerStatus.ready) {
            reason = `${containerStatus.name} 容器异常，请点击 Pod 详情查看容器状态及日志。`
        }
    })
    if (reason !== "") {
        return reason
    }
    pod.status?.containerStatuses?.forEach(containerStatus => {
        if (!containerStatus.ready) {
            reason = `${containerStatus.name} 容器异常，请点击 Pod 详情查看容器状态及日志。`
        }
    })
    if (reason !== "") {
        return reason
    }

    return "pod 异常，请点击详情查看其 event 或日志。"
}

const isPodReady = (pod: RawPod) => {
    let conditionTrue = 0
    pod.status?.conditions?.forEach(condition => {
        if ((condition.type === "ContainersReady" || condition.type === "Ready") && condition.status === "True") {
            conditionTrue++
        }
    })
    return conditionTrue === 2;
}