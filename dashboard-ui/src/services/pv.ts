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

import { host } from '@/services/common';
import {
  PersistentVolume,
  PersistentVolumeClaim,
} from 'kubernetes-types/core/v1';
import { StorageClass } from 'kubernetes-types/storage/v1';

export type PV = PersistentVolume & {
  Pod: {
    namespace: string;
    name: string;
  };
  failedReason?: string;
};

export type PVC = PersistentVolumeClaim & {
  Pod: {
    namespace: string;
    name: string;
  };
  failedReason?: string;
};

export interface PVPagingListArgs {
  pageSize?: number;
  current?: number;
  name?: string;
  pvc?: string;
  sc?: string;
  sort: Record<string, 'descend' | 'ascend' | null>;
  filter: Record<string, (string | number)[] | null>;
}

const failedReasonOfPVC = (pvc: PersistentVolumeClaim) => {
  if (pvc.status?.phase === 'Bound') {
    return '';
  }
  if (pvc.spec?.storageClassName !== '') {
    return 'pvNotCreatedMsg';
  }
  if (pvc.spec.volumeName) {
    return 'pvOfPVCNotFoundErrMsg';
  }
  if (pvc.spec.selector === undefined) {
    return 'pvcSelectorErrMsg';
  }
  return 'pvOfPVCNotFoundErrMsg';
};

const failedReasonOfPV = (pv: PersistentVolume) => {
  if (pv.status?.phase === 'Bound') {
    return '';
  }
  return 'pvcOfPVNotFoundErrMsg';
};

export const listPV = async (args: PVPagingListArgs) => {
  let data: {
    pvs: PV[];
    total: number;
  };
  try {
    const order = args.sort['time'] || 'descend';
    const name = args.name || '';
    const pvc = args.pvc || '';
    const sc = args.sc || '';
    const pageSize = args.pageSize || 20;
    const current = args.current || 1;
    const rawPV = await fetch(
      `${host}/api/v1/pvs?order=${order}&name=${name}&pvc=${pvc}&sc=${sc}&pageSize=${pageSize}&current=${current}`,
    );
    data = JSON.parse(await rawPV.text());
  } catch (e) {
    console.log(`fail to list pv`);
    return { data: null, success: false };
  }
  data.pvs.forEach((pv) => {
    pv.failedReason = failedReasonOfPV(pv);
  });
  return {
    pvs: data.pvs,
    success: true,
    total: data.total,
  };
};

export interface PVCPagingListArgs {
  pageSize?: number;
  current?: number;
  namespace?: number;
  name?: string;
  pv?: string;
  sc?: string;
  sort: Record<string, 'descend' | 'ascend' | null>;
  filter: Record<string, (string | number)[] | null>;
}

export const listPVC = async (args: PVCPagingListArgs) => {
  let data: {
    pvcs: PVC[];
    total: number;
  };
  try {
    const order = args.sort['time'] || 'descend';
    const namespace = args.namespace || '';
    const name = args.name || '';
    const pv = args.pv || '';
    const sc = args.sc || '';
    const pageSize = args.pageSize || 20;
    const current = args.current || 1;
    const rawPVC = await fetch(
      `${host}/api/v1/pvcs?order=${order}&namespace=${namespace}&name=${name}&pv=${pv}&sc=${sc}&pageSize=${pageSize}&current=${current}`,
    );
    data = JSON.parse(await rawPVC.text());
  } catch (e) {
    console.log(`fail to list pvc`);
    return { data: null, success: false };
  }
  data.pvcs.forEach((pvc) => {
    pvc.failedReason = failedReasonOfPVC(pvc);
  });
  return {
    pvcs: data.pvcs,
    success: true,
    total: data.total,
  };
};

export interface SCPagingListArgs {
  name?: string;
  sort: Record<string, 'descend' | 'ascend' | null>;
}

export const listStorageClass = async (args: SCPagingListArgs) => {
  let data: StorageClass[] = [];
  try {
    const rawSC = await fetch(`${host}/api/v1/storageclasses`);
    data = JSON.parse(await rawSC.text());
  } catch (e) {
    console.log(`fail to list sc`);
    return { data: [], success: false };
  }
  if (args.name) {
    data = data.filter((sc) => sc.metadata?.name?.includes(args.name!));
  }
  if (args.sort['time'] === 'ascend') {
    data.sort((a, b) => {
      if (a.metadata?.creationTimestamp && b.metadata?.creationTimestamp) {
        return a.metadata.creationTimestamp.localeCompare(
          b.metadata.creationTimestamp,
        );
      }
      return 0;
    });
  } else {
    data.sort((a, b) => {
      if (a.metadata?.creationTimestamp && b.metadata?.creationTimestamp) {
        return b.metadata.creationTimestamp.localeCompare(
          a.metadata.creationTimestamp,
        );
      }
      return 0;
    });
  }
  return {
    data,
    success: true,
  };
};

export const getPV = async (pvName: string) => {
  try {
    const rawPV = await fetch(`${host}/api/v1/pv/${pvName}/`);
    return JSON.parse(await rawPV.text());
  } catch (e) {
    console.log(`fail to get pv(${pvName}): ${e}`);
    return null;
  }
};

export const getPVEvents = async (pvName: string) => {
  try {
    const events = await fetch(`${host}/api/v1/pv/${pvName}/events`);
    return JSON.parse(await events.text());
  } catch (e) {
    console.log(`fail to get pv events (${pvName}): ${e}`);
    return null;
  }
};

export const getPVC = async (namespace: string, pvcName: string) => {
  try {
    const rawPV = await fetch(`${host}/api/v1/pvc/${namespace}/${pvcName}/`);
    return JSON.parse(await rawPV.text());
  } catch (e) {
    console.log(`fail to get pvc(${namespace}/${pvcName}): ${e}`);
    return null;
  }
};

export const getPVCEvents = async (namespace: string, pvcName: string) => {
  try {
    const events = await fetch(
      `${host}/api/v1/pvc/${namespace}/${pvcName}/events`,
    );
    return JSON.parse(await events.text());
  } catch (e) {
    console.log(`fail to get pvc(${namespace}/${pvcName}): ${e}`);
    return null;
  }
};

export const getSC = async (scName: string) => {
  try {
    const rawSC = await fetch(`${host}/api/v1/storageclass/${scName}/`);
    return JSON.parse(await rawSC.text());
  } catch (e) {
    console.log(`fail to get sc (${scName}): ${e}`);
    return null;
  }
};

export const getMountPodOfPVC = async (namespace: string, pvcName: string) => {
  try {
    const rawPod = await fetch(
      `${host}/api/v1/pvc/${namespace}/${pvcName}/mountpods`,
    );
    return JSON.parse(await rawPod.text());
  } catch (e) {
    console.log(`fail to get mountpod of pvc(${namespace}/${pvcName}): ${e}`);
    return null;
  }
};

export const getMountPodOfPV = async (pvName: string) => {
  try {
    const rawPod = await fetch(`${host}/api/v1/pv/${pvName}/mountpods`);
    return JSON.parse(await rawPod.text());
  } catch (e) {
    console.log(`fail to get mountpod of pv (${pvName}): ${e}`);
    return null;
  }
};

export const getPVOfSC = async (scName: string) => {
  try {
    const pvs = await fetch(`${host}/api/v1/storageclass/${scName}/pvs`);
    return JSON.parse(await pvs.text());
  } catch (e) {
    console.log(`fail to get pvs of sc (${scName}): ${e}`);
    return null;
  }
};
