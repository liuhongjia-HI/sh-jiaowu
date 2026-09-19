import { Alert, Button, Card, Empty, Form, Input, Modal, Pagination, Select, Skeleton, Space, Table, Tabs, Tag, Typography, message } from 'antd';
import { CheckCircleFilled, PlusOutlined, ReloadOutlined, SendOutlined, SyncOutlined, TeamOutlined } from '@ant-design/icons';
import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData } from '../../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../../components/ListViews';
import { NoticeDialog } from './ResourceDialogs';
import { gradeOptions } from '../../utils/curriculum';
import type { CurrentUser, NoticeCreateRequest, OfficialAudiencePreview, OfficialCampaign, OfficialTemplate } from '../../types/starline';

type NoticeRow = Record<string, unknown>;
type CampaignCreateBody = { templateId: string; grades: string[]; values: Record<string, string>; pagePath: string; draft: boolean };
const DRAFT_KEY = 'starline:official-message-draft';

function statusColor(status: string) {
  if (status === '发送完成' || status === '发送成功') return 'success';
  if (status === '部分失败' || status === '发送失败') return 'error';
  if (status === '发送中') return 'processing';
  return 'default';
}

function WechatMessagePreview({ template, values }: { template?: OfficialTemplate; values: Record<string, string> }) {
  const titleField = template?.fields[0];
  return <div className="wechat-phone-stage"><div className="wechat-phone"><div className="wechat-phone-speaker" /><div className="wechat-phone-screen">
    <div className="wechat-statusbar"><strong>9:41</strong><span>● ● ▰</span></div>
    <div className="wechat-chat-header"><span>‹</span><strong>星线教育</strong><span>♙</span></div>
    <div className="wechat-message-time">今天 10:24</div>
    <div className="wechat-message-row"><div className="wechat-avatar">星</div><div className="wechat-template-card">
      <strong className="wechat-template-title">{titleField ? values[titleField.key] || titleField.label : '选择模板后预览消息'}</strong>
      <div className="wechat-template-fields">{template?.fields.slice(1).map((field) => <div key={field.key}><span>{field.label}</span><b>{values[field.key] || '待填写'}</b></div>)}</div>
      <div className="wechat-template-link">查看详情 <span>›</span></div>
    </div></div>
    <div className="wechat-inputbar"><span>◉</span><i /><span>☺</span><span>＋</span></div>
  </div></div></div>;
}

