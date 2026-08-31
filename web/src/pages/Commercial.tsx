import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Row,
  Select,
  Skeleton,
  Space,
  Statistic,
  Table,
  Tag,
  Typography
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  BellOutlined,
  FileDoneOutlined,
  FileProtectOutlined,
  MessageOutlined,
  PayCircleOutlined,
  PlusOutlined,
  RedoOutlined,
  RollbackOutlined,
  StopOutlined
} from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData } from '../services/http';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton } from '../components/ListViews';
import type {
  CommercialOrder,
  CommercialOrderCreateRequest,
  CommercialSummary,
  ContractCreateRequest,
  InvoiceCreateRequest,
  LessonConsumptionCreateRequest,
  ParentNoticeCreateRequest,
  PaymentCreateRequest,
  RefundCreateRequest,
  RefundSuspensionResult,
  RenewalReminderCreateRequest,
  Student,
  StudyPackage
} from '../types/starline';

type ActionKind = 'payment' | 'refund' | 'refundAndSuspend' | 'contract' | 'invoice' | 'lesson' | 'renewal' | 'notice';

const actionTitle: Record<ActionKind, string> = {
  payment: '登记线下收款',
  refund: '登记退款',
  refundAndSuspend: '退款并停学',
  contract: '签署合同',
  invoice: '开具发票',
  lesson: '登记课消',
  renewal: '创建续费提醒',
  notice: '发送家长通知'
};

