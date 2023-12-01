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

import { listStorageClass } from '@/services/pv';
import {
    ActionType,
    PageContainer,
    ProColumns,
    ProTable,
} from '@ant-design/pro-components';
import { Button } from 'antd';
import { StorageClass } from 'kubernetes-types/storage/v1';
import React, { useRef, useState } from 'react';
import { FormattedMessage, Link } from 'umi';

const SCTable: React.FC<unknown> = () => {
    const [, handleModalVisible] = useState<boolean>(false);
    const actionRef = useRef<ActionType>();
    const [, setSelectedRows] = useState<StorageClass[]>([]);
    const columns: ProColumns<StorageClass>[] = [
        {
            title: <FormattedMessage id="name" />,
            key: 'name',
            render: (_, sc) => (
                <Link to={`/storageclass/${sc.metadata?.name}/`}>
                    {sc.metadata?.name}
                </Link>
            ),
        },
        {
            title: <FormattedMessage id="reclaimPolicy" />,
            key: 'reclaimPolicy',
            search: false,
            dataIndex: ['reclaimPolicy'],
        },
        {
            title: <FormattedMessage id="allowVolumeExpansion" />,
            key: 'allowVolumeExpansion',
            search: false,
            render: (_, sc) => {
                if (sc.allowVolumeExpansion) {
                    return <div>{<FormattedMessage id="true" />}</div>;
                } else {
                    return (
                        <div>
                            <FormattedMessage id="false" />,
                        </div>
                    );
                }
            },
        },
        {
            title: <FormattedMessage id="createAt" />,
            key: 'time',
            sorter: 'time',
            search: false,
            render: (_, sc) => (
                <span>
                    {new Date(
                        sc.metadata?.creationTimestamp || '',
                    ).toLocaleDateString('en-US', {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                    })}
                </span>
            ),
        },
    ];
    return (
        <PageContainer
            header={{
                title: <FormattedMessage id="scTablePageName" />,
            }}
        >
            <ProTable<StorageClass>
                headerTitle={<FormattedMessage id="scTableName" />}
                actionRef={actionRef}
                rowKey={(record) => record.metadata?.uid || ''}
                search={{
                    labelWidth: 120,
                }}
                toolBarRender={() => [
                    <Button
                        key="1"
                        type="primary"
                        onClick={() => handleModalVisible(true)}
                        hidden={true}
                    >
                        新建
                    </Button>,
                ]}
                request={async (params, sort) => {
                    const { data, success } = await listStorageClass({
                        ...params,
                        sort,
                    });
                    return {
                        data: data || [],
                        success,
                    };
                }}
                columns={columns}
                rowSelection={{
                    onChange: (_, selectedRows) =>
                        setSelectedRows(selectedRows),
                }}
            />
            {/*{selectedRowsState?.length > 0 && (*/}
            {/*    <FooterToolbar*/}
            {/*        extra={*/}
            {/*            <div>*/}
            {/*                已选择{' '}*/}
            {/*                <a style={{fontWeight: 600}}>{selectedRowsState.length}</a>{' '}*/}
            {/*                项&nbsp;&nbsp;*/}
            {/*            </div>*/}
            {/*        }*/}
            {/*    >*/}
            {/*        <Button*/}
            {/*            onClick={async () => {*/}
            {/*                setSelectedRows([]);*/}
            {/*                actionRef.current?.reloadAndRest?.();*/}
            {/*            }}*/}
            {/*        >*/}
            {/*            批量删除*/}
            {/*        </Button>*/}
            {/*    </FooterToolbar>*/}
            {/*)}*/}
        </PageContainer>
    );
};

export default SCTable;
