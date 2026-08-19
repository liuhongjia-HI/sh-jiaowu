import { Alert, Button, Card, Empty, Form, Input, Modal, Skeleton, Space, Table, Typography, message } from 'antd';
import { EditOutlined, ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import type { SettingUpdateRequest } from '../../types/starline';

const labels: Record<string, string> = { academicYear: '当前学年', academicYearStart: '校历学年开始日期', academicYearEnd: '校历学年结束日期', grades: '适用年级', semesters: '学期设置', grantDefaultStart: '套餐默认开始日期', grantDefaultEnd: '套餐默认结束日期', academicPeriods: '期中/期末时间段', watermarkRule: '水印规则', downloadPolicy: '下载策略', miniProgramDomainStatus: '小程序域名状态', productionApiDomain: '生产接口域名', officialAccountBindingStatus: '公众号绑定状态', templateMessageStatus: '模板消息状态' };

// 校历直接决定新开通套餐的到期日，单独给一句说明，避免管理员不清楚改了会影响什么。
const hints: Record<string, string> = {
  academicYearStart: '校历学年的开学日期，格式 YYYY-MM-DD。',
  academicYearEnd: '校历学年的结束日期。新开通的学习套餐默认在这一天到期，开通当天即生效；已开通的记录不受影响，单个学生的有效期仍可在开通时单独调整。'
};
const order = Object.keys(labels);

export default function SettingsPage() {
  const [form] = Form.useForm<SettingUpdateRequest>();
  const [editing, setEditing] = useState<Record<string, string> | null>(null);
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => getData<Record<string, string>>('/settings') });
  const save = useMutation({ mutationFn: (values: SettingUpdateRequest) => putData<Record<string, string>>('/settings', values), onSuccess: () => { message.success('系统设置已保存。'); setEditing(null); form.resetFields(); queryClient.invalidateQueries({ queryKey: ['settings'] }); queryClient.invalidateQueries({ queryKey: ['logs'] }); }, onError: () => message.error('保存设置失败，请检查设置值。') });
  const rows = Object.entries(settings.data ?? {}).map(([key, value]) => ({ key, value })).sort((a, b) => order.indexOf(a.key) - order.indexOf(b.key));
  return <div className="page-stack"><div className="page-heading"><div><Typography.Title level={3}>系统设置</Typography.Title><Typography.Text type="secondary">维护学年、水印、访问和提醒规则。</Typography.Text></div></div>{settings.isLoading ? <Skeleton active /> : settings.error ? <Alert type="error" message="系统设置加载失败，请稍后重试。" /> : <><Card extra={<ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => settings.refetch()} />}>{rows.length === 0 ? <Empty description="还没有系统设置。" /> : <Table rowKey="key" pagination={false} dataSource={rows} columns={[{ title: '设置项', dataIndex: 'key', render: (key) => labels[key] ?? key }, { title: '当前值', dataIndex: 'value' }, { title: '操作', render: (_, row) => <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => { setEditing(row); form.setFieldsValue(row); }} /> }]} />}</Card><Modal title={`编辑${editing ? labels[editing.key] ?? editing.key : '设置'}`} open={Boolean(editing)} onCancel={() => setEditing(null)} footer={<Space><Button onClick={() => setEditing(null)}>取消</Button><Button type="primary" loading={save.isPending} onClick={() => form.submit()}>保存</Button></Space>}><Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)}><Form.Item name="key" hidden><Input /></Form.Item><Form.Item name="value" label="当前值" extra={editing ? hints[editing.key] : undefined} rules={[{ required: true, message: '请输入设置值' }]}><Input.TextArea rows={4} /></Form.Item></Form></Modal></>}</div>;
}
