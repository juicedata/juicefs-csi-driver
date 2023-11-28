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

import { EventTable, getPodTableContent } from '@/pages/DetailedPod';
import { PVStatusEnum } from '@/services/common';
import { getMountPodOfPVC, getPVC, getPVCEvents } from '@/services/pv';
import {
  PageContainer,
  PageLoading,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components';
import { useLocation, useParams, useSearchParams } from '@umijs/max';
import {
  Event,
  PersistentVolumeClaim,
  Pod as RawPod,
} from 'kubernetes-types/core/v1';
import React, { useEffect, useState } from 'react';
import { FormattedMessage, Link } from 'umi';
import { formatData } from '../utils';

const DetailedPVC: React.FC<unknown> = () => {
  const location = useLocation();
  const params = useParams();
  const [searchParams] = useSearchParams();
  const namespace = params['namespace'] || '';
  const name = params['name'] || '';
  const format = searchParams.get('raw');
  const [pvc, setPVC] = useState<PersistentVolumeClaim>();
  const [mountpods, setMountPod] = useState<RawPod[]>();
  const [events, setEvents] = useState<Event[]>();

  useEffect(() => {
    getPVC(namespace, name)
      .then(setPVC)
      .then(() => getMountPodOfPVC(namespace, name))
      .then(setMountPod);
  }, [setPVC, setMountPod]);
  useEffect(() => {
    getPVCEvents(namespace, name).then(setEvents);
  }, [setEvents]);

  if (namespace === '' || name === '') {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="pvcNotFound" />,
        }}
      ></PageContainer>
    );
  }

  const getPVCTabsContent = (pvc: PersistentVolumeClaim) => {
    const accessModeMap: { [key: string]: string } = {
      ReadWriteOnce: 'RWO',
      ReadWriteMany: 'RWM',
      ReadOnlyMany: 'ROM',
      ReadWriteOncePod: 'RWOP',
    };

    let content: any;
    content = (
      <div>
        <ProCard title={<FormattedMessage id="basic" />}>
          <ProDescriptions
            column={2}
            dataSource={{
              name: pvc.metadata?.name,
              namespace: pvc.metadata?.namespace,
              pv: `${pvc.spec?.volumeName || '-'}`,
              capacity: pvc.spec?.resources?.requests?.storage,
              accessMode: pvc.spec?.accessModes
                ?.map((accessMode) => accessModeMap[accessMode] || 'Unknown')
                .join(','),
              storageClass: pvc.spec?.storageClassName,
              status: pvc.status?.phase,
              time: pvc.metadata?.creationTimestamp,
            }}
            columns={[
              {
                title: 'PV',
                key: 'pv',
                render: (_, record) => {
                  if (record.pv === '-') {
                    return '-';
                  }
                  return <Link to={`/pv/${record.pv}`}>{record.pv}</Link>;
                },
              },
              {
                title: <FormattedMessage id="namespace" />,
                key: 'namespace',
                dataIndex: 'namespace',
              },
              {
                title: <FormattedMessage id="capacity" />,
                key: 'capacity',
                dataIndex: 'capacity',
              },
              {
                title: <FormattedMessage id="accessMode" />,
                key: 'accessMode',
                dataIndex: 'accessMode',
              },
              {
                title: 'StorageClass',
                key: 'storageClass',
                render: (_, record) => {
                  if (!record.storageClass) {
                    return '-';
                  }
                  return (
                    <Link to={`/storageclass/${record.storageClass}`}>
                      {record.storageClass}
                    </Link>
                  );
                },
              },
              {
                title: <FormattedMessage id="status" />,
                key: 'status',
                dataIndex: 'status',
                valueType: 'select',
                valueEnum: PVStatusEnum,
              },
              {
                title: <FormattedMessage id="createAt" />,
                key: 'time',
                dataIndex: 'time',
              },
              {
                title: 'Yaml',
                key: 'yaml',
                render: () => (
                  <Link to={`${location.pathname}?raw=yaml`}>
                    {<FormattedMessage id="clickToView" />}
                  </Link>
                ),
              },
            ]}
          />
        </ProCard>

        {getPodTableContent(mountpods || [], 'Mount Pods')}

        {EventTable(events || [])}
      </div>
    );
    return content;
  };

  let contents;
  if (!pvc) {
    return <PageLoading />;
  } else if (
    typeof format === 'string' &&
    (format === 'json' || format === 'yaml')
  ) {
    contents = formatData(pvc, format);
  } else {
    contents = <ProCard direction="column">{getPVCTabsContent(pvc)}</ProCard>;
  }
  return (
    <PageContainer
      header={{
        title: (
          <Link to={`/pvc/${pvc.metadata?.namespace}/${pvc?.metadata?.name}`}>
            {pvc?.metadata?.name}
          </Link>
        ),
      }}
      fixedHeader
    >
      {contents}
    </PageContainer>
  );
};

export default DetailedPVC;
