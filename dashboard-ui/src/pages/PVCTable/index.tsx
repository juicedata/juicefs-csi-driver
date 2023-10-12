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
    PageContainer,
    ProColumns,
    ProTable,
} from '@ant-design/pro-components';
import {Button, Divider, Drawer, message, Tooltip} from 'antd';
import React, {useRef, useState} from 'react';
import {PVC, listPVC} from '@/services/pv';
import {Link} from 'umi';
import {PVStatusEnum} from "@/services/common";
import {AlertTwoTone} from "@ant-design/icons";

const PVCTable: React.FC<unknown> = () => {
    const [createModalVisible, handleModalVisible] = useState<boolean>(false);
    const [updateModalVisible, handleUpdateModalVisible] =
        useState<boolean>(false);
    const [stepFormValues, setStepFormValues] = useState({});
    const actionRef = useRef<ActionType>();
    const [selectedRowsState, setSelectedRows] = useState<PVC[]>([]);
    const accessModeMap: { [key: string]: string } = {
        ReadWriteOnce: 'RWO',
        ReadWriteMany: 'RWM',
        ReadOnlyMany: 'ROM',
        ReadWriteOncePod: 'RWOP',
    };
    const columns: ProColumns<PVC>[] = [
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
            render: (_, pvc) => {
                if (pvc.failedReason === "") {
                    return (
                        <Link to={`/pvc/${pvc.metadata?.namespace}/${pvc.metadata?.name}`}>
                            {pvc.metadata?.name}
                        </Link>
                    )
                }
                return (
                    <div>
                        <Tooltip title={pvc.failedReason}>
                            <AlertTwoTone twoToneColor='#cf1322'/>
                        </Tooltip>
                        <Link to={`/pvc/${pvc.metadata?.namespace}/${pvc.metadata?.name}`}>
                            {pvc.metadata?.name}
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
            title: 'PV',
            render: (_, pvc) => {
                if (!pvc.spec?.volumeName) {
                    return <span>无</span>
                } else {
                    return (
                        <Link to={`/pv/${pvc.spec.volumeName}`}>
                            {pvc.spec.volumeName}
                        </Link>
                    )
                }
            }
        },
        {
            title: '容量',
            key: 'storage',
            search: false,
            dataIndex: ['spec', 'resources', 'requests', 'storage'],
        },
        {
            title: '访问模式',
            key: 'accessModes',
            search: false,
            render: (_, pvc) => {
                let accessModes: string
                if (pvc.spec?.accessModes) {
                    accessModes = pvc.spec.accessModes.map(accessMode => accessModeMap[accessMode] || 'Unknown').join(",")
                    return (
                        <div>{accessModes}</div>
                    )
                }
            }
        },
        {
            title: 'StorageClass',
            key: 'storageClassName',
            render: (_, pvc) => {
                if (pvc.spec?.storageClassName) {
                    return (
                        <Link to={`/storageclass/${pvc.spec?.storageClassName}/`}>{pvc.spec?.storageClassName}</Link>
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
                title: 'PVC 管理',
            }}
        >
            <ProTable<PVC>
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
                    const {data, success} = await listPVC();
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
            {/*    </FooterToolbar>*/}
            {/*)}*/}
        </PageContainer>
    );
};

export default PVCTable;
