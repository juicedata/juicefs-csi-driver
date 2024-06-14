import {
  Pod as NativePod,
  Node,
  PersistentVolume,
  PersistentVolumeClaim,
} from 'kubernetes-types/core/v1'

export type Pod = {
  mountPods?: NativePod[]
  node: Node
  pvcs: PersistentVolumeClaim[]
  pvs: PersistentVolume[]
  csiNode: NativePod
} & NativePod

export const PVStatusEnum = () => {
  return {
    Pending: {
      text: 'Pending',
      color: 'yellow',
    },
    Bound: {
      text: 'Bound',
      color: 'green',
    },
    Available: {
      text: 'Available',
      color: 'blue',
    },
    Released: {
      text: 'Released',
      color: 'grey',
    },
    Failed: {
      text: 'Failed',
      color: 'red',
    },
  }
}

export const PodStatusEnum = () => {
  return {
    Pending: {
      text: 'Pending',
      color: 'yellow',
    },
    Running: {
      text: 'Running',
      color: 'green',
    },
    Succeeded: {
      text: 'Succeeded',
      color: 'blue',
    },
    Failed: {
      text: 'Failed',
      color: 'red',
    },
    Unknown: {
      text: 'Unknown',
      color: 'grey',
    },
    Terminating: {
      text: 'Terminating',
      color: 'grey',
    },
    ContainerCreating: {
      text: 'ContainerCreating',
      color: 'yellow',
    },
    PodInitializing: {
      text: 'PodInitializing',
      color: 'yellow',
    },
    Error: {
      text: 'Error',
      color: 'red',
    },
  }
}
