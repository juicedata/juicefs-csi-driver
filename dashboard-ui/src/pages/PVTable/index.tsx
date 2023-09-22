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
import {PV, listPV} from '@/services/pv';
import {Link} from 'umi';

const PVTable: React.FC<unknown> = () => {
    const [createModalVisible, handleModalVisible] = useState<boolean>(false);
    const [updateModalVisible, handleUpdateModalVisible] =
        useState<boolean>(false);
    const [stepFormValues, setStepFormValues] = useState({});
    const actionRef = useRef<ActionType>();
    const [selectedRowsState, setSelectedRows] = useState<PV[]>([]);
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
            render: (_, pv) => (
                <Link to={`/pv/${pv.metadata?.name}`}>
                    {pv.metadata?.name}
                </Link>
            ),
        },
        {
            title: '持久卷申领',
            render: (_, pv) => {
                if (!pv.spec?.claimRef) {
                    return <span>无</span>
                } else {
                    pv.spec.claimRef.name
                    return (
                        <Link to={`/pv/${pv.spec.claimRef.namespace}/${pv.spec.claimRef.name}`}>
                            {pv.spec.claimRef.name}
                        </Link>
                    )
                }
            }
        },
        {
            title: '状态',
            dataIndex: ['status', 'phase'],
            hideInForm: true,
            valueEnum: {
                0: {text: '等待中...', status: 'Pending'},
                1: {text: '已绑定', status: 'Bound'},
            },
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
                rowKey="id"
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

export default PVTable;
