import {
  ActionType,
  FooterToolbar,
  PageContainer,
  ProDescriptions,
  ProColumns,
  ProTable,
  ProDescriptionsItemProps,
} from '@ant-design/pro-components';
import { Button, Divider, Drawer, message } from 'antd';
import React, { useRef, useState } from 'react';
import { Pod, listAppPods } from '@/services/pod';


const AppPodTable: React.FC<unknown> = () => {
  const [createModalVisible, handleModalVisible] = useState<boolean>(false);
  const [updateModalVisible, handleUpdateModalVisible] =
    useState<boolean>(false);
  const [stepFormValues, setStepFormValues] = useState({});
  const actionRef = useRef<ActionType>();
  const [selectedRowsState, setSelectedRows] = useState<Pod[]>([]);
  const columns: ProColumns<Pod>[] = [
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
      render: (_, pod) => (
        <a
          onClick={() => { }}
        >
          {pod.metadata?.namespace + '/' + pod.metadata?.name}
        </a>
      ),
    },
    {
      title: '挂载 Pod',
      render: (_, pod) => {
        if (pod.mountPods?.length == 1) {
          return (
            <a onClick={() => { }}>
              {pod.mountPods[0].metadata?.name}
            </a>
          )
        } else {
          return (
            <ul>
              {pod.mountPods?.map((mountPod) => (
                <li>
                <a onClick={() => { }}>
                  {mountPod.metadata?.name}
                </a>
                </li>
              ))}
            </ul>
          )
        }
      }
    },
    {
      title: '集群内 IP',
      dataIndex: ['status', 'podIP'],
      valueType: 'text',
    },
    {
      title: '状态',
      dataIndex: ['status', 'phase'],
      hideInForm: true,
      valueEnum: {
        0: { text: '等待中...', status: 'Pending' },
        1: { text: '正在运行', status: 'Running' },
        2: { text: '运行成功', status: 'Succeeded' },
        3: { text: '运行失败', status: 'Failed' },
        4: { text: '未知状态', status: 'Unknown' },
      },
    },
    {
      title: '所在节点',
      dataIndex: ['spec', 'nodeName'],
      valueType: 'text',
    },
  ];
  return (
    <PageContainer
      header={{
        title: '应用 Pod 管理',
      }}
    >
      <ProTable<Pod>
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
          const { data, success } = await listAppPods({
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
      {selectedRowsState?.length > 0 && (
        <FooterToolbar
          extra={
            <div>
              已选择{' '}
              <a style={{ fontWeight: 600 }}>{selectedRowsState.length}</a>{' '}
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

export default AppPodTable;
