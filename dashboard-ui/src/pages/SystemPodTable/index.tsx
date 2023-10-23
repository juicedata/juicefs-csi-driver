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

import {
    ActionType,
    FooterToolbar,
    PageContainer,
    ProColumns,
    ProTable,
} from '@ant-design/pro-components';
import {Button, Tooltip} from 'antd';
import React, {useRef, useState} from 'react';
import {Pod, listSystemPods} from '@/services/pod';
import {Link} from 'umi';
import {AlertTwoTone} from "@ant-design/icons";
import {PodStatusEnum} from "@/services/common";

const SystemPodTable: React.FC<unknown> = () => {
    const [createModalVisible, handleModalVisible] = useState<boolean>(false);
    const actionRef = useRef<ActionType>();
    const [selectedRowsState, setSelectedRows] = useState<Pod[]>([]);
    const columns: ProColumns<Pod>[] = [
        {
            title: '名称',
            disable: true,
            key: 'name',
            render: (_, pod) => {
                if (pod.failedReason === "") {
                    return (
                        <Link to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
                            {pod.metadata?.name}
                        </Link>
                    )
                }
                return (
                    <div>
                        <Tooltip title={pod.failedReason}>
                            <AlertTwoTone twoToneColor='#cf1322'/>
                        </Tooltip>
                        <Link to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
                            {pod.metadata?.name}
                        </Link>
                    </div>
                )
            },
        },
        {
            title: '命名空间',
            key: 'namespace',
            dataIndex: ['metadata', 'namespace'],
        },
        {
            title: '状态',
            disable: true,
            search: false,
            filters: true,
            onFilter: true,
            key: 'status',
            dataIndex: ['status', 'phase'],
            valueType: 'select',
            valueEnum: PodStatusEnum,
        },
        {
            title: '创建时间',
            key: 'time',
            sorter: 'time',
            defaultSortOrder: 'ascend',
            search: false,
            render: (_, pod) => (
                <span>{
                    (new Date(pod.metadata?.creationTimestamp || "")).toLocaleDateString("en-US", {
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit"
                    })
                }</span>
            ),
        },
        {
            title: '所在节点',
            key: 'node',
            dataIndex: ['spec', 'nodeName'],
            valueType: 'text',
        },
    ];
    return (
        <PageContainer
            header={{
                title: '系统 Pod 管理',
            }}
        >
            <ProTable<Pod>
                headerTitle="查询表格"
                actionRef={actionRef}
                rowKey={(record) => record.metadata?.uid!}
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
                    const {data, success} = await listSystemPods({
                        ...params,
                        sort,
                        filter,
                    });
                    return {
                        data: data || [],
                        success,
                    };
                }}
                columns={columns}
                rowSelection={{
                    onChange: (_, selectedRows) => setSelectedRows(selectedRows),
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
