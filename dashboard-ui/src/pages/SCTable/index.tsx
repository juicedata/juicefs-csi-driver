import {
    ActionType,
    FooterToolbar,
    PageContainer,
    ProDescriptions,
    ProColumns,
    ProTable,
    ProDescriptionsItemProps,
} from '@ant-design/pro-components';
import {Button, Divider, Drawer, message} from 'antd';
import React, {useRef, useState} from 'react';
import {listStorageClass} from '@/services/pv';
import {Link} from 'umi';
import {Badge} from "antd/lib";
import {StorageClass} from "kubernetes-types/storage/v1";

const SCTable: React.FC<unknown> = () => {
    const [createModalVisible, handleModalVisible] = useState<boolean>(false);
    const [updateModalVisible, handleUpdateModalVisible] =
        useState<boolean>(false);
    const [stepFormValues, setStepFormValues] = useState({});
    const actionRef = useRef<ActionType>();
    const [selectedRowsState, setSelectedRows] = useState<StorageClass[]>([]);
    const columns: ProColumns<StorageClass>[] = [
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
            render: (_, sc) => (
                <Link to={`/storageclass/${sc.metadata?.name}/`}>
                    {sc.metadata?.name}
                </Link>
            ),
        },
        {
            title: '回收策略',
            key: 'reclaimPolicy',
            dataIndex: ['reclaimPolicy'],
        },
        {
            title: '支持扩容',
            key: 'allowVolumeExpansion',
            render: (_, sc) => {
                if (sc.allowVolumeExpansion) {
                    return (
                        <div>支持</div>
                    )
                } else {
                    return (
                        <div>不支持</div>
                    )
                }
            }
        },
        {
            title: '创建时间',
            key: 'time',
            sorter: 'time',
            search: false,
            render: (_, sc) => (
                <span>{
                    (new Date(sc.metadata?.creationTimestamp || "")).toLocaleDateString("en-US", {
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
                title: 'StorageClass 管理',
            }}
        >
            <ProTable<StorageClass>
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
                    const {data, success} = await listStorageClass();
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
            {selectedRowsState?.length > 0 && (
                <FooterToolbar
                    extra={
                        <div>
                            已选择{' '}
                            <a style={{fontWeight: 600}}>{selectedRowsState.length}</a>{' '}
                            项&nbsp;&nbsp;
                        </div>
                    }
                >
                    <Button
                        onClick={async () => {
                            setSelectedRows([]);
                            actionRef.current?.reloadAndRest?.();
                        }}
                    >
                        批量删除
                    </Button>
                    <Button type="primary">批量审批</Button>
                </FooterToolbar>
            )}
        </PageContainer>
    );
};


export default SCTable;