export default function Commercial() {
  const [orderForm] = Form.useForm<CommercialOrderCreateRequest>();
  const [actionForm] = Form.useForm();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [activeAction, setActiveAction] = useState<{ kind: ActionKind; order: CommercialOrder } | null>(null);

  const summary = useQuery({ queryKey: ['commercial', 'summary'], queryFn: () => getData<CommercialSummary>('/commercial/summary') });
  const orders = useQuery({ queryKey: ['commercial', 'orders'], queryFn: () => getData<CommercialOrder[]>('/commercial/orders') });
  const students = useQuery({ queryKey: ['students'], queryFn: () => getData<Student[]>('/students') });
  const packages = useQuery({ queryKey: ['packages'], queryFn: () => getData<StudyPackage[]>('/packages') });

  const refreshCommercial = () => {
    queryClient.invalidateQueries({ queryKey: ['commercial'] });
    queryClient.invalidateQueries({ queryKey: ['students'] });
    queryClient.invalidateQueries({ queryKey: ['permissions'] });
    queryClient.invalidateQueries({ queryKey: ['dashboard'] });
  };

  const studentStatusByID = useMemo(
    () => new Map((students.data ?? []).map((student) => [student.id, student.accountStatus])),
    [students.data]
  );

  const createOrder = useMutation({
    mutationFn: (values: CommercialOrderCreateRequest) => postData<CommercialOrder>('/commercial/orders', values),
    onSuccess: () => {
      message.success('订单已创建。');
      setCreateOpen(false);
      orderForm.resetFields();
      refreshCommercial();
    },
    onError: (err: Error) => message.error(err.message || '订单创建失败，请检查学生和套餐。')
  });

  const submitAction = useMutation({
    mutationFn: async (values: Record<string, unknown>) => {
      if (!activeAction) return null;
      const orderID = activeAction.order.id;
      switch (activeAction.kind) {
        case 'payment':
          return postData(`/commercial/orders/${orderID}/payments`, values as PaymentCreateRequest);
        case 'refund':
          return postData(`/commercial/orders/${orderID}/refunds`, values as RefundCreateRequest);
        case 'refundAndSuspend':
          return postData<RefundSuspensionResult>(`/commercial/orders/${orderID}/refund-and-suspend`, values as RefundCreateRequest);
        case 'contract':
          return postData(`/commercial/orders/${orderID}/contracts`, values as ContractCreateRequest);
        case 'invoice':
          return postData(`/commercial/orders/${orderID}/invoices`, values as InvoiceCreateRequest);
        case 'lesson':
          return postData('/commercial/lesson-consumptions', { ...values, orderId: orderID } as LessonConsumptionCreateRequest);
        case 'renewal':
          return postData('/commercial/renewal-reminders', { ...values, orderId: orderID } as RenewalReminderCreateRequest);
        case 'notice':
          return postData('/commercial/parent-notices', { ...values, orderId: orderID } as ParentNoticeCreateRequest);
        default:
          return null;
      }
    },
    onSuccess: (result) => {
      if (activeAction?.kind === 'refundAndSuspend' && result) {
        const suspension = result as RefundSuspensionResult;
        message.success(`已退款并停用账号，已收回 ${suspension.revokedGrantCount} 项套餐权限，移除 ${suspension.removedFutureClassCount} 节后续课程。`);
      } else {
        message.success(`${activeAction ? actionTitle[activeAction.kind] : '操作'}已完成。`);
      }
      setActiveAction(null);
      actionForm.resetFields();
      refreshCommercial();
    },
    onError: (err: Error) => message.error(err.message || '操作失败，请检查后重试。')
  });

  const columns = useMemo<ColumnsType<CommercialOrder>>(() => [
    {
      title: '订单',
      dataIndex: 'orderNo',
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{record.orderNo}</Typography.Text>
          <Typography.Text type="secondary">{record.createdAt}</Typography.Text>
        </Space>
      )
    },
    {
      title: '学生 / 套餐',
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{record.studentName}</Typography.Text>
          <Typography.Text type="secondary">{record.packageName}</Typography.Text>
        </Space>
      )
    },
    { title: '订单金额', dataIndex: 'amountCent', align: 'right', render: moneyText },
    { title: '实收', dataIndex: 'paidAmountCent', align: 'right', render: moneyText },
    { title: '退款', dataIndex: 'refundedAmountCent', align: 'right', render: moneyText },
    {
      title: '课时',
      render: (_, record) => `${record.lessonConsumed}/${record.lessonTotal}`
    },
    {
      title: '状态',
      render: (_, record) => (
        <Space size={6} wrap>
          <Tag color={statusColor(record.status)}>{record.status}</Tag>
          <Tag color={record.contractStatus === '已签署' ? 'green' : 'default'}>{record.contractStatus}</Tag>
          <Tag color={record.invoiceStatus === '已开票' ? 'blue' : 'default'}>{record.invoiceStatus}</Tag>
        </Space>
      )
    },
    {
      title: '操作',
      width: 236,
      render: (_, record) => (
        <Space size={6} wrap>
          <ActionButton tooltip="线下收款" icon={<PayCircleOutlined />} onClick={() => openAction('payment', record)} />
          <ActionButton tooltip="退款" icon={<RollbackOutlined />} onClick={() => openAction('refund', record)} />
          {canRefundAndSuspend(record, studentStatusByID) && <ActionButton danger tooltip="退款并停学" icon={<StopOutlined />} onClick={() => openAction('refundAndSuspend', record)} />}
          <ActionButton tooltip="合同" icon={<FileProtectOutlined />} onClick={() => openAction('contract', record)} />
          <ActionButton tooltip="发票" icon={<FileDoneOutlined />} onClick={() => openAction('invoice', record)} />
          <ActionButton tooltip="课消" icon={<RedoOutlined />} onClick={() => openAction('lesson', record)} />
          <ActionButton tooltip="续费" icon={<BellOutlined />} onClick={() => openAction('renewal', record)} />
          <ActionButton tooltip="通知" icon={<MessageOutlined />} onClick={() => openAction('notice', record)} />
        </Space>
      )
    }
  ], [studentStatusByID]);

  function openAction(kind: ActionKind, order: CommercialOrder) {
    setActiveAction({ kind, order });
    actionForm.resetFields();
    if (kind === 'payment') {
      actionForm.setFieldsValue({ amountCent: Math.max(0, order.amountCent - order.paidAmountCent), method: '线下收款' });
    }
    if (kind === 'refund' || kind === 'refundAndSuspend') {
      actionForm.setFieldsValue({ amountCent: Math.max(0, order.paidAmountCent - order.refundedAmountCent) });
    }
    if (kind === 'invoice') {
      actionForm.setFieldsValue({ amountCent: Math.max(0, order.paidAmountCent - order.refundedAmountCent), title: order.studentName });
    }
    if (kind === 'contract') {
      actionForm.setFieldsValue({ title: `${order.studentName} ${order.packageName} 服务合同`, signer: order.studentName });
    }
    if (kind === 'lesson') {
      actionForm.setFieldsValue({ lessonCount: 1 });
    }
    if (kind === 'renewal') {
      actionForm.setFieldsValue({ reason: '剩余课时较少，建议联系家长确认续费意向。' });
    }
    if (kind === 'notice') {
      actionForm.setFieldsValue({ title: '学习服务提醒', content: `${order.studentName} 的 ${order.packageName} 有新的服务进展，请及时查看。` });
    }
  }

  if (summary.isLoading || orders.isLoading || students.isLoading || packages.isLoading) return <Skeleton active />;
  if (summary.error || orders.error || students.error || packages.error) return <Alert type="error" message="商业运营数据加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>商业运营</Typography.Title>
          <Typography.Text type="secondary">记录线下订单、收款、课消和续费跟进；家长购买课程后，在学生管理中直接开通。</Typography.Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>新建订单</Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={8} xl={4}><MetricCard title="订单数" value={summary.data?.orderCount ?? 0} /></Col>
        <Col xs={24} md={8} xl={4}><MetricCard title="已支付订单" value={summary.data?.paidOrderCount ?? 0} /></Col>
        <Col xs={24} md={8} xl={4}><MetricCard title="实收金额" value={moneyText(summary.data?.revenueCent ?? 0)} /></Col>
        <Col xs={24} md={8} xl={4}><MetricCard title="退款金额" value={moneyText(summary.data?.refundCent ?? 0)} /></Col>
        <Col xs={24} md={8} xl={4}><MetricCard title="剩余课时" value={summary.data?.lessonRemainCount ?? 0} /></Col>
        <Col xs={24} md={8} xl={4}><MetricCard title="待跟进续费" value={summary.data?.renewalTodoCount ?? 0} /></Col>
      </Row>

      <Card title="订单与课消">
        <Table
          rowKey="id"
          columns={columns}
          dataSource={orders.data ?? []}
          pagination={{ pageSize: 8 }}
          scroll={{ x: 1160 }}
          locale={{ emptyText: '暂无订单。先为学生创建订单，再记录线下收款和课消。' }}
        />
      </Card>

      <FormDrawer
        title="新建订单"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onSubmit={() => orderForm.submit()}
        submitting={createOrder.isPending}
      >
        <Form form={orderForm} layout="vertical" onFinish={(values) => createOrder.mutate(normalizeOrder(values))}>
          <Form.Item name="studentId" label="学生" rules={[{ required: true, message: '请选择学生' }]}>
            <Select showSearch optionFilterProp="label" placeholder="选择学生" options={(students.data ?? []).map((item) => ({ label: `${item.name} · ${item.grade}`, value: item.id }))} />
          </Form.Item>
          <Form.Item name="packageId" label="套餐" rules={[{ required: true, message: '请选择套餐' }]}>
            <Select showSearch optionFilterProp="label" placeholder="选择套餐" options={(packages.data ?? []).map((item) => ({ label: item.name, value: item.id }))} />
          </Form.Item>
          <Form.Item name="amountCent" label="订单金额" rules={[{ required: true, message: '请输入订单金额' }]}>
            <InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="lessonTotal" label="购买课时" rules={[{ required: true, message: '请输入课时数' }]}>
            <InputNumber min={1} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} maxLength={120} showCount placeholder="可填写报名渠道、优惠说明或家长诉求" />
          </Form.Item>
        </Form>
      </FormDrawer>

      <Modal
        title={activeAction ? `${actionTitle[activeAction.kind]} · ${activeAction.order.studentName}` : '订单操作'}
        open={Boolean(activeAction)}
        onCancel={() => setActiveAction(null)}
        onOk={() => actionForm.submit()}
        confirmLoading={submitAction.isPending}
        okButtonProps={{ danger: activeAction?.kind === 'refundAndSuspend' }}
        destroyOnClose
      >
        <Form form={actionForm} layout="vertical" onFinish={(values) => submitAction.mutate(normalizeAction(values))}>
          {activeAction && <ActionFields kind={activeAction.kind} />}
        </Form>
      </Modal>
    </div>
  );
}

