import { Alert, Button, Card, Form, Input, Pagination, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData } from '../../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../../components/ListViews';
import { NoticeDialog } from './ResourceDialogs';
import type { NoticeCreateRequest } from '../../types/starline';

type NoticeRow = Record<string, unknown>;

export default function NoticesPage() {
  const [form] = Form.useForm<NoticeCreateRequest>();
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:notices');
  const queryClient = useQueryClient();
  const notices = useQuery({ queryKey: ['notices'], queryFn: () => getData<NoticeRow[]>('/notices') });
  const send = useMutation({ mutationFn: (values: NoticeCreateRequest) => postData('/notices', values), onSuccess: () => { message.success('通知已发送。'); setOpen(false); form.resetFields(); queryClient.invalidateQueries({ queryKey: ['notices'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '发送通知失败，请检查接收对象。') });
  const retry = useMutation({ mutationFn: (row: NoticeRow) => postData(`/notices/${row.id}/retry`, {}), onSuccess: () => { message.success('通知已补发。'); queryClient.invalidateQueries({ queryKey: ['notices'] }); }, onError: (error: Error) => message.error(error.message || '补发失败，请检查通知配置。') });
  const filtered = useMemo(() => { const term = keyword.trim().toLowerCase(); return (notices.data ?? []).filter((row) => !term || Object.values(row).join(' ').toLowerCase().includes(term)); }, [keyword, notices.data]);
  if (notices.isLoading) return <Skeleton active />;
  if (notices.error) return <Alert type="error" message="通知提醒加载失败，请稍后重试。" />;
  const rows = filtered.slice((page - 1) * 10, page * 10);
  const retryAction = (row: NoticeRow) => row.channel === '公众号模板消息' && ['发送中', '发送失败', '待配置'].includes(String(row.status)) ? <ActionButton tooltip="补发" icon={<ReloadOutlined />} loading={retry.isPending} onClick={() => retry.mutate(row)} /> : <Typography.Text type="secondary">-</Typography.Text>;
  return <div className="page-stack"><div className="page-heading"><div><Typography.Title level={3}>通知提醒</Typography.Title><Typography.Text type="secondary">发送练习、批改、资料和到期提醒。</Typography.Text></div><Space><Button type="primary" icon={<PlusOutlined />} onClick={() => { form.setFieldsValue({ type: '通知', title: '', target: '', summary: '', channel: '站内通知', recipientOpenId: '', relatedType: '', relatedId: '' }); setOpen(true); }}>发送通知</Button><ListViewToggle storageKey="starline:list-view:notices" value={viewMode} onChange={setViewMode} /></Space></div><Card><div className="list-toolbar" style={{ marginBottom: 16 }}><Space><Input.Search allowClear placeholder="搜索通知" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} /><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => notices.refetch()} /></Space></div>{viewMode === 'table' ? <Table rowKey="id" dataSource={rows} pagination={false} columns={[{ title: '类型', dataIndex: 'type' }, { title: '标题', dataIndex: 'title' }, { title: '接收对象', dataIndex: 'target' }, { title: '渠道', dataIndex: 'channel' }, { title: '状态', dataIndex: 'status', render: (status) => <Tag>{String(status ?? '-')}</Tag> }, { title: '操作', render: (_, row) => retryAction(row) }]} /> : <CardList rows={rows} rowKey={(row) => String(row.id)} emptyText={keyword ? '没有符合条件的结果' : '还没有通知提醒。'} renderCard={(row) => <InfoCard title={String(row.title ?? '通知')} subtitle={String(row.target ?? '')} fields={[{ label: '渠道', value: String(row.channel ?? '-') }, { label: '状态', value: String(row.status ?? '-') }]} actions={retryAction(row)} />} />}{filtered.length > 10 && <Pagination current={page} pageSize={10} total={filtered.length} showSizeChanger={false} onChange={setPage} style={{ marginTop: 16 }} />}</Card><NoticeDialog form={form} open={open} loading={send.isPending} onCancel={() => setOpen(false)} onSubmit={(values) => send.mutate(values)} /></div>;
}
