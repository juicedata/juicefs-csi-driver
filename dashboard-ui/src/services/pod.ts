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

import {Pod as RawPod, Node as RawNode, Event, PersistentVolume, PersistentVolumeClaim} from 'kubernetes-types/core/v1'

export type Pod = RawPod & {
    mountPods?: RawPod[],
    appPods?: RawPod[],
    pvcs?: PersistentVolumeClaim[],
    pvs?: PersistentVolume[],
    csiNode?: RawPod,
    node?: RawNode,
    logs: Map<string, string>,
    events?: Event[],
    failedReason?: string,
    finalStatus: string,
}

export type SortOrder = 'descend' | 'ascend' | null;

export interface AppPagingListArgs {
    pageSize?: number;
    current?: number;
    namespace?: string;
    name?: string;
    pv?: string;
    mountPod?: string;
    csiNode?: string;
    sort: Record<string, SortOrder>;
    filter: Record<string, (string | number)[] | null>
}

export const listAppPods = async (args: AppPagingListArgs) => {
    let data: {
        pods: Pod[]
        total: number
    }
    try {
        const order = args.sort['time'] || 'descend'
        const namespace = args.namespace || ""
        const name = args.name || ""
        const pv = args.pv || ""
        const csiNode = args.csiNode || ""
        const mountPod = args.mountPod || ""
        const pageSize = args.pageSize || 20
        const current = args.current || 1
        const pods = await fetch(`http://localhost:8088/api/v1/pods?order=${order}&namespace=${namespace}&name=${name}&pv=${pv}&mountpod=${mountPod}&csinode=${csiNode}&pageSize=${pageSize}&current=${current}`)
        data = JSON.parse(await pods.text())
    } catch (e) {
        console.log(`fail to list pods: ${e}`)
        return {data: null, success: false}
    }
    data.pods.forEach(pod => {
        pod.failedReason = failedReasonOfAppPod(pod)
    })
    data.pods.forEach(pod => {
        pod.finalStatus = podStatus(pod) || "Unknown"
    })
    return {
        pods: data.pods,
        success: true,
        total: data.total,
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
            const node = await fetch(`http://localhost:8088/api/v1/pod/${pod.metadata?.namespace}/${pod.metadata?.name}/node`)
            pod.node = JSON.parse(await node.text())
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
        pod.finalStatus = podStatus(pod) || "Unknown"
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

export interface SysPagingListArgs {
    pageSize?: number;
    current?: number;
    namespace?: string;
    name?: string;
    node?: string;
    sort: Record<string, SortOrder>;
    filter: Record<string, (string | number)[] | null>
}

export const listSystemPods = async (args: SysPagingListArgs) => {
    console.log(`list system pods with args: ${JSON.stringify(args)}`)
    let data: {
        pods: Pod[]
        total: number
    }
    try {
        const order = args.sort['time'] || 'ascend'
        const namespace = args.namespace || ""
        const name = args.name || ""
        const node = args.node || ""
        const pageSize = args.pageSize || 20
        const current = args.current || 1
        const podList = await fetch(`http://localhost:8088/api/v1/syspods?namespace=${namespace}&name=${name}&node=${node}&order=${order}&pageSize=${pageSize}&current=${current}`)
        data = JSON.parse(await podList.text())
    } catch (e) {
        console.log(`fail to list sys pods: ${e}`)
        return {data: null, success: false}
    }
    data.pods.forEach(pod => {
        pod.failedReason = failedReasonOfSysPod(pod)
    })
    data.pods.forEach(pod => {
        pod.finalStatus = podStatus(pod) || "Unknown"
    })
    return {
        pods: data.pods,
        success: true,
        total: data.total,
    }
}

export const podStatus = (pod: RawPod) => {
    if (pod.metadata?.deletionTimestamp) {
        return "Terminating"
    }
    if (!pod.status) {
        return "Unknown"
    }
    let status: string = ""
    pod.status?.containerStatuses?.forEach(containerStatus => {
        if (!containerStatus.ready) {
            if (containerStatus.state?.waiting && containerStatus.state.waiting.message) {
                status = "Error"
                return
            }
            if (containerStatus.state?.terminated && containerStatus.state.terminated.message) {
                status = "Error"
                return
            }
        }
    })
    if (status !== "") {
        return status
    }
    return pod.status.phase
}

const failedReasonOfAppPod = (pod: Pod) => {
    const {mountPods, pvcs, csiNode} = pod
    // check if pod is ready
    if (isPodReady(pod)) {
        return ""
    }

    let reason = ""
    // 1. PVC pending
    pvcs?.forEach(pvc => {
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

    // 3. node not ready
    if (pod.node) {
        pod.node.status?.conditions?.forEach(condition => {
            if (condition.type === "Ready" && condition.status !== "True") {
                reason = "所在节点异常，请检查节点状态。"
            }
        })
    }
    if (reason !== "") {
        return reason
    }

    // sidecar mode
    if (pod.metadata?.labels != undefined && pod.metadata?.labels["done.sidecar.juicefs.com/inject"] === "true") {
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
    // 4. check csi node
    if (csiNode === undefined) {
        return "所在节点 CSI Node 未启动成功，请检查：1. 若是 sidecar 模式，请查看其所在 namespace 是否打上需要的 label 或查看 CSI Controller 日志以确认为何 sidecar 未注入；2. 若是 Mount Pod 模式，请检查 CSI Node DaemonSet 是否未调度到该节点上。"
    }
    if (!isPodReady(csiNode)) {
        return "所在节点 CSI Node 未启动成功，请点击右方「CSI Node」查看其状态及日志。"
    }
    // 5. check mount pod
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

const failedReasonOfSysPod = (pod: Pod) => {
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

    // 2. node not ready
    if (pod.node) {
        pod.node.status?.conditions?.forEach(condition => {
            if (condition.type === "Ready" && condition.status !== "True") {
                reason = "所在节点异常，请检查节点状态。"
            }
        })
    }
    if (reason !== "") {
        return reason
    }

    // 3. container error
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