function MetricCard({ title, value }: { title: string; value: string | number }) {
  return (
    <Card className="metric-card">
      <Statistic title={title} value={value} />
    </Card>
  );
}

function ActionFields({ kind }: { kind: ActionKind }) {
  if (kind === 'payment') {
    return (
      <>
        <Form.Item name="amountCent" label="收款金额" rules={[{ required: true, message: '请输入收款金额' }]}><InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="method" label="收款方式" rules={[{ required: true, message: '请选择收款方式' }]}><Select options={['线下收款', '微信支付', '支付宝', '银行卡', '现金', '其他'].map((value) => ({ label: value, value }))} /></Form.Item>
        <Form.Item name="transactionNo" label="交易单号"><Input maxLength={64} placeholder="有第三方流水时填写" /></Form.Item>
      </>
    );
  }
  if (kind === 'refund' || kind === 'refundAndSuspend') {
    return (
      <>
        {kind === 'refundAndSuspend' && <Alert type="warning" showIcon message="此操作不可自动恢复" description="将退清该订单的剩余实收金额，停用学生账号，收回该套餐权限，并从后续课次中移除该学生。部分退款请使用普通退款。" />}
        <Form.Item name="amountCent" label="退款金额" rules={[{ required: true, message: '请输入退款金额' }]}><InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="reason" label="退款原因" rules={[{ required: true, message: '请填写退款原因' }]}><Input.TextArea rows={3} maxLength={120} showCount /></Form.Item>
      </>
    );
  }
  if (kind === 'contract') {
    return (
      <>
        <Form.Item name="title" label="合同标题" rules={[{ required: true, message: '请填写合同标题' }]}><Input maxLength={80} /></Form.Item>
        <Form.Item name="signer" label="签署人" rules={[{ required: true, message: '请填写签署人' }]}><Input maxLength={40} /></Form.Item>
      </>
    );
  }
  if (kind === 'invoice') {
    return (
      <>
        <Form.Item name="title" label="发票抬头" rules={[{ required: true, message: '请填写发票抬头' }]}><Input maxLength={80} /></Form.Item>
        <Form.Item name="taxNo" label="税号"><Input maxLength={40} placeholder="个人发票可不填" /></Form.Item>
        <Form.Item name="amountCent" label="开票金额" rules={[{ required: true, message: '请输入开票金额' }]}><InputNumber min={1} precision={0} addonAfter="分" style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="invoiceNo" label="发票号码"><Input maxLength={40} /></Form.Item>
      </>
    );
  }
  if (kind === 'lesson') {
    return (
      <>
        <Form.Item name="scheduleClassId" label="关联排课"><Input maxLength={64} placeholder="可填写排课 ID，线下补课可留空" /></Form.Item>
        <Form.Item name="lessonCount" label="消耗课时" rules={[{ required: true, message: '请输入课时数' }]}><InputNumber min={1} precision={0} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="remark" label="课消备注"><Input.TextArea rows={3} maxLength={120} showCount /></Form.Item>
      </>
    );
  }
  if (kind === 'renewal') {
    return (
      <>
        <Form.Item name="reason" label="跟进原因" rules={[{ required: true, message: '请填写跟进原因' }]}><Input.TextArea rows={3} maxLength={120} showCount /></Form.Item>
        <Form.Item name="dueAt" label="跟进时间" rules={[{ required: true, message: '请填写跟进时间' }]}><Input placeholder="例如 2026-07-01 10:00" /></Form.Item>
      </>
    );
  }
  return (
    <>
      <Form.Item name="title" label="通知标题" rules={[{ required: true, message: '请填写通知标题' }]}><Input maxLength={60} /></Form.Item>
      <Form.Item name="content" label="通知内容" rules={[{ required: true, message: '请填写通知内容' }]}><Input.TextArea rows={4} maxLength={240} showCount /></Form.Item>
    </>
  );
}

function normalizeOrder(values: CommercialOrderCreateRequest): CommercialOrderCreateRequest {
  return {
    ...values,
    amountCent: Number(values.amountCent || 0),
    lessonTotal: Number(values.lessonTotal || 0),
    remark: values.remark || ''
  };
}

function normalizeAction(values: Record<string, unknown>) {
  return {
    ...values,
    amountCent: Number(values.amountCent || 0),
    lessonCount: Number(values.lessonCount || 0)
  };
}

function moneyText(value: number) {
  return `¥${(Number(value || 0) / 100).toFixed(2)}`;
}

function statusColor(status: string) {
  if (status === '已支付') return 'green';
  if (status === '待续费') return 'orange';
  if (status === '已退款') return 'red';
  return 'blue';
}

function canRefundAndSuspend(order: CommercialOrder, studentStatusByID: Map<string, string>) {
  return order.paidAmountCent > order.refundedAmountCent && order.status !== '已退款' && studentStatusByID.get(order.studentId) === '正常';
}
