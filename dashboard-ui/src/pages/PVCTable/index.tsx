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
import {PVC, listPVC} from '@/services/pv';
import {Link} from 'umi';
import {Badge} from "antd/lib";
import {Pod as RawPod} from "kubernetes-types/core/v1";

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
            render: (_, pvc) => (
                <Link to={`/pvc/${pvc.metadata?.namespace}/${pvc.metadata?.name}`}>
                    {pvc.metadata?.name}
                </Link>
            ),
        },
        {
            title: '命名空间',
            key: 'namespace',
            dataIndex: ['metadata', 'namespace'],
        },
        {
            title: '持久卷',
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
            dataIndex: ['spec', 'resources', 'requests', 'storage'],
        },
        {
            title: '访问模式',
            key: 'accessModes',
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
            key: 'StorageClassName',
            dataIndex: ['spec', 'StorageClassName'],
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
            valueEnum: {
                Pending: {
                    text: '等待运行',
                    color: 'yellow',
                },
                Bound: {
                    text: '已绑定',
                    color: 'green',
                },
                Available: {
                    text: '可绑定',
                    color: 'blue',
                },
                Released: {
                    text: '已释放',
                    color: 'grey',
                },
                Failed: {
                    text: '失败',
                    color: 'red',
                }
            },
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
