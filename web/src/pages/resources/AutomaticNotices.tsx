import { Alert, Button, Card, Empty, Input, Modal, Select, Skeleton, Space, Switch, Table, Tag, Tooltip, Typography, message } from 'antd';
import { ReloadOutlined, SyncOutlined, TeamOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { getData, postData, putData } from '../../services/http';
import type { OfficialTemplate } from '../../types/starline';

type Binding = { kind: string; title: string; templateId: string; enabled: boolean; enabledAt?: string; ready: boolean; reason?: string; requiredFields: Record<string, string>; studentIds?: string[]; approvedReasons?: string[] };
type Task = { id: string; eventId: string; kind: string; studentId: string; studentName: string; guardianName: string; status: string; failureReason?: string; dueAt: string; createdAt: string; acceptedAt?: string; deliveredAt?: string; attempts: number; retryable: boolean };
type StudentOption = { id: string; name: string; grade: string };

const timing: Record<string, string> = { schedule_confirmed: '首次正式确认后', schedule_changed: '调课正式生效后', schedule_reminder: '课前 2 小时', schedule_cancelled: '取消后', homework_submitted: '提交保存成功后', review_exception: '老师标记异常后', homework_published: '作业发布后', review_completed: '最终批改完成后' };
const dateText = (value?: string) => value ? new Date(value).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai', hour12: false }) : '-';
function color(status: string) { return status === '已送达' ? 'success' : status === '微信已受理' || status === '发送中' ? 'processing' : ['发送失败', '配置错误'].includes(status) ? 'error' : status === '结果待确认' || status === '不可触达' ? 'warning' : 'default'; }
function matchingTemplate(binding: Binding, template: OfficialTemplate) { const fields = template.fields.map((field) => field.key).sort(); const required = Object.keys(binding.requiredFields).sort(); return fields.length > 0 && fields.join('|') === required.join('|'); }

export default function AutomaticNotices() {
  const queryClient = useQueryClient();
  const bindings = useQuery({ queryKey: ['automatic-notice-bindings'], queryFn: () => getData<Binding[]>('/official-account/automatic-notices') });
  const templates = useQuery({ queryKey: ['official-templates'], queryFn: () => getData<OfficialTemplate[]>('/official-account/templates') });
  const students = useQuery({ queryKey: ['automatic-notice-students'], queryFn: () => getData<StudentOption[]>('/students') });
  const tasks = useQuery({ queryKey: ['automatic-notice-tasks'], queryFn: () => getData<Task[]>('/official-account/automatic-notice-tasks'), refetchInterval: 10000 });
  const [keyword, setKeyword] = useState('');
  const [kind, setKind] = useState<string>();
  const [status, setStatus] = useState<string>();
  const [editing, setEditing] = useState<Binding>();
  const update = useMutation({ mutationFn: (binding: Binding) => putData<Binding[]>(`/official-account/automatic-notices/${binding.kind}`, binding), onSuccess: (rows) => { queryClient.setQueryData(['automatic-notice-bindings'], rows); queryClient.invalidateQueries({ queryKey: ['automatic-notice-tasks'] }); setEditing(undefined); message.success('配置已保存'); }, onError: (error: Error) => message.error(error.message) });
  const retry = useMutation({ mutationFn: (id: string) => postData(`/official-account/automatic-notice-tasks/${id}/retry`, {}), onSuccess: () => { message.success('已加入发送队列'); queryClient.invalidateQueries({ queryKey: ['automatic-notice-tasks'] }); }, onError: (error: Error) => message.error(error.message) });
  const sync = useMutation({ mutationFn: () => postData<OfficialTemplate[]>('/official-account/templates/sync', {}), onSuccess: (rows) => { queryClient.setQueryData(['official-templates'], rows); queryClient.invalidateQueries({ queryKey: ['automatic-notice-bindings'] }); message.success('模板已同步'); }, onError: (error: Error) => message.error(error.message) });
  const followers = useMutation({ mutationFn: () => postData<{ subscribedCount: number; matchedCount: number }>('/official-account/followers/sync', {}), onSuccess: (result) => message.success(`已关注 ${result.subscribedCount} 位，身份匹配 ${result.matchedCount} 位`), onError: (error: Error) => message.error(error.message) });
  const rows = useMemo(() => (tasks.data ?? []).filter((task) => (!kind || task.kind === kind) && (!status || task.status === status) && (!keyword || `${task.studentName} ${task.guardianName} ${task.failureReason ?? ''}`.includes(keyword.trim()))), [tasks.data, kind, status, keyword]);
  const titles = Object.fromEntries((bindings.data ?? []).map((binding) => [binding.kind, binding.title]));

  function save(binding: Binding) {
    if (binding.enabled && !(bindings.data ?? []).find((item) => item.kind === binding.kind)?.enabled) {
      Modal.confirm({ title: `开启${binding.title}？`, content: binding.studentIds?.length ? `将对所选 ${binding.studentIds.length} 名学生的已关联家长自动推送。` : '将对所有符合条件的学生家长自动推送，历史业务通知不会补发。', okText: '开启', onOk: () => update.mutateAsync(binding) });
    } else update.mutate(binding);
  }

  if (bindings.isLoading) return <Skeleton active />;
  if (bindings.error) return <Alert type="error" showIcon message="自动通知配置加载失败" action={<Button onClick={() => bindings.refetch()}>重试</Button>} />;
  return <div className="page-stack">
    <Card title="自动通知" extra={<Space wrap><Button icon={<TeamOutlined />} loading={followers.isPending} onClick={() => followers.mutate()}>同步关注者</Button><Button icon={<SyncOutlined />} loading={sync.isPending} onClick={() => sync.mutate()}>同步模板</Button></Space>}>
      <Table rowKey="kind" dataSource={bindings.data ?? []} pagination={false} scroll={{ x: 850 }} columns={[
        { title: '业务', dataIndex: 'title', width: 140 },
        { title: '模板', render: (_, binding: Binding) => templates.data?.find((item) => item.id === binding.templateId)?.title || '-' },
        { title: '发送时机', render: (_, binding: Binding) => <Tooltip title={binding.kind === 'schedule_reminder' ? '普通提醒在 08:00—21:00 发送，窗口外提前到前一日 20:00' : undefined}>{timing[binding.kind]}</Tooltip>, width: 170 },
        { title: '接收范围', render: (_, binding: Binding) => binding.studentIds?.length ? `${binding.studentIds.length} 名学生` : '所有学生', width: 120 },
        { title: '状态', render: (_, binding: Binding) => <Tooltip title={binding.reason}><Tag color={!binding.ready ? 'warning' : binding.enabled ? 'success' : 'default'}>{!binding.ready ? '待配置' : binding.enabled ? '已开启' : '已关闭'}</Tag></Tooltip>, width: 100 },
        { title: '操作', render: (_, binding: Binding) => <Button size="small" disabled={!'schedule_confirmed schedule_changed schedule_reminder homework_submitted'.split(' ').includes(binding.kind)} onClick={() => setEditing({ ...binding })}>配置</Button>, width: 80 }
      ]} />
    </Card>
    <Card title="自动发送记录">
      <Space wrap style={{ marginBottom: 16 }}><Input.Search allowClear placeholder="学生、家长或失败原因" value={keyword} onChange={(event) => setKeyword(event.target.value)} /><Select allowClear placeholder="业务类型" value={kind} onChange={setKind} style={{ width: 160 }} options={(bindings.data ?? []).map((binding) => ({ value: binding.kind, label: binding.title }))} /><Select allowClear placeholder="发送状态" value={status} onChange={setStatus} style={{ width: 150 }} options={['待发送', '发送中', '微信已受理', '已送达', '发送失败', '配置错误', '不可触达', '结果待确认', '已失效'].map((value) => ({ value, label: value }))} /><Button icon={<ReloadOutlined />} onClick={() => tasks.refetch()}>刷新</Button></Space>
      {tasks.error ? <Alert type="error" showIcon message="发送记录加载失败" action={<Button onClick={() => tasks.refetch()}>重试</Button>} /> : <Table rowKey="id" loading={tasks.isLoading} dataSource={rows} pagination={{ pageSize: 10 }} scroll={{ x: 1000 }} locale={{ emptyText: <Empty description="暂无发送记录" /> }} columns={[
        { title: '学生', dataIndex: 'studentName', width: 110 }, { title: '家长', dataIndex: 'guardianName', width: 110 }, { title: '业务', render: (_, task: Task) => titles[task.kind], width: 120 },
        { title: '计划时间', dataIndex: 'dueAt', render: dateText, width: 185 }, { title: '状态', dataIndex: 'status', render: (value: string) => <Tag color={color(value)}>{value}</Tag>, width: 120 },
        { title: '说明', render: (_, task: Task) => task.failureReason || (task.deliveredAt ? `送达 ${dateText(task.deliveredAt)}` : task.acceptedAt ? `受理 ${dateText(task.acceptedAt)}` : '-'), ellipsis: true },
        { title: '尝试', dataIndex: 'attempts', width: 65 }, { title: '操作', render: (_, task: Task) => task.retryable ? <Button type="link" loading={retry.isPending} onClick={() => retry.mutate(task.id)}>补发</Button> : '-', width: 75 }
      ]} />}
    </Card>
    <Modal title={editing ? `${editing.title}配置` : '通知配置'} open={!!editing} onCancel={() => setEditing(undefined)} onOk={() => editing && save(editing)} confirmLoading={update.isPending} okButtonProps={{ disabled: !editing || (editing.enabled && (!editing.templateId || Object.keys(editing.requiredFields).length === 0)) }}>
      {editing && <Space direction="vertical" size="large" style={{ width: '100%' }}>
        {Object.keys(editing.requiredFields).length === 0 ? <Alert showIcon type="info" message="需先补齐此业务的模板和字段映射" /> : <><div><Typography.Text>公众号模板</Typography.Text><Select style={{ width: '100%', marginTop: 8 }} value={editing.templateId || undefined} placeholder="选择匹配模板" options={(templates.data ?? []).map((template) => ({ label: template.title, value: template.id, disabled: !matchingTemplate(editing, template) }))} onChange={(templateId) => setEditing({ ...editing, templateId })} /></div><div><Typography.Text>接收范围</Typography.Text><Select mode="multiple" allowClear optionFilterProp="label" style={{ width: '100%', marginTop: 8 }} placeholder="所有学生；试运行时选择学生" value={editing.studentIds ?? []} options={(students.data ?? []).map((student) => ({ value: student.id, label: `${student.name} · ${student.grade}` }))} onChange={(studentIds) => setEditing({ ...editing, studentIds })} /></div><Space><Typography.Text>自动推送</Typography.Text><Switch checked={editing.enabled} onChange={(enabled) => setEditing({ ...editing, enabled })} /></Space></>}
        {editing.reason && <Typography.Text type="secondary">{editing.reason}</Typography.Text>}
      </Space>}
    </Modal>
  </div>;
}
