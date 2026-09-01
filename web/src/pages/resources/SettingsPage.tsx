import { Alert, Button, Card, Empty, Form, Input, InputNumber, Popconfirm, Select, Skeleton, Space, Switch, Table, Tabs, Tag, Typography, message } from 'antd';
import { CalendarOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined, ReloadOutlined, SafetyOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, putData } from '../../services/http';
import { FormDrawer } from '../../components/FormDrawer';
import { ActionButton } from '../../components/ListViews';
import type { SettingUpdateRequest, SubjectMetadata, SubjectMetadataUpdateRequest } from '../../types/starline';

const CALENDAR_KEY = 'academicCalendar';
const FALL_LABEL = 'S1 第一学期';
const SPRING_LABEL = 'S2 第二学期';

// 三个 Tab 分组：系统设置项一多，摊平成一张大表格谁都懒得找。分组按“运营会想在什么场景下打开这一项”来划，
// 不按数据类型分——校历天天要看，接入状态一年调一次，放在一起只会互相淹没。
const contentKeys = ['grades', 'semesters', 'watermarkRule', 'downloadPolicy', 'launchCampaign'];
const integrationKeys = ['miniProgramDomainStatus', 'officialAccountBindingStatus', 'templateMessageStatus', 'miniProgramSubscribeStatus', 'productionApiDomain'];

const labels: Record<string, string> = {
  grades: '适用年级',
  semesters: '学期设置',
  watermarkRule: '水印规则',
  downloadPolicy: '下载规则',
  miniProgramDomainStatus: '小程序域名状态',
  miniProgramSubscribeStatus: '小程序订阅消息状态',
  productionApiDomain: '生产接口域名',
  officialAccountBindingStatus: '公众号绑定状态',
  templateMessageStatus: '模板消息状态'
  ,launchCampaign: '开屏营销活动配置'
};

function launchFields(raw: string) {
  try { const v = JSON.parse(raw || '{}'); return { key: 'launchCampaign', enabled: !!v.enabled, templateType: v.templateType || 'generic', title: v.title || '', message: v.message || '', subMessage: v.subMessage || '', imageUrl: v.imageUrl || '', primaryActionText: v.primaryActionText || '立即了解', timeOptionsText: Array.isArray(v.timeOptions) ? v.timeOptions.join('\n') : '', frequency: v.frequency || 'once', priority: Number(v.priority) || 0, startsAt: v.startsAt || '', endsAt: v.endsAt || '' }; } catch { return { key: 'launchCampaign', enabled: false, templateType: 'generic', title: '', message: '', subMessage: '', imageUrl: '', primaryActionText: '立即了解', timeOptionsText: '', frequency: 'once', priority: 0, startsAt: '', endsAt: '' }; }
}

type AcademicTerm = {
  academicYear: string;
  semester: string;
  startDate: string;
  endDate: string;
};

// 培训机构运营脑子里没有“学期条目”这个概念，只有“这一学年，秋季学期哪天到哪天、
// 春季学期哪天到哪天”。后端仍然按学期条目存（每学年两条），但界面按学年折成一行，
// 一行四个日期，不需要先搞懂“学期”是什么、也不需要一年点两次“新增”。
type AcademicYearRow = {
  academicYear: string;
  fallStart: string;
  fallEnd: string;
  springStart: string;
  springEnd: string;
};

