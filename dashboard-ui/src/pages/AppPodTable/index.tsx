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
import { Link } from 'umi';

const AppPodTable: React.FC<unknown> = () => {
  const [createModalVisible, handleModalVisible] = useState<boolean>(false);
  const actionRef = useRef<ActionType>();
  const [selectedRowsState, setSelectedRows] = useState<Pod[]>([]);
  const columns: ProColumns<Pod>[] = [
    {
      title: '命名空间',
      disable: true,
      key: 'namespace',
      dataIndex: ['metadata', 'namespace'],
    },
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
      render: (_, pod) => (
        <Link to={`/pod/${pod.metadata?.namespace}/${pod.metadata?.name}`}>
          {pod.metadata?.name}
        </Link>
      ),
    },
    {
      title: '持久卷',
      key: 'pv',
      render: (_, pod) => {
        if (!pod.mountPods || pod.mountPods.size == 0) {
          return <span>无</span>
        } else if (pod.mountPods.size == 1) {
          const [firstKey] = pod.mountPods.keys();
          const mountPod = pod.mountPods.get(firstKey);
          return (
            <Link to={`/pv/${pod.metadata?.namespace}/${firstKey}`}>
              {firstKey}
            </Link>
          )
        } else {
          return (
            <ul>
              {Array.from(pod.mountPods?.keys()).map((key) => (
                <li>
                <Link to={`/pv/${pod.metadata?.namespace}/${key}`}>
                  {key}
                </Link>
                </li>
              ))}
            </ul>
          )
        }
      }
    },
    {
      title: '创建时间',
      key: 'time',
      sorter: 'time',
      search: false,
      render: (_, pod) => (
        <span>{
          (new Date(pod.metadata?.creationTimestamp||"")).toLocaleDateString("en-US", { hour: "2-digit", minute: "2-digit", second: "2-digit" })
        }</span>
      ),
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
      valueEnum: {
        Pending: {
          text: '等待运行',
          color: 'yellow',
        },
        Running: {
          text: '运行中',
          color: 'green',
        },
        Succeeded: {
          text: '已完成',
          color: 'blue',
        },
        Failed: {
          text: '失败',
          color: 'red',
        },
        Unknown: {
          text: '未知',
          color: 'grey',
        },
      },
    },
    {
      title: 'CSI 节点',
      key: 'csiNode',
      render: (_, pod) => (
        <Link to={`/pod/${pod.csiNode?.metadata?.namespace}/${pod.csiNode?.metadata?.name}`}>
          {pod.csiNode?.metadata?.name}
        </Link>
      ),
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
