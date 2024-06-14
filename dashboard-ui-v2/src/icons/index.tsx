/**
 * Copyright 2024 Juicedata Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import Icon from '@ant-design/icons'
import type { GetProps } from 'antd'

import DS from '@/assets/ds-256.png'
import LOGO from '@/assets/logo.svg'
import POD from '@/assets/pod-256.png'
import PV from '@/assets/pv-256.png'
import PVC from '@/assets/pvc-256.png'
import SC from '@/assets/sc-256.png'

type CustomIconComponentProps = GetProps<typeof Icon>

const DSIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width ?? 18} src={DS} />} {...props} />
)
const PODIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width ?? 18} src={POD} />} {...props} />
)
const PVIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width ?? 18} src={PV} />} {...props} />
)
const PVCIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width ?? 18} src={PVC} />} {...props} />
)
const SCIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width ?? 18} src={SC} />} {...props} />
)
const LOGOIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width ?? 18} src={LOGO} />} {...props} />
)

export { DSIcon, PODIcon, PVCIcon, PVIcon, SCIcon, LOGOIcon }
