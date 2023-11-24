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

import { getPVOfSC, getSC } from '@/services/pv';
import { Link } from '@@/exports';
import {
  PageContainer,
  ProCard,
  ProDescriptions,
} from '@ant-design/pro-components';
import { useLocation, useParams, useSearchParams } from '@umijs/max';
import { Empty, List, Table, Tag } from 'antd';
import { PersistentVolume } from 'kubernetes-types/core/v1';
import { StorageClass } from 'kubernetes-types/storage/v1';
import React, { useEffect, useState } from 'react';
import { FormattedMessage } from 'umi';
import { formatData } from '../utils';

const DetailedSC: React.FC<unknown> = () => {
  const location = useLocation();
  const params = useParams();
  const [searchParams] = useSearchParams();
  const scName = params['scName'] || '';
  const format = searchParams.get('raw');
  const [sc, setSC] = useState<StorageClass>();
  const [pvs, setPVs] = useState<PersistentVolume[]>();

  useEffect(() => {
    getSC(scName).then(setSC);
  }, [setSC]);
  useEffect(() => {
    getPVOfSC(scName).then(setPVs);
  }, [setPVs]);

  if (scName !== '') {
    return (
      <PageContainer
        header={{
          title: <FormattedMessage id="scNotFound" />,
        }}
      ></PageContainer>
    );
  }

  const getSCTabsContent = (sc: StorageClass) => {
    let content: any;
    let parameters: string[] = [];
    for (const key in sc.parameters) {
      if (sc.parameters.hasOwnProperty(key)) {
        const value = sc.parameters[key];
        parameters.push(`${key}: ${value}`);
      }
    }
    content = (
      <div>
        <ProCard title={<FormattedMessage id="basic" />}>
          <ProDescriptions
            column={2}
            dataSource={{
              reclaimPolicy: sc.reclaimPolicy,
              expansion: sc.allowVolumeExpansion ? (
                <FormattedMessage id="true" />
              ) : (
                <FormattedMessage id="false" />
              ),
              time: sc.metadata?.creationTimestamp,
            }}
            columns={[
              {
                title: <FormattedMessage id="reclaimPolicy" />,
                key: 'reclaimPolicy',
                dataIndex: 'reclaimPolicy',
              },
              {
                title: <FormattedMessage id="allowVolumeExpansion" />,
                key: 'expansion',
                dataIndex: 'expansion',
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
        <ProCard title="Paramters">
          <List
            dataSource={parameters}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>
        <ProCard title={<FormattedMessage id="mountOptions" />}>
          <List
            dataSource={sc.mountOptions}
            split={false}
            size="small"
            renderItem={(item) => (
              <List.Item>
                <code>{item}</code>
              </List.Item>
            )}
          />
        </ProCard>

        <ProCard title={'PV'}>
          <Table
            columns={[
              {
                title: <FormattedMessage id="name" />,
                key: 'name',
                render: (pv) => (
                  <Link to={`/pv/${pv.metadata.name}/`}>
                    {pv.metadata.name}
                  </Link>
                ),
              },
              {
                title: <FormattedMessage id="status" />,
                key: 'status',
                dataIndex: ['status', 'phase'],
                render: (status) => {
                  let color = 'grey';
                  let text = status;
                  switch (status) {
                    case 'Pending':
                      color = 'yellow';
                      break;
                    case 'Bound':
                      color = 'green';
                      break;
                    case 'Available':
                      color = 'blue';
                      break;
                    case 'Failed':
                      color = 'red';
                      break;
                    case 'Released':
                    default:
                      color = 'grey';
                      break;
                  }
                  return <Tag color={color}>{text}</Tag>;
                },
              },
              {
                title: <FormattedMessage id="startStatus" />,
                dataIndex: ['metadata', 'creationTimestamp'],
                key: 'startAt',
              },
            ]}
            dataSource={pvs}
            rowKey={(c) => c.metadata?.uid || ''}
            pagination={false}
          />
        </ProCard>
      </div>
    );
    return content;
  };

  let contents;
  if (!sc) {
    return <Empty />;
  } else if (
    typeof format === 'string' &&
    (format === 'json' || format === 'yaml')
  ) {
    contents = formatData(sc, format);
  } else {
    contents = (
      <ProCard key={sc.metadata?.uid} direction="column">
        {getSCTabsContent(sc)}
      </ProCard>
    );
  }
  return (
    <PageContainer
      fixedHeader
      header={{
        title: (
          <Link to={`/storageclass/${sc.metadata?.name}`}>
            {sc.metadata?.name}
          </Link>
        ),
      }}
    >
      {contents}
    </PageContainer>
  );
};

export default DetailedSC;