function OfficialComposer() {
  const queryClient = useQueryClient();
  const restored = useMemo(() => { try { return JSON.parse(localStorage.getItem(DRAFT_KEY) || '{}') as Partial<CampaignCreateBody>; } catch { return {}; } }, []);
  const [grades, setGrades] = useState<string[]>(restored.grades ?? []);
  const [templateId, setTemplateId] = useState(restored.templateId ?? '');
  const [values, setValues] = useState<Record<string, string>>(restored.values ?? {});
  const [pagePath, setPagePath] = useState(restored.pagePath ?? '/pages/notices/index');
  const templates = useQuery({ queryKey: ['official-templates'], queryFn: () => getData<OfficialTemplate[]>('/official-account/templates') });
  const audience = useQuery({ queryKey: ['official-audience-preview', grades], enabled: grades.length > 0, queryFn: () => postData<OfficialAudiencePreview>('/official-account/campaigns/preview', { grades }) });
  const syncTemplates = useMutation({ mutationFn: () => postData<OfficialTemplate[]>('/official-account/templates/sync', {}), onSuccess: (rows) => { message.success(`已同步 ${rows.length} 个公众号模板。`); queryClient.setQueryData(['official-templates'], rows); }, onError: (error: Error) => message.error(error.message || '模板同步失败。') });
  const syncFollowers = useMutation({ mutationFn: () => postData<{ subscribedCount: number; matchedCount: number }>('/official-account/followers/sync', {}), onSuccess: (result) => { message.success(`已同步 ${result.subscribedCount} 位关注者，匹配 ${result.matchedCount} 位家长。`); audience.refetch(); }, onError: (error: Error) => message.error(error.message || '关注者同步失败。') });
  const createCampaign = useMutation({ mutationFn: (body: CampaignCreateBody) => postData<OfficialCampaign>('/official-account/campaigns', body), onSuccess: (campaign) => { message.success(campaign.status === '草稿' ? '草稿已保存。' : `发送任务已创建，将发送给 ${campaign.targetCount} 位家长。`); if (campaign.status !== '草稿') localStorage.removeItem(DRAFT_KEY); queryClient.invalidateQueries({ queryKey: ['official-campaigns'] }); }, onError: (error: Error) => message.error(error.message || '操作失败。') });
  const selectedTemplate = templates.data?.find((item) => item.id === templateId);

  useEffect(() => { if (!templateId && templates.data?.length) setTemplateId(templates.data[0].id); }, [templateId, templates.data]);
  useEffect(() => { localStorage.setItem(DRAFT_KEY, JSON.stringify({ templateId, grades, values, pagePath })); }, [grades, pagePath, templateId, values]);

  function submit(draft: boolean) {
    const body = { templateId, grades, values, pagePath, draft };
    if (draft) { createCampaign.mutate(body); return; }
    Modal.confirm({ title: '确认发送公众号消息？', content: `将使用“${selectedTemplate?.title ?? ''}”发送给 ${audience.data?.reachableCount ?? 0} 位家长。发送后无法撤回。`, okText: '确认发送', cancelText: '再检查一下', onOk: () => createCampaign.mutate(body) });
  }

  if (templates.isLoading) return <Skeleton active />;
  if (templates.error) return <Alert type="error" message="公众号模板加载失败，请检查微信配置。" />;

  return <div className="official-message-layout"><Card className="official-composer-card">
    <section className="official-step"><div className="official-step-title"><span>1</span><strong>选择接收年级</strong></div>
      <Select mode="multiple" value={grades} onChange={setGrades} placeholder="请选择一个或多个年级" options={gradeOptions()} style={{ width: '100%' }} />
      {grades.length > 0 && <div className="official-audience-summary"><TeamOutlined />{audience.isLoading ? <span>正在计算可触达人数…</span> : audience.error ? <span>接收人数计算失败</span> : <div><strong>共 <em>{audience.data?.studentCount ?? 0}</em> 名学生 · 可触达 <em>{audience.data?.reachableCount ?? 0}</em> 位家长</strong><small>{audience.data?.unreachableCount ?? 0} 位家长尚未关注公众号或身份未匹配</small></div>}</div>}
    </section>
    <section className="official-step"><div className="official-step-heading"><div className="official-step-title"><span>2</span><strong>选择公众号模板</strong></div><Space><Button icon={<TeamOutlined />} loading={syncFollowers.isPending} onClick={() => syncFollowers.mutate()}>同步关注用户</Button><Button icon={<SyncOutlined />} loading={syncTemplates.isPending} onClick={() => syncTemplates.mutate()}>同步模板</Button></Space></div>
      <div className="official-template-select-row"><Select value={templateId || undefined} onChange={(id) => { setTemplateId(id); setValues({}); }} placeholder="请选择公众号模板" options={(templates.data ?? []).map((item) => ({ label: item.title, value: item.id }))} style={{ minWidth: 280 }} />{selectedTemplate && <Typography.Text type="secondary"><CheckCircleFilled style={{ color: '#07c160' }} /> 已从公众号同步 · {selectedTemplate.syncedAt || '刚刚更新'}</Typography.Text>}</div>
      {(templates.data?.length ?? 0) === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="公众号还没有同步模板" />}
    </section>
    <section className="official-step"><div className="official-step-title"><span>3</span><strong>编辑模板内容</strong><Typography.Text type="secondary">模板字段由公众号同步，填写右侧内容即可</Typography.Text></div>
      <div className="official-field-table"><div className="official-field-head"><span>模板字段</span><span>填写内容</span></div>{selectedTemplate?.fields.map((field) => <div className="official-field-row" key={field.key}><label>{field.label}</label>{field.maxLength && field.maxLength > 32 ? <Input.TextArea value={values[field.key] ?? ''} onChange={(event) => setValues((current) => ({ ...current, [field.key]: event.target.value }))} maxLength={field.maxLength} showCount autoSize={{ minRows: 2, maxRows: 4 }} placeholder={`请输入${field.label}`} /> : <Input value={values[field.key] ?? ''} onChange={(event) => setValues((current) => ({ ...current, [field.key]: event.target.value }))} maxLength={field.maxLength} placeholder={`请输入${field.label}`} />}</div>)}</div>
      <div className="official-page-path"><strong>点击消息后打开</strong><Select value={pagePath} onChange={setPagePath} options={[{ label: '小程序 · 通知中心', value: '/pages/notices/index' }, { label: '小程序 · 课程安排', value: '/pages/schedule/index' }, { label: '小程序 · 学习任务', value: '/pages/tasks/index' }]} /></div>
    </section>
    <div className="official-actionbar"><Typography.Text type="secondary"><CheckCircleFilled style={{ color: '#07c160' }} /> 草稿已自动保存在本机</Typography.Text><Space><Typography.Text type="secondary">将发送给 {audience.data?.reachableCount ?? 0} 位家长</Typography.Text><Button onClick={() => submit(true)} loading={createCampaign.isPending}>保存草稿</Button><Button type="primary" icon={<SendOutlined />} disabled={!selectedTemplate || !audience.data?.reachableCount} loading={createCampaign.isPending} onClick={() => submit(false)}>确认发送</Button></Space></div>
  </Card><Card className="official-preview-card" title={<div><strong>微信效果预览</strong><small>填写内容后实时更新</small></div>}><WechatMessagePreview template={selectedTemplate} values={values} /></Card></div>;
}

