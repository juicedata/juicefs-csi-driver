import { PersistentVolume } from 'kubernetes-types/core/v1'

export type PV = PersistentVolume & {
    Pod: {
        namespace: string
        name: string
    }
}

export const getPV = async (namespace: string, podName: string) => {
    try {
        const rawPV = await fetch(`http://localhost:8088/api/v1/pv/${namespace}/${podName}/`)
        return JSON.parse(await rawPV.text())
    } catch (e) {
        console.log(`fail to get pod(${namespace}/${podName}): ${e}`)
        return null
    }
}

export const getMountPodofPV = async (namespace: string, podName: string) => {
    try {
        const rawPod = await fetch(`http://localhost:8088/api/v1/pv/${namespace}/${podName}/mountpod`)
        return JSON.parse(await rawPod.text())
    } catch (e) {
        console.log(`fail to get mountpod of pvc(${namespace}/${podName}): ${e}`)
        return null
    }
}
