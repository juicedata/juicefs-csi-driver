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
import {Button, Tooltip} from 'antd';
import React, {useRef, useState} from 'react';
import {Pod, listAppPods, podStatus} from '@/services/pod';
import {Link} from 'umi';
import {Badge} from 'antd/lib';
import {PodStatusEnum} from "@/services/common";
import {AlertTwoTone} from "@ant-design/icons";
import {getPodStatusBadge, getPVStatusBadge} from "@/pages/utils";

const AppPodTable: React.FC<unknown> = () => {
    const [createModalVisible, handleModalVisible] = useState<boolean>(false);
    const actionRef = useRef<ActionType>();
    const [selectedRowsState, setSelectedRows] = useState<Pod[]>([]);
    const columns: ProColumns<Pod>[] = [
        {
            title: '名称',
            disable: true,
            key: 'name',
            formItemProps: {
                rules: [
                    {
                        required: true,
                        message: '名称为必填项',
                    },
                ],
            },
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
                        <Link to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
                            {pod.metadata?.name}
                        </Link>
                        <Tooltip title={pod.failedReason}>
                            <AlertTwoTone twoToneColor='#cf1322'/>
                        </Tooltip>
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
            key: 'pv',
            render: (_, pod) => {
                if (!pod.pvs || pod.pvs.length == 0) {
                    return <span>-</span>
                } else if (pod.pvs.length == 1) {
                    const pv = pod.pvs[0]
                    return (
                        <Badge color={getPVStatusBadge(pv)} text={
                            <Link to={`/pv/${pv.metadata?.name}`}>
                                {pv.metadata?.name}
                            </Link>
                        }/>
                    )
                } else {
                    return (
                        <div>
                            {pod.pvs.map((key) => (
                                <div>
                                    <Badge color={getPVStatusBadge(key)} text={
                                        <Link to={`/pv/${key.metadata?.name}`}>
                                            {key.metadata?.name}
                                        </Link>
                                    }/>
                                    <br/>
                                </div>
                            ))}
                        </div>
                    )
                }
            }
        },
        {
            title: 'Mount Pods',
            key: 'mountPod',
            render: (_, pod) => {
                if (!pod.mountPods || pod.mountPods.length == 0) {
                    return <span>-</span>
                } else if (pod.mountPods.length == 1) {
                    const mountPod = pod.mountPods[0]
                    if (mountPod === undefined) {
                        return
                    }
                    return (
                        <Badge color={getPodStatusBadge(podStatus(mountPod) || "")} text={
                            <Link to={`/pod/${mountPod?.metadata?.namespace}/${mountPod?.metadata?.name}/`}>
                                {mountPod?.metadata?.name}
                            </Link>
                        }/>
                    )
                } else {
                    return (
                        <div>
                            {pod.mountPods.map((mountPod) => (
                                <div>
                                    <Badge color={getPodStatusBadge(podStatus(mountPod) || "")} text={
                                        <Link
                                            to={`/pod/${mountPod.metadata?.namespace}/${mountPod.metadata?.name}/`}>
                                            {mountPod?.metadata?.name}
                                        </Link>
                                    }/>
                                    <br/>
                                </div>
                            ))}
                        </div>
                    )
                }
            }
        },
        {
            title: '状态',
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
            title: '创建时间',
            key: 'time',
            sorter: 'time',
            defaultSortOrder: 'descend',
            sortDirections: ['descend', 'ascend'],
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
            title: 'CSI Node',
            key: 'csiNode',
            render: (_, pod) => {
                if (!pod.csiNode) {
                    return "-"
                }
                return (
                    <Badge color={getPodStatusBadge(podStatus(pod.csiNode) || "")} text={
                        <Link to={`/pod/${pod.csiNode.metadata?.namespace}/${pod.csiNode.metadata?.name}/`}>
                            {pod.csiNode.metadata?.name}
                        </Link>
                    }/>
                )
            },
        },
    ];
    return (
        <PageContainer
            header={{
                title: '应用 Pod 管理',
            }}
        >
            <ProTable<Pod>
                headerTitle="Pod 列表"
                tooltip="此列表只显示使用了 JuiceFS CSI 的 Pod"
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
                    const {pods, success, total} = await listAppPods({
                        ...params,
                        sort,
                        filter,
                    });
                    return {
                        data: pods || [],
                        success,
                        total,
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


export default AppPodTable;
