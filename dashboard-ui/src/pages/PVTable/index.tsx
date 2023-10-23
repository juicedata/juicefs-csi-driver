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
import {PV, listPV} from '@/services/pv';
import {Link} from 'umi';
import {PVStatusEnum} from "@/services/common";
import {AlertTwoTone} from "@ant-design/icons";

const PVTable: React.FC<unknown> = () => {
    const [createModalVisible, handleModalVisible] = useState<boolean>(false);
    const [updateModalVisible, handleUpdateModalVisible] =
        useState<boolean>(false);
    const [stepFormValues, setStepFormValues] = useState({});
    const actionRef = useRef<ActionType>();
    const [selectedRowsState, setSelectedRows] = useState<PV[]>([]);
    const accessModeMap: { [key: string]: string } = {
        ReadWriteOnce: 'RWO',
        ReadWriteMany: 'RWM',
        ReadOnlyMany: 'ROM',
        ReadWriteOncePod: 'RWOP',
    };
    const columns: ProColumns<PV>[] = [
        {
            title: '名称',
            formItemProps: {
                rules: [
                    {
                        required: true,
                        message: '名称为必填项',
                    },
                ],
            },
            render: (_, pv) => {
                if (pv.failedReason === "") {
                    return (
                        <Link to={`/pv/${pv.metadata?.name}/`}>
                            {pv.metadata?.name}
                        </Link>
                    )
                }
                return (
                    <div>
                        <Tooltip title={pv.failedReason}>
                            <AlertTwoTone twoToneColor='#cf1322'/>
                        </Tooltip>
                        <Link to={`/pv/${pv.metadata?.name}/`}>
                            {pv.metadata?.name}
                        </Link>
                    </div>
                )
            },
        },
        {
            title: 'PVC',
            render: (_, pv) => {
                if (!pv.spec?.claimRef) {
                    return <span>无</span>
                } else {
                    return (
                        <Link to={`/pvc/${pv.spec.claimRef.namespace}/${pv.spec.claimRef.name}`}>
                            {pv.spec.claimRef.namespace}/{pv.spec.claimRef.name}
                        </Link>
                    )
                }
            }
        },
        {
            title: '容量',
            key: 'storage',
            search: false,
            dataIndex: ['spec', 'capacity', 'storage'],
        },
        {
            title: '访问模式',
            key: 'accessModes',
            search: false,
            render: (_, pv) => {
                let accessModes: string
                if (pv.spec?.accessModes) {
                    accessModes = pv.spec.accessModes.map(accessMode => accessModeMap[accessMode] || 'Unknown').join(",")
                    return (
                        <div>{accessModes}</div>
                    )
                }
            }
        },
        {
            title: '回收策略',
            key: 'persistentVolumeReclaimPolicy',
            search: false,
            dataIndex: ['spec', 'persistentVolumeReclaimPolicy'],
        },
        {
            title: 'StorageClass',
            key: 'storageClassName',
            render: (_, pv) => {
                if (pv.spec?.storageClassName) {
                    return (
                        <Link to={`/storageclass/${pv.spec?.storageClassName}/`}>{pv.spec?.storageClassName}</Link>
                    )
                }
                return "-"
            }
        },
        {
            title: '状态',
            dataIndex: ['status', 'phase'],
            hideInForm: true,
            valueType: 'select',
            disable: true,
            search: false,
            filters: true,
            onFilter: true,
            key: 'status',
            valueEnum: PVStatusEnum,
        },
        {
            title: '创建时间',
            key: 'time',
            sorter: 'time',
            search: false,
            render: (_, pv) => (
                <span>{
                    (new Date(pv.metadata?.creationTimestamp || "")).toLocaleDateString("en-US", {
                        hour: "2-digit",
                        minute: "2-digit",
                        second: "2-digit"
                    })
                }</span>
            ),
        },
    ];
    return (
        <PageContainer
            header={{
                title: 'PV 管理',
            }}
        >
            <ProTable<PV>
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
                    const {data, success} = await listPV();
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


export default PVTable;
