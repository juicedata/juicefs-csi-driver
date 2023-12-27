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

import { getNodeStatusBadge } from '@/pages/utils';
import { PodStatusEnum } from '@/services/common';
import { Pod, listSystemPods } from '@/services/pod';
import { AlertTwoTone } from '@ant-design/icons';
import {
    ActionType,
    PageContainer,
    ProColumns,
    ProTable,
} from '@ant-design/pro-components';
import { Button, Tooltip } from 'antd';
import { Badge } from 'antd/lib';
import React, { useRef, useState } from 'react';
import { FormattedMessage, Link } from 'umi';

const SystemPodTable: React.FC<unknown> = () => {
    const [, handleModalVisible] = useState<boolean>(false);
    const actionRef = useRef<ActionType>();
    const [, setSelectedRows] = useState<Pod[]>([]);
    const columns: ProColumns<Pod>[] = [
        {
            title: <FormattedMessage id="name" />,
            disable: true,
            key: 'name',
            render: (_, pod) => {
                const podFailReason = pod.failedReason || '';
                if (pod.failedReason === '') {
                    return (
                        <Link
                            to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}`}
                        >
                            {pod.metadata?.name}
                        </Link>
                    );
                }
                const failReason = <FormattedMessage id={podFailReason} />;
                return (
                    <div>
                        <Link
                            to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}`}
                        >
                            {pod.metadata?.name}
                        </Link>
                        <Tooltip title={failReason}>
                            <AlertTwoTone twoToneColor="#cf1322" />
                        </Tooltip>
                    </div>
                );
            },
        },
        {
            title: <FormattedMessage id="namespace" />,
            key: 'namespace',
            search: false,
            dataIndex: ['metadata', 'namespace'],
        },
        {
            title: <FormattedMessage id="status" />,
            disable: true,
            search: false,
            filters: true,
            onFilter: true,
            key: 'finalStatus',
            dataIndex: ['finalStatus'],
            valueType: 'select',
            valueEnum: PodStatusEnum,
        },
        {
            title: <FormattedMessage id="createAt" />,
            key: 'time',
            sorter: 'time',
            defaultSortOrder: 'ascend',
            search: false,
            render: (_, pod) => (
                <span>
                    {new Date(
                        pod.metadata?.creationTimestamp || '',
                    ).toLocaleDateString('en-US', {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                    })}
                </span>
            ),
        },
        {
            title: <FormattedMessage id="node" />,
            key: 'node',
            dataIndex: ['spec', 'nodeName'],
            valueType: 'text',
            render: (_, pod) => {
                if (!pod.node) {
                    return '-';
                }
                return (
                    <Badge
                        color={getNodeStatusBadge(pod.node)}
                        text={pod.spec?.nodeName}
                    />
                );
            },
        },
    ];
    return (
        <PageContainer
            header={{
                title: <FormattedMessage id="systemPodTablePageName" />,
            }}
        >
            <ProTable<Pod>
                headerTitle={<FormattedMessage id="sysPodTableName" />}
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
                request={async (params, sort, filter) => {
                    const { pods, success, total } = await listSystemPods({
                        ...params,
                        sort,
                        filter,
                    });
                    return {
                        data: pods || [],
                        total,
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
            {/*        <Button type="primary">批量审批</Button>*/}
            {/*    </FooterToolbar>*/}
            {/*)}*/}
        </PageContainer>
    );
};

export default SystemPodTable;
