import React from 'react'
import { ProCard, ProDescriptions } from '@ant-design/pro-components'
import { EnvVar } from 'kubernetes-types/core/v1'
import { FormattedMessage } from 'react-intl'

import PVCWithSelector from '@/components/config/pvc-with-selector.tsx'
import { KeyValue, mountPodPatch } from '@/types/config.ts'
import { PVCWithPod } from '@/types/k8s.ts'

const MountPodPatchDetail: React.FC<{
  patch: mountPodPatch
  pvcs?: PVCWithPod[]
}> = (props) => {
  const { patch, pvcs } = props
  const kvDescribe = (kv?: KeyValue[]) => {
    if (!kv) return null
    return Object.entries(kv).map(([key, value]) => (
      <ProDescriptions.Item key={key} label={value.key}>
        {value.key}: {value.value}
      </ProDescriptions.Item>
    ))
  }

  const envDescribe = (l?: EnvVar[]) => {
    if (!l) return null
    return Object.entries(l).map(([key, value], index) => (
      <React.Fragment key={key}>
        <ProDescriptions.Item key={key} label={value.name}>
          {value.name}: {value.value}
        </ProDescriptions.Item>
        {index < Object.entries(patch.env || []).length - 1 && <br />}
      </React.Fragment>
    ))
  }

  const renderProDescriptionItem = (
    condition: boolean,
    labelId: string,
    content: React.ReactNode,
  ) =>
    condition && (
      <ProDescriptions.Item label={<FormattedMessage id={labelId} />}>
        {content}
      </ProDescriptions.Item>
    )

  return (
    <>
      {patch.pvcSelector && (
        <ProCard title={<FormattedMessage id="selector" />}>
          <ProDescriptions column={3} dataSource={patch.pvcSelector}>
            {renderProDescriptionItem(
              !!patch.pvcSelector.matchName,
              'pvcName',
              patch.pvcSelector.matchName,
            )}
            {renderProDescriptionItem(
              !!patch.pvcSelector.matchStorageClassName,
              'scName',
              patch.pvcSelector.matchStorageClassName,
            )}
            {renderProDescriptionItem(
              !!patch.pvcSelector.matchLabels,
              'pvcLabelMatch',
              kvDescribe(patch.pvcSelector.matchLabels),
            )}
          </ProDescriptions>
        </ProCard>
      )}

      <ProCard title={<FormattedMessage id="patch" />}>
        <ProDescriptions column={2} dataSource={patch}>
          {renderProDescriptionItem(
            !!patch.ceMountImage,
            'ceImage',
            patch.ceMountImage,
          )}
          {renderProDescriptionItem(
            !!patch.eeMountImage,
            'eeImage',
            patch.eeMountImage,
          )}
          {renderProDescriptionItem(
            !!patch.labels,
            'labels',
            kvDescribe(patch.labels),
          )}
          {renderProDescriptionItem(
            !!patch.annotations,
            'annotations',
            kvDescribe(patch.annotations),
          )}
          {renderProDescriptionItem(
            !!patch.mountOptions,
            'mountOptions',
            patch.mountOptions?.map((value) => (
              <ProDescriptions.Item key={value.key}>
                {value.value}
              </ProDescriptions.Item>
            )),
          )}
          {renderProDescriptionItem(
            !!patch.env,
            'envs',
            envDescribe(patch.env),
          )}
          {renderProDescriptionItem(
            !!patch.resources?.requests,
            'resourceRequests',
            <>
              {renderProDescriptionItem(
                !!patch.resources?.requests?.cpu,
                'cpu',
                <>
                  <FormattedMessage id="cpu" />:{' '}
                  {patch.resources?.requests?.cpu}
                </>,
              )}
              {patch.resources?.requests?.cpu &&
              patch.resources.requests.memory ? (
                <br />
              ) : null}
              {renderProDescriptionItem(
                !!patch.resources?.requests?.memory,
                'memory',
                <>
                  <FormattedMessage id="memory" />:{' '}
                  {patch.resources?.requests?.memory}
                </>,
              )}
            </>,
          )}
          {renderProDescriptionItem(
            !!patch.resources?.limits,
            'resourceLimits',
            <>
              {renderProDescriptionItem(
                !!patch.resources?.limits?.cpu,
                'cpu',
                <>
                  <FormattedMessage id="cpu" />: {patch.resources?.limits?.cpu}
                </>,
              )}
              {patch.resources?.limits?.cpu && patch.resources.limits.memory ? (
                <br />
              ) : null}
              {renderProDescriptionItem(
                !!patch.resources?.limits?.memory,
                'memory',
                <>
                  <FormattedMessage id="memory" />:{' '}
                  {patch.resources?.limits?.memory}
                </>,
              )}
            </>,
          )}
        </ProDescriptions>
      </ProCard>

      <PVCWithSelector pvcSelector={patch.pvcSelector} pvcs={pvcs} />
    </>
  )
}

export default MountPodPatchDetail
