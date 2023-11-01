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
        }
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
        }
    }
}