function CampaignRecords() {
  const queryClient = useQueryClient();
  const campaigns = useQuery({ queryKey: ['official-campaigns'], queryFn: () => getData<OfficialCampaign[]>('/official-account/campaigns'), refetchInterval: 5000 });
  const retry = useMutation({ mutationFn: (id: string) => postData<OfficialCampaign>(`/official-account/campaigns/${id}/retry`, {}), onSuccess: () => { message.success('已重新提交失败消息。'); queryClient.invalidateQueries({ queryKey: ['official-campaigns'] }); }, onError: (error: Error) => message.error(error.message || '重试失败。') });
  if (campaigns.isLoading) return <Skeleton active />;
  if (campaigns.error) return <Alert type="error" message="发送记录加载失败。" />;
  return <Card><Table rowKey="id" dataSource={campaigns.data ?? []} pagination={{ pageSize: 10 }} columns={[{ title: '发送时间', dataIndex: 'createdAt', width: 170 }, { title: '模板', dataIndex: 'templateTitle' }, { title: '接收年级', dataIndex: 'grades', render: (items: string[]) => items?.join('、') || '-' }, { title: '目标人数', dataIndex: 'targetCount', width: 90 }, { title: '成功', dataIndex: 'successCount', width: 80 }, { title: '失败', dataIndex: 'failureCount', width: 80 }, { title: '状态', dataIndex: 'status', width: 110, render: (status: string) => <Tag color={statusColor(status)}>{status}</Tag> }, { title: '操作', width: 90, render: (_, row) => row.failureCount > 0 ? <Button type="link" loading={retry.isPending} onClick={() => retry.mutate(row.id)}>重试失败</Button> : '-' }]} /></Card>;
}

