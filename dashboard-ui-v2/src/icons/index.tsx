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

import CM from '@/assets/cm-256.png'
import DS from '@/assets/ds-256.png'
import LOGO from '@/assets/logo.svg'
import POD from '@/assets/pod-256.png'
import PV from '@/assets/pv-256.png'
import PVC from '@/assets/pvc-256.png'
import SC from '@/assets/sc-256.png'

type CustomIconComponentProps = GetProps<typeof Icon>

const DSIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => <img width={props.width ?? 18} src={DS} />}
    {...props}
  />
)
const PODIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => <img width={props.width ?? 18} src={POD} />}
    {...props}
  />
)
const PVIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => <img width={props.width ?? 18} src={PV} />}
    {...props}
  />
)
const PVCIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => <img width={props.width ?? 18} src={PVC} />}
    {...props}
  />
)
const SCIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => <img width={props.width ?? 18} src={SC} />}
    {...props}
  />
)
const CMIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => <img width={props.width ?? 18} src={CM} />}
    {...props}
  />
)
const LOGOIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon component={() => <img width={props.width} src={LOGO} />} {...props} />
)
const LocaleIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => (
      <svg viewBox="0 0 1024 1024" width="1em" height="1em">
        <path d="M549.12 642.986667l-108.373333-107.093334 1.28-1.28A747.52 747.52 0 0 0 600.32 256H725.333333V170.666667h-298.666666V85.333333H341.333333v85.333334H42.666667v84.906666h476.586666C490.666667 337.92 445.44 416 384 484.266667 344.32 440.32 311.466667 392.106667 285.44 341.333333h-85.333333c31.146667 69.546667 73.813333 135.253333 127.146666 194.56l-217.173333 214.186667L170.666667 810.666667l213.333333-213.333334 132.693333 132.693334 32.426667-87.04zM789.333333 426.666667h-85.333333L512 938.666667h85.333333l47.786667-128h202.666667L896 938.666667h85.333333l-192-512z m-111.786666 298.666666l69.12-184.746666L815.786667 725.333333h-138.24z"></path>
      </svg>
    )}
    {...props}
  />
)

const TerminalIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => (
      <svg viewBox="0 0 1024 1024" width="1em" height="1em">
        <path d="M93.568 984.234667c-12.416 0-23.296-4.650667-32.64-13.994667-18.645333-18.645333-18.645333-48.213333 0-65.28l388.693333-388.693333-388.693333-388.693334c-18.645333-18.645333-18.645333-48.213333 0-65.28 18.645333-18.645333 48.213333-18.645333 65.28 0l421.333333 421.333334c18.688 18.645333 18.688 48.213333 0 65.28L126.208 970.24a44.757333 44.757333 0 0 1-32.64 13.994667zM934.698667 982.698667h-419.797334c-26.453333 0-46.634667-20.224-46.634666-46.634667 0-26.453333 20.224-46.634667 46.634666-46.634667h419.797334c26.453333 0 46.634667 20.181333 46.634666 46.634667s-20.224 46.634667-46.634666 46.634667z"></path>{' '}
      </svg>
    )}
    {...props}
  />
)

const LogIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => (
      <svg viewBox="0 0 1099 1024" width="16" height="16">
        <path
          d="M733.129568 1.700997H1.700997v1020.598006h1020.598006v-765.448505z m204.119601 935.548172h-850.498338v-850.498338h614.910299l235.588039 206.671096z"
          fill="#4F4A4A"
          p-id="4314"
        ></path>
        <path
          d="M170.099668 171.800664h279.813953v85.049834H170.099668zM170.099668 372.518272h683.800664v85.049834H170.099668zM170.099668 567.282392h683.800664v85.049834H170.099668zM170.099668 762.046512h683.800664v85.049834H170.099668z"
          fill="#4F4A4A"
          p-id="4315"
        ></path>
      </svg>
    )}
    {...props}
  />
)

const AccessLogIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => (
      <svg viewBox="0 0 1024 1024" width="16" height="16">
        <path
          d="M156.261326 1024c-85.943729 0-156.261326-69.82928-156.261326-156.261326V316.917501c0-85.943729 69.82928-156.261326 156.261326-156.261326h765.680496c85.943729 0 156.261326 69.82928 156.261325 156.261326v550.821173c0 85.943729-69.82928 156.261326-156.261325 156.261326H156.261326z m0-813.04721c-49.808298 0-104.988078 56.156414-104.988079 105.964711v550.821173c0 49.808298 55.668097 106.941345 104.988079 106.941345h765.680496c49.808298 0 106.453028-57.621364 106.453028-106.941345V316.917501c0-49.808298-56.644731-105.964711-106.453028-105.964711H156.261326z m0 16.114449"
          fill="#515151"
          p-id="5159"
        ></path>
        <path
          d="M622.115403 506.872675h-444.368145c-1.953267 0-3.906533-1.953267-3.906533-3.906533V442.903195c0-1.953267 1.46495-3.418216 3.418216-3.418216h444.368145c1.953267 0 3.906533 1.953267 3.906533 3.906533v60.062947c0.488317 1.953267-1.46495 3.418216-3.418216 3.418216z m0 238.298522h-444.368145c-1.953267 0-3.906533-1.953267-3.906533-3.906533v-60.062947c0-1.953267 1.953267-3.906533 3.906533-3.906533h444.368145c1.953267 0 3.906533 1.953267 3.906533 3.906533v60.062947c0 1.953267-1.953267 3.906533-3.906533 3.906533z m141.12351-188.978541c-6.836433 0-12.696233-2.441583-17.579399-7.324749l-66.411064-66.411064c-9.766333-9.766333-9.766333-25.392465 0-35.158798 4.39485-4.39485 10.742966-7.32475 17.579399-7.32475 6.836433 0 12.696233 2.441583 17.579399 7.32475l48.831665 48.831664 98.639962-98.639962c4.39485-4.883166 10.742966-7.32475 17.579399-7.324749 6.836433 0 12.696233 2.441583 17.579399 7.324749 9.766333 9.766333 9.766333 25.392465 0 35.158799l-116.219361 116.219361c-4.883166 4.883166-10.742966 7.32475-17.579399 7.324749z m0 238.298522c-6.836433 0-12.696233-2.441583-17.579399-7.32475l-66.411064-66.411063c-9.766333-9.766333-9.766333-25.392465 0-35.158798 4.39485-4.39485 10.742966-7.32475 17.579399-7.32475 6.836433 0 12.696233 2.441583 17.579399 7.32475l48.831665 48.831664 98.639962-98.639962c4.39485-4.883166 10.742966-7.32475 17.579399-7.32475 6.836433 0 12.696233 2.441583 17.579399 7.32475 9.766333 9.766333 9.766333 25.392465 0 35.158798l-116.219361 116.219361c-4.883166 4.883166-10.742966 7.32475-17.579399 7.32475z m0 0"
          fill="#515151"
          p-id="5160"
        ></path>
        <path
          d="M1074.784931 316.917501c0-84.478779-68.852647-153.331426-153.331426-153.331426H156.261326c-3.906533 0-7.32475 0.488317-11.231283 0.488317h-1.46495l-0.488317-1.46495V156.261326c0-85.943729 69.82928-156.261326 156.261326-156.261326H1064.530281C1150.962327 0 1220.791607 69.82928 1220.791607 156.261326v550.821173c0 81.060563-63.481164 149.424893-144.541726 155.284692h-1.46495V316.917501z m0 0"
          fill="#515151"
          p-id="5161"
        ></path>
        <path
          d="M1064.530281 41.995231H299.338102c-57.621364 0-109.871245 62.50453-109.871245 120.125894h731.986648c85.455412 0 154.796376 69.340963 154.796376 154.796376v499.547926c44.925131-5.8598 104.988078-63.481164 104.988078-109.871245V156.261326C1180.749642 105.476395 1114.826896 41.995231 1064.530281 41.995231z m0 22.950882"
          fill="#515151"
          p-id="5162"
        ></path>
      </svg>
    )}
    {...props}
  />
)

const YamlIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => (
      <svg viewBox="0 0 1024 1024" width="16" height="16">
        <path
          d="M955.315057 236.765325L745.495794 26.296158a89.256924 89.256924 0 0 0-63.300727-26.296148H311.619355c-75.089 0-136.130058 61.291021-136.130058 136.58999V471.180985H235.040573V136.59c0-42.343797 34.33497-76.828745 76.578782-76.828745h323.442617v147.088453c0 77.718615 62.830796 140.769378 140.299447 140.769378H921.939946v539.640946c0 42.263809-34.454953 76.838744-76.578781 76.838743H311.619355c-42.243812 0-76.578782-34.494947-76.578782-76.838743V723.743986h-59.551276v163.516046c0 75.298969 61.041058 136.58999 136.130058 136.58999h533.761807c75.178987 0 136.140056-61.16104 136.140056-136.58999V300.276021a90.116798 90.116798 0 0 0-26.206161-63.510696z m-179.973635 51.092516a80.878152 80.878152 0 0 1-80.738172-80.988136V59.761255L921.939946 287.857841z"
          p-id="4896"
          fill="#707070"
        ></path>
        <path
          d="M342.264865 542.190582l-19.707113 51.462461h39.04428l-18.947224-51.462461h-0.389943z"
          p-id="4897"
          fill="#707070"
        ></path>
        <path
          d="M810.146323 425.017747H80.773172a38.354381 38.354381 0 0 0-38.29439 38.414373v230.496234a38.354381 38.354381 0 0 0 38.29439 38.414372h729.373151a38.344383 38.344383 0 0 0 38.284392-38.414372V463.43212a38.344383 38.344383 0 0 0-38.284392-38.414373z m-623.70863 172.474734v56.851671h-34.834897v-56.851671l-58.191475-94.47616h40.774027l34.644924 58.57142h0.379945l32.925176-58.57142h40.764029z m197.751031 56.851671l-11.108373-29.995605h-62.200888l-11.488317 29.995605H262.836501l60.291168-151.327831h38.994287l58.57142 151.327831z m253.062927 0h-34.834897V557.168388h-0.379944L562.99253 654.344152h-33.885036l-39.434223-97.175764h-0.379945v97.175764h-34.644924V503.016321h48.992823l42.103832 105.234584h0.389942l42.103832-105.234584h48.992823z m160.246525 0H684.97466V503.016321h34.634926v120.602333h77.908587z"
          p-id="4898"
          fill="#707070"
        ></path>
      </svg>
    )}
    {...props}
  />
)