function parseCalendar(raw: string | undefined): AcademicTerm[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

// 按学年分组，每组内按“语义”找秋季/春季学期，而不是按数组顺序——顺序在编辑几次之后就不可靠了。
// semester 文案不严格等于 FALL_LABEL/SPRING_LABEL 时（比如手工改过措辞），退回按开始日期早晚区分，
// 保证旧数据、别人手改过的数据也能正常显示，不会因为文案对不上就整行消失。
function groupIntoYearRows(terms: AcademicTerm[]): AcademicYearRow[] {
  const byYear = new Map<string, AcademicTerm[]>();
  terms.forEach((term) => {
    const list = byYear.get(term.academicYear) ?? [];
    list.push(term);
    byYear.set(term.academicYear, list);
  });
  const rows: AcademicYearRow[] = [];
  byYear.forEach((list, academicYear) => {
    const sorted = [...list].sort((a, b) => a.startDate.localeCompare(b.startDate));
    const fall = sorted.find((t) => t.semester.trim().toUpperCase().startsWith('S1')) ?? sorted[0];
    const spring = sorted.find((t) => t !== fall && t.semester.trim().toUpperCase().startsWith('S2')) ?? sorted.find((t) => t !== fall);
    rows.push({
      academicYear,
      fallStart: fall?.startDate ?? '',
      fallEnd: fall?.endDate ?? '',
      springStart: spring?.startDate ?? '',
      springEnd: spring?.endDate ?? ''
    });
  });
  return rows.sort((a, b) => b.academicYear.localeCompare(a.academicYear));
}

// 把一学年的两个学期日期落回“学期条目”数组，替换掉这个学年原来的条目（不影响其他学年）。
function upsertYearRow(base: AcademicTerm[], originalYear: string | null, values: AcademicYearRow): AcademicTerm[] {
  const targetYear = originalYear ?? values.academicYear;
  const kept = base.filter((term) => term.academicYear !== targetYear);
  const nextTerms: AcademicTerm[] = [...kept];
  if (values.fallStart && values.fallEnd) {
    nextTerms.push({ academicYear: values.academicYear, semester: FALL_LABEL, startDate: values.fallStart, endDate: values.fallEnd });
  }
  if (values.springStart && values.springEnd) {
    nextTerms.push({ academicYear: values.academicYear, semester: SPRING_LABEL, startDate: values.springStart, endDate: values.springEnd });
  }
  return nextTerms;
}

// 新增学年时预填一套默认日期（9/1 开学、1/15 期末、2/1 开学、7/15 结课），
// 运营只需要按本校日历微调，不用从空白表单一个个字段推敲格式。学年名从最新一行的年份 +1 推出来。
function suggestNextYear(rows: AcademicYearRow[]): AcademicYearRow {
  const latest = rows[0]?.academicYear ?? '';
  const match = latest.match(/(\d{4})\D+(\d{4})/);
  const startYear = match ? Number(match[1]) + 1 : new Date().getFullYear();
  return {
    academicYear: `${startYear}.${startYear + 1}学年`,
    fallStart: `${startYear}-09-01`,
    fallEnd: `${startYear + 1}-01-15`,
    springStart: `${startYear + 1}-02-01`,
    springEnd: `${startYear + 1}-07-15`
  };
}

function DateField(props: { value?: string; onChange?: (value: string) => void }) {
  return <Input type="date" value={props.value ?? ''} onChange={(event) => props.onChange?.(event.target.value)} />;
}

function AcademicCalendarCard({
  rawValue,
  onSave,
  saving
}: {
  rawValue: string | undefined;
  onSave: (value: string) => void;
  saving: boolean;
}) {
  const [form] = Form.useForm<AcademicYearRow>();
  const [modalOpen, setModalOpen] = useState(false);
  const [originalYear, setOriginalYear] = useState<string | null>(null);

  const terms = parseCalendar(rawValue);
  const rows = groupIntoYearRows(terms);

  function openAdd() {
    setOriginalYear(null);
    form.setFieldsValue(suggestNextYear(rows));
    setModalOpen(true);
  }

  function openEdit(row: AcademicYearRow) {
    setOriginalYear(row.academicYear);
    form.setFieldsValue(row);
    setModalOpen(true);
  }

  function close() {
    setModalOpen(false);
    setOriginalYear(null);
  }

  function submit(values: AcademicYearRow) {
    onSave(JSON.stringify(upsertYearRow(terms, originalYear, values)));
    close();
  }

  function removeYear(academicYear: string) {
    onSave(JSON.stringify(terms.filter((term) => term.academicYear !== academicYear)));
  }

  return (
    <Card title="学年校历" extra={<Button type="primary" size="small" icon={<PlusOutlined />} onClick={openAdd}>新增学年</Button>}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        每学年一行，填秋季、春季学期的起止日期即可，可以提前配好下一学年。教务开通课程时当天自动生效，并在当前学年最后一天到期；已开通记录不受影响。
      </Typography.Paragraph>
      {rows.length === 0 ? (
        <Empty description="还没有配置校历，点击右上角新增学年。" />
      ) : (
        <Table
          rowKey="academicYear"
          pagination={false}
          dataSource={rows}
          columns={[
            { title: '学年', dataIndex: 'academicYear' },
            { title: '秋季学期', render: (_, row) => row.fallStart && row.fallEnd ? `${row.fallStart} 至 ${row.fallEnd}` : <Typography.Text type="secondary">未设置</Typography.Text> },
            { title: '春季学期', render: (_, row) => row.springStart && row.springEnd ? `${row.springStart} 至 ${row.springEnd}` : <Typography.Text type="secondary">未设置</Typography.Text> },
            {
              title: '操作',
              render: (_, row) => (
                <Space>
                  <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(row)} />
                  <Popconfirm title={`删除${row.academicYear}的校历？`} okText="删除" cancelText="取消" onConfirm={() => removeYear(row.academicYear)}>
                    <ActionButton tooltip="删除" icon={<DeleteOutlined />} />
                  </Popconfirm>
                </Space>
              )
            }
          ]}
        />
      )}

      <FormDrawer
        title={originalYear ? `编辑${originalYear}` : '新增学年'}
        open={modalOpen}
        onCancel={close}
        onSubmit={() => form.submit()}
        submitting={saving}
      >
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item name="academicYear" label="学年" rules={[{ required: true, message: '请输入学年，例如 2026.2027学年' }]}>
            <Input placeholder="例如：2026.2027学年" />
          </Form.Item>
          <Space.Compact block>
            <Form.Item name="fallStart" label="秋季学期开始" style={{ width: '50%' }} rules={[{ required: true, message: '请选择开始日期' }]}>
              <DateField />
            </Form.Item>
            <Form.Item name="fallEnd" label="秋季学期结束" style={{ width: '50%' }} rules={[{ required: true, message: '请选择结束日期' }]}>
              <DateField />
            </Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="springStart" label="春季学期开始" style={{ width: '50%' }} rules={[{ required: true, message: '请选择开始日期' }]}>
              <DateField />
            </Form.Item>
            <Form.Item name="springEnd" label="春季学期结束" style={{ width: '50%' }} rules={[{ required: true, message: '请选择结束日期' }]}>
              <DateField />
            </Form.Item>
          </Space.Compact>
        </Form>
      </FormDrawer>
    </Card>
  );
}