function LegacyNotices({ onOpen }: { onOpen: () => void }) {
  const [keyword, setKeyword] = useState(''); const [page, setPage] = useState(1); const [viewMode, setViewMode] = useListViewMode('starline:list-view:notices'); const queryClient = useQueryClient();
  const notices = useQuery({ queryKey: ['notices'], queryFn: () => getData<NoticeRow[]>('/notices') });
  const retry = useMutation({ mutationFn: (row: NoticeRow) => postData(`/notices/${row.id}/retry`, {}), onSuccess: () => { message.success('通知已补发。'); queryClient.invalidateQueries({ queryKey: ['notices'] }); }, onError: (error: Error) => message.error(error.message || '补发失败。') });
  const filtered = useMemo(() => { const term = keyword.trim().toLowerCase(); return (notices.data ?? []).filter((row) => !term || Object.values(row).join(' ').toLowerCase().includes(term)); }, [keyword, notices.data]);
  if (notices.isLoading) return <Skeleton active />; if (notices.error) return <Alert type="error" message="通知提醒加载失败，请稍后重试。" />;
  const rows = filtered.slice((page - 1) * 10, page * 10); const retryAction = (row: NoticeRow) => row.channel === '公众号模板消息' && ['发送中', '发送失败', '待配置'].includes(String(row.status)) ? <ActionButton tooltip="补发" icon={<ReloadOutlined />} loading={retry.isPending} onClick={() => retry.mutate(row)} /> : <Typography.Text type="secondary">-</Typography.Text>;
  return <Card><div className="list-toolbar" style={{ marginBottom: 16 }}><Space><Input.Search allowClear placeholder="搜索站内通知" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} /><Button type="primary" icon={<PlusOutlined />} onClick={onOpen}>发送站内通知</Button></Space><ListViewToggle storageKey="starline:list-view:notices" value={viewMode} onChange={setViewMode} /></div>{viewMode === 'table' ? <Table rowKey="id" dataSource={rows} pagination={false} columns={[{ title: '类型', dataIndex: 'type' }, { title: '标题', dataIndex: 'title' }, { title: '接收对象', dataIndex: 'target' }, { title: '渠道', dataIndex: 'channel' }, { title: '状态', dataIndex: 'status', render: (status) => <Tag>{String(status ?? '-')}</Tag> }, { title: '操作', render: (_, row) => retryAction(row) }]} /> : <CardList rows={rows} rowKey={(row) => String(row.id)} emptyText={keyword ? '没有符合条件的结果' : '还没有通知提醒。'} renderCard={(row) => <InfoCard title={String(row.title ?? '通知')} subtitle={String(row.target ?? '')} fields={[{ label: '渠道', value: String(row.channel ?? '-') }, { label: '状态', value: String(row.status ?? '-') }]} actions={retryAction(row)} />} />}{filtered.length > 10 && <Pagination current={page} pageSize={10} total={filtered.length} showSizeChanger={false} onChange={setPage} style={{ marginTop: 16 }} />}</Card>;
}

export default function NoticesPage({ user }: { user: CurrentUser }) {
  const [form] = Form.useForm<NoticeCreateRequest>(); const [open, setOpen] = useState(false); const queryClient = useQueryClient(); const canUseOfficialMessaging = user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role));
  const send = useMutation({ mutationFn: (values: NoticeCreateRequest) => postData('/notices', values), onSuccess: () => { message.success('通知已发送。'); setOpen(false); form.resetFields(); queryClient.invalidateQueries({ queryKey: ['notices'] }); }, onError: (error: Error) => message.error(error.message || '发送通知失败。') });
  const openLegacy = () => { form.setFieldsValue({ type: '通知', title: '', target: '', summary: '', channel: '站内通知', recipientOpenId: '', relatedType: '', relatedId: '' }); setOpen(true); };
  return <div className="page-stack official-message-page"><div className="page-heading"><div><Typography.Title level={3}>通知提醒</Typography.Title><Typography.Text type="secondary">按年级向已关注公众号的家长发送模板消息。</Typography.Text></div>{canUseOfficialMessaging && <Button onClick={openLegacy}>发送站内通知</Button>}</div>{canUseOfficialMessaging ? <Tabs defaultActiveKey="compose" items={[{ key: 'compose', label: '消息推送', children: <OfficialComposer /> }, { key: 'records', label: '发送记录', children: <CampaignRecords /> }, { key: 'station', label: '站内通知', children: <LegacyNotices onOpen={openLegacy} /> }]} /> : <LegacyNotices onOpen={openLegacy} />}<NoticeDialog form={form} open={open} loading={send.isPending} onCancel={() => setOpen(false)} onSubmit={(formValues) => send.mutate(formValues)} /></div>;
}
