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

import {PersistentVolume, PersistentVolumeClaim} from 'kubernetes-types/core/v1'
import {StorageClass} from 'kubernetes-types/storage/v1'

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
    try {
        const rawPV = await fetch(`http://localhost:8088/api/v1/pvs`)
        data = JSON.parse(await rawPV.text())
    } catch (e) {
        console.log(`fail to list pv`)
        return {data: null, success: false}
    }
    return {
        data,
        success: true,
    }
}

export const listPVC = async () => {
    let data: PVC[] = []
    try {
        const rawPV = await fetch(`http://localhost:8088/api/v1/pvcs`)
        data = JSON.parse(await rawPV.text())
    } catch (e) {
        console.log(`fail to list pv`)
        return {data: null, success: false}
    }
    return {
        data,
        success: true,
    }
}

export const listStorageClass = async () => {
    let data: StorageClass[] = []
    try {
        const rawSC = await fetch(`http://localhost:8088/api/v1/storageclasses`)
        data = JSON.parse(await rawSC.text())
    } catch (e) {
        console.log(`fail to list sc`)
        return {data: [], success: false}
    }
    return {
        data,
        success: true,
    }
}

export const getPV = async (pvName: string) => {
    try {
        const rawPV = await fetch(`http://localhost:8088/api/v1/pv/${pvName}/`)
        return JSON.parse(await rawPV.text())
    } catch (e) {
        console.log(`fail to get pv(${pvName}): ${e}`)
        return null
    }
}

export const getPVEvents = async (pvName: string) => {
    try {
        const events = await fetch(`http://localhost:8088/api/v1/pv/${pvName}/events`)
        return JSON.parse(await events.text())
    } catch (e) {
        console.log(`fail to get pv events (${pvName}): ${e}`)
        return null
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

export const getPVCEvents = async (namespace: string, pvcName: string) => {
    try {
        const events = await fetch(`http://localhost:8088/api/v1/pvc/${namespace}/${pvcName}/events`)
        return JSON.parse(await events.text())
    } catch (e) {
        console.log(`fail to get pvc(${namespace}/${pvcName}): ${e}`)
        return null
    }
}

export const getSC = async (scName: string) => {
    try {
        const rawSC = await fetch(`http://localhost:8088/api/v1/storageclass/${scName}/`)
        return JSON.parse(await rawSC.text())
    } catch (e) {
        console.log(`fail to get sc (${scName}): ${e}`)
        return null
    }
}

export const getMountPodOfPVC = async (namespace: string, pvcName: string) => {
    try {
        const rawPod = await fetch(`http://localhost:8088/api/v1/pvc/${namespace}/${pvcName}/mountpods`)
        return JSON.parse(await rawPod.text())
    } catch (e) {
        console.log(`fail to get mountpod of pvc(${namespace}/${pvcName}): ${e}`)
        return null
    }
}

export const getMountPodOfPV = async (pvName: string) => {
    try {
        const rawPod = await fetch(`http://localhost:8088/api/v1/pv/${pvName}/mountpods`)
        return JSON.parse(await rawPod.text())
    } catch (e) {
        console.log(`fail to get mountpod of pv (${pvName}): ${e}`)
        return null
    }
}

export const getPVOfSC = async (scName: string) => {
    try {
        const pvs = await fetch(`http://localhost:8088/api/v1/storageclass/${scName}/pvs`)
        return JSON.parse(await pvs.text())
    } catch (e) {
        console.log(`fail to get pvs of sc (${scName}): ${e}`)
        return null
    }
}