function FlatSettingsCard({
  title,
  rows,
  onEdit,
  extra
}: {
  title?: string;
  rows: { key: string; value: string }[];
  onEdit: (row: { key: string; value: string }) => void;
  extra?: React.ReactNode;
}) {
  return (
    <Card title={title} extra={extra}>
      {rows.length === 0 ? (
        <Empty description="这里还没有配置项。" />
      ) : (
        <Table
          rowKey="key"
          pagination={false}
          dataSource={rows}
          columns={[
            { title: '设置项', dataIndex: 'key', render: (key) => labels[key] ?? key },
            { title: '当前值', dataIndex: 'value' },
            { title: '操作', render: (_, row) => <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => onEdit(row)} /> }
          ]}
        />
      )}
    </Card>
  );
}

function SubjectMetadataCard() {
  const [form] = Form.useForm<SubjectMetadataUpdateRequest>();
  const [editing, setEditing] = useState<SubjectMetadata | null>(null);
  const queryClient = useQueryClient();
  const subjects = useQuery({ queryKey: ['subjects'], queryFn: () => getData<SubjectMetadata[]>('/subjects') });
  const save = useMutation({
    mutationFn: (values: SubjectMetadataUpdateRequest) => putData<SubjectMetadata>(`/subjects/${editing?.id}`, values),
    onSuccess: () => {
      message.success('学科显示配置已保存。');
      setEditing(null);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey: ['subjects'] });
      queryClient.invalidateQueries({ queryKey: ['subjects-for-schedule'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: (error: Error) => message.error(error.message || '保存学科显示配置失败，请检查输入。')
  });

  function openEdit(subject: SubjectMetadata) {
    setEditing(subject);
    form.setFieldsValue({ shortLabel: subject.shortLabel, color: subject.color, sortOrder: subject.sortOrder, status: subject.status });
  }

  return (
    <Card title="学科显示配置" extra={<ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => subjects.refetch()} />}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        学科名称由课程和学习空间使用，此处只维护课表等页面的简称、颜色、排序和启用状态。
      </Typography.Paragraph>
      {subjects.isLoading ? <Skeleton active /> : subjects.error ? <Alert type="error" message="学科配置加载失败，请稍后重试。" /> : (
        <Table
          rowKey="id"
          pagination={false}
          dataSource={subjects.data ?? []}
          columns={[
            { title: '学科', dataIndex: 'name' },
            { title: '简称', dataIndex: 'shortLabel' },
            { title: '显示颜色', dataIndex: 'color', render: (color: string) => <Space size={8}><span aria-label={`颜色 ${color}`} style={{ width: 18, height: 18, borderRadius: '50%', background: color, border: '1px solid #d9d9d9', display: 'inline-block' }} /><Typography.Text>{color}</Typography.Text></Space> },
            { title: '排序', dataIndex: 'sortOrder' },
            { title: '状态', dataIndex: 'status', render: (status: SubjectMetadata['status']) => <Tag color={status === '启用' ? 'green' : 'default'}>{status}</Tag> },
            { title: '操作', render: (_: unknown, row: SubjectMetadata) => <ActionButton tooltip="编辑显示配置" icon={<EditOutlined />} onClick={() => openEdit(row)} /> }
          ]}
        />
      )}
      <FormDrawer
        title={editing ? `编辑${editing.name}显示配置` : '编辑学科显示配置'}
        open={Boolean(editing)}
        onCancel={() => setEditing(null)}
        onSubmit={() => form.submit()}
        submitting={save.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)}>
          <Form.Item name="shortLabel" label="显示简称" rules={[{ required: true, message: '请输入显示简称' }, { max: 20, message: '显示简称不能超过20个字符' }]}>
            <Input placeholder="例如：Eng" />
          </Form.Item>
          <Form.Item name="color" label="显示颜色" rules={[{ required: true, message: '请选择显示颜色' }, { pattern: /^#[0-9a-fA-F]{6}$/, message: '颜色格式无效' }]}>
            <Input type="color" />
          </Form.Item>
          <Form.Item name="sortOrder" label="展示排序" rules={[{ required: true, message: '请输入展示排序' }]}>
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true, message: '请选择状态' }]}>
            <Select options={[{ label: '启用', value: '启用' }, { label: '停用', value: '停用' }]} />
          </Form.Item>
        </Form>
      </FormDrawer>
    </Card>
  );
}

