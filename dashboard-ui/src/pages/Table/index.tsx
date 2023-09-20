import {
  ActionType,
  FooterToolbar,
  PageContainer,
  ProDescriptions,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { Button, Divider, Drawer, message } from 'antd';
import React, { useRef, useState } from 'react';
import { Pod } from 'kubernetes-types/core/v1'
import { listAppPods } from '@/services/pod';

/**
 *  删除节点
 * @param selectedRows
 */
const handleRemove = async (selectedRows: Pod[]) => {
  const hide = message.loading('正在删除');
  if (!selectedRows) return true;
  try {
    // await deleteUser({
    //   userId: selectedRows.find((row) => row.id)?.id || '',
    // });
    hide();
    message.success('删除成功，即将刷新');
    return true;
  } catch (error) {
    hide();
    message.error('删除失败，请重试');
    return false;
  }
};

const TableList: React.FC<unknown> = () => {
  const [createModalVisible, handleModalVisible] = useState<boolean>(false);
  const [updateModalVisible, handleUpdateModalVisible] =
    useState<boolean>(false);
  const [stepFormValues, setStepFormValues] = useState({});
  const actionRef = useRef<ActionType>();
  const [row, setRow] = useState<Pod>();
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
              await handleRemove(selectedRowsState);
              setSelectedRows([]);
              actionRef.current?.reloadAndRest?.();
            }}
          >
            批量删除
          </Button>
          <Button type="primary">批量审批</Button>
        </FooterToolbar>
      )}
      {/* <CreateForm
        onCancel={() => handleModalVisible(false)}
        modalVisible={createModalVisible}
      >
        <ProTable<API.UserInfo, API.UserInfo>
          onSubmit={async (value) => {
            const success = await handleAdd(value);
            if (success) {
              handleModalVisible(false);
              if (actionRef.current) {
                actionRef.current.reload();
              }
            }
          }}
          rowKey="id"
          type="form"
          columns={columns}
        />
      </CreateForm> */}
      {/* {stepFormValues && Object.keys(stepFormValues).length ? (
        <UpdateForm
          onSubmit={async (value) => {
            const success = await handleUpdate(value);
            if (success) {
              handleUpdateModalVisible(false);
              setStepFormValues({});
              if (actionRef.current) {
                actionRef.current.reload();
              }
            }
          }}
          onCancel={() => {
            handleUpdateModalVisible(false);
            setStepFormValues({});
          }}
          updateModalVisible={updateModalVisible}
          values={stepFormValues}
        />
      ) : null} */}

      <Drawer
        width={600}
        open={!!row}
        onClose={() => {
          setRow(undefined);
        }}
        closable={false}
      >
        {row?.metadata?.name && (
          <ProDescriptions<Pod>
            column={2}
            title={row?.metadata?.name}
            request={async () => ({
              data: row || {},
            })}
            params={{
              id: row?.metadata?.name,
            }}
            columns={columns}
          />
        )}
      </Drawer>
    </PageContainer>
  );
};

export default TableList;
