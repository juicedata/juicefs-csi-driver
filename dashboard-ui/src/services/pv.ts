import {PersistentVolume, PersistentVolumeClaim} from 'kubernetes-types/core/v1'
import {Pod} from "@/services/pod";

export type PV = PersistentVolume & {
    Pod: {
        namespace: string
        name: string
    }
}

export type PVC = PersistentVolumeClaim & {
    Pod: {
        namespace: string
        name: string
    }
}

export const listPV = async () => {
    let data: PV[] = []
    let pvc: Map<string, any> = new Map
    try {
        const rawPV = await fetch(`http://localhost:8088/api/v1/pvs`)
        pvc = new Map(Object.entries(JSON.parse(await rawPV.text())))
    } catch (e) {
        console.log(`fail to list pv`)
        return {data: null, success: false}
    }
    pvc.forEach((pvData, pvcName) => {
        const pv = {
            metadata: pvData.metadata,
            spec: pvData.spec,
            status: pvData.status,
            Pod: pvData.Pod,
        }
        data.push(pv)
    })
    return {
        data,
        success: true,
    }
}

export const getPVC = async (namespace: string, pvcName: string) => {
    try {
        const rawPV = await fetch(`http://localhost:8088/api/v1/pvc/${namespace}/${pvcName}/`)
        return JSON.parse(await rawPV.text())
    } catch (e) {
        console.log(`fail to get pvc(${namespace}/${pvcName}): ${e}`)
        return null
    }
}

export const getMountPodOfPVC = async (namespace: string, pvcName: string) => {
    try {
        const rawPod = await fetch(`http://localhost:8088/api/v1/pvc/${namespace}/${pvcName}/mountpod`)
        return JSON.parse(await rawPod.text())
    } catch (e) {
        console.log(`fail to get mountpod of pvc(${namespace}/${pvcName}): ${e}`)
        return null
    }
}
