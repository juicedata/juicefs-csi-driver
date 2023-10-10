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
            text: '等待运行',
            color: 'yellow',
        },
        Bound: {
            text: '已绑定',
            color: 'green',
        },
        Available: {
            text: '可绑定',
            color: 'blue',
        },
        Released: {
            text: '已释放',
            color: 'grey',
        },
        Failed: {
            text: '失败',
            color: 'red',
        }
    }
}