const DebugIcon = (props: Partial<CustomIconComponentProps>) => (
  <Icon
    component={() => (
      <svg viewBox="0 0 1024 1024" width="16" height="16">
        <path
          d="M1022.06544 583.40119c0 11.0558-4.034896 20.61962-12.111852 28.696576-8.077979 8.077979-17.639752 12.117992-28.690436 12.117992L838.446445 624.215758c0 72.690556-14.235213 134.320195-42.718941 184.89915l132.615367 133.26312c8.076956 8.065699 12.117992 17.634636 12.117992 28.690436 0 11.050684-4.034896 20.614503-12.117992 28.691459-7.653307 8.065699-17.209964 12.106736-28.690436 12.106736-11.475356 0-21.040199-4.041036-28.690436-12.106736L744.717737 874.15318c-2.124384 2.118244-5.308913 4.88424-9.558703 8.283664-4.259 3.3984-13.180184 9.463536-26.78504 18.171871-13.598716 8.715499-27.415396 16.473183-41.439808 23.276123-14.029528 6.797823-31.462572 12.966313-52.289923 18.49319-20.827351 5.517667-41.446971 8.28571-61.842487 8.28571L552.801776 379.38668l-81.611739 0 0 571.277058c-21.668509 0-43.250036-2.874467-64.707744-8.615215-21.473057-5.734608-39.960107-12.749372-55.476499-21.039175-15.518438-8.289804-29.541827-16.572444-42.077328-24.867364-12.541641-8.290827-21.781072-15.193027-27.739784-20.714787l-9.558703-8.93244L154.95056 998.479767c-8.500605 8.921183-18.699897 13.386892-30.606065 13.386892-10.201339 0-19.335371-3.40454-27.409257-10.202363-8.079002-7.652284-12.437264-17.10968-13.080923-28.372188-0.633427-11.263531 2.659573-21.143553 9.893324-29.647227l128.787178-144.727219c-24.650423-48.464805-36.980239-106.699114-36.980239-174.710091L42.738895 624.207571c-11.057847 0-20.61655-4.041036-28.690436-12.111852-8.079002-8.082072-12.120039-17.640776-12.120039-28.696576 0-11.050684 4.041036-20.61962 12.120039-28.689413 8.073886-8.072863 17.632589-12.107759 28.690436-12.107759l142.81466 0L185.553555 355.156836l-110.302175-110.302175c-8.074909-8.077979-12.113899-17.640776-12.113899-28.691459 0-11.04966 4.044106-20.61962 12.113899-28.690436 8.071839-8.076956 17.638729-12.123109 28.691459-12.123109 11.056823 0 20.612457 4.052293 28.692482 12.123109l110.302175 110.302175 538.128077 0 110.303198-110.302175c8.070816-8.076956 17.632589-12.123109 28.690436-12.123109 11.050684 0 20.617573 4.052293 28.689413 12.123109 8.077979 8.070816 12.119015 17.640776 12.119015 28.690436 0 11.050684-4.041036 20.614503-12.119015 28.691459l-110.302175 110.302175 0 187.448206 142.815683 0c11.0558 0 20.618597 4.034896 28.690436 12.113899 8.076956 8.069793 12.117992 17.638729 12.117992 28.683273l0 0L1022.06544 583.40119 1022.06544 583.40119zM716.021162 216.158085 307.968605 216.158085c0-56.526411 19.871583-104.667851 59.616796-144.414087 39.733956-39.746236 87.88256-59.611679 144.411017-59.611679 56.529481 0 104.678084 19.865443 144.413064 59.611679C696.156742 111.48921 716.021162 159.631674 716.021162 216.158085L716.021162 216.158085 716.021162 216.158085 716.021162 216.158085z"
          fill="#272636"
          p-id="4303"
        ></path>
      </svg>
    )}
    {...props}
  />
)

export {
  DSIcon,
  PODIcon,
  PVCIcon,
  PVIcon,
  SCIcon,
  LOGOIcon,
  CMIcon,
  LocaleIcon,
  TerminalIcon,
  LogIcon,
  AccessLogIcon,
  YamlIcon,
  DebugIcon,
}