export default function SettingsPage() {
  const [form] = Form.useForm<SettingUpdateRequest>();
  const [editing, setEditing] = useState<Record<string, string> | null>(null);
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => getData<Record<string, string>>('/settings') });

  const save = useMutation({
    mutationFn: (values: SettingUpdateRequest) => putData<Record<string, string>>('/settings', values),
    onSuccess: () => {
      message.success('系统设置已保存。');
      setEditing(null);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: () => message.error('保存设置失败，请检查设置值。')
  });

  function saveCalendar(value: string) {
    save.mutate({ key: CALENDAR_KEY, value });
  }

  function openEdit(row: { key: string; value: string }) {
    setEditing(row);
    form.setFieldsValue(row.key === 'launchCampaign' ? launchFields(row.value) : row);
  }

  const allEntries = Object.entries(settings.data ?? {}).filter(([key]) => key !== CALENDAR_KEY && key !== 'academicYear' && key !== 'academicPeriods');
  const toRows = (keys: string[]) => allEntries.filter(([key]) => keys.includes(key)).map(([key, value]) => ({ key, value })).sort((a, b) => keys.indexOf(a.key) - keys.indexOf(b.key));
  const knownKeys = new Set([...contentKeys, ...integrationKeys]);
  const contentRows = toRows(contentKeys);
  // 未来加新设置项时，只要没来得及分组，先兜底出现在“内容与安全”里，不会莫名其妙从页面上消失。
  const otherRows = allEntries.filter(([key]) => !knownKeys.has(key)).map(([key, value]) => ({ key, value }));
  const integrationRows = toRows(integrationKeys);

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>系统设置</Typography.Title>
          <Typography.Text type="secondary">维护学年、水印、访问和提醒规则。</Typography.Text>
        </div>
      </div>
      {settings.isLoading ? (
        <Skeleton active />
      ) : settings.error ? (
        <Alert type="error" message="系统设置加载失败，请稍后重试。" />
      ) : (
        <>
          <Tabs
            defaultActiveKey="calendar"
            items={[
              {
                key: 'calendar',
                label: <span><CalendarOutlined /> 学年校历</span>,
                children: <AcademicCalendarCard rawValue={settings.data?.[CALENDAR_KEY]} onSave={saveCalendar} saving={save.isPending} />
              },
              {
                key: 'content',
                label: <span><SafetyOutlined /> 内容与安全</span>,
                children: <Space direction="vertical" size={16} style={{ width: '100%' }}><FlatSettingsCard title="资料保护" rows={[...contentRows, ...otherRows]} onEdit={openEdit} /><SubjectMetadataCard /></Space>
              },
              {
                key: 'integration',
                label: <span><LinkOutlined /> 小程序与公众号</span>,
                children: (
                  <FlatSettingsCard
                    rows={integrationRows}
                    onEdit={openEdit}
                    extra={<ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => settings.refetch()} />}
                  />
                )
              }
            ]}
          />

          <FormDrawer
            title={`编辑${editing ? labels[editing.key] ?? editing.key : '设置'}`}
            open={Boolean(editing)}
            onCancel={() => setEditing(null)}
            onSubmit={() => form.submit()}
            submitting={save.isPending}
          >
            <Form form={form} layout="vertical" onFinish={(values) => {
              if (editing?.key === 'launchCampaign') {
                const v = values as any;
                save.mutate({ key: 'launchCampaign', value: JSON.stringify({ enabled: !!v.enabled, templateType: v.templateType || 'generic', title: v.title || '', message: v.message || '', subMessage: v.subMessage || '', imageUrl: v.imageUrl || '', primaryActionText: v.primaryActionText || '立即了解', timeOptions: String(v.timeOptionsText || '').split('\n').map((x) => x.trim()).filter(Boolean), frequency: v.frequency || 'once', priority: Number(v.priority) || 0, startsAt: v.startsAt || '', endsAt: v.endsAt || '' }) });
              } else save.mutate(values);
            }}>
              <Form.Item name="key" hidden><Input /></Form.Item>
              {editing?.key === 'launchCampaign' ? <>
                <Form.Item name="enabled" label="启用开屏活动" valuePropName="checked"><Switch /></Form.Item>
                <Form.Item name="templateType" label="活动模板"><Select options={[{ value: 'generic', label: '通用图文' }, { value: 'small_class_reservation', label: '小班课预约' }]} /></Form.Item>
                <Form.Item name="title" label="标题"><Input maxLength={40} showCount /></Form.Item>
                <Form.Item name="message" label="正文"><Input.TextArea rows={3} maxLength={120} showCount /></Form.Item>
                <Form.Item name="subMessage" label="辅助说明"><Input.TextArea rows={2} maxLength={80} showCount /></Form.Item>
                <Form.Item name="imageUrl" label="图片地址"><Input placeholder="可选" /></Form.Item>
                <Form.Item name="primaryActionText" label="按钮文案"><Input maxLength={12} /></Form.Item>
                <Form.Item name="timeOptionsText" label="可选上课时间（每行一个）"><Input.TextArea rows={3} placeholder="工作日晚上\n周六上午" /></Form.Item>
                <Space align="start"><Form.Item name="frequency" label="展示频次"><Select style={{ width: 160 }} options={[{ value: 'once', label: '活动期间一次' }, { value: 'daily', label: '每天一次' }, { value: 'every_entry', label: '每次进入首页' }]} /></Form.Item><Form.Item name="priority" label="优先级"><InputNumber min={0} max={99} /></Form.Item></Space>
                <Space align="start"><Form.Item name="startsAt" label="开始时间"><Input placeholder="YYYY-MM-DD HH:mm:ss" /></Form.Item><Form.Item name="endsAt" label="结束时间"><Input placeholder="YYYY-MM-DD HH:mm:ss" /></Form.Item></Space>
              </> : <Form.Item name="value" label="当前值" rules={[{ required: true, message: '请输入设置值' }]}> 
                {editing?.key === 'downloadPolicy' ? (
                  <Select
                    options={[
                      { label: '仅在线预览（推荐）', value: '仅在线预览' },
                      { label: '允许下载带水印 PDF', value: '允许下载带水印PDF' }
                    ]}
                  />
                ) : <Input.TextArea rows={4} />}
              </Form.Item>}
            </Form>
          </FormDrawer>
        </>
      )}
    </div>
  );
}
