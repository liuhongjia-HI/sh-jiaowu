import { Alert, Button, Card, Empty, Form, Input, InputNumber, Popconfirm, Select, Skeleton, Space, Table, Tabs, Tag, Typography, message } from 'antd';
import { CopyOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined, ReloadOutlined, SafetyOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteData, getData, putData, resolveApiUrl } from '../../services/http';
import { FormDrawer } from '../../components/FormDrawer';
import { ActionButton } from '../../components/ListViews';
import type { SettingUpdateRequest, SubjectMetadata, SubjectMetadataUpdateRequest, WechatSettings, WechatSettingsUpdateRequest } from '../../types/starline';
import { subjectLabel } from '../../utils/curriculum';

const CALENDAR_KEY = 'academicCalendar';
const FALL_LABEL = 'S1 第一学期';
const SPRING_LABEL = 'S2 第二学期';

// 教学相关配置已集中到“教学配置”；系统设置只保留资料保护和微信接入。
const contentKeys = ['watermarkRule'];

const labels: Record<string, string> = {
  watermarkRule: '水印规则',
  miniProgramDomainStatus: '小程序域名状态',
  miniProgramSubscribeStatus: '小程序订阅消息状态',
  productionApiDomain: '生产接口域名',
  officialAccountBindingStatus: '公众号绑定状态',
  templateMessageStatus: '模板消息状态'
};

type AcademicTerm = {
  academicYear: string;
  semester: string;
  startDate: string;
  midtermDate?: string;
  endDate: string;
};

// 培训机构运营脑子里没有“学期条目”这个概念，只有“这一学年，秋季学期哪天到哪天、
// 春季学期哪天到哪天”。后端仍然按学期条目存（每学年两条），但界面按学年折成一行，
// 一行三个日期，不需要先搞懂“学期”是什么、也不需要一年点两次“新增”。
type AcademicYearRow = {
  academicYear: string;
  fallStart: string;
  fallMidterm: string;
  fallEnd: string;
  springStart: string;
  springMidterm: string;
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
      fallMidterm: fall?.midtermDate ?? '',
      fallEnd: fall?.endDate ?? '',
      springStart: spring?.startDate ?? '',
      springMidterm: spring?.midtermDate ?? '',
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
    nextTerms.push({ academicYear: values.academicYear, semester: FALL_LABEL, startDate: values.fallStart, midtermDate: values.fallMidterm, endDate: values.fallEnd });
  }
  if (values.springStart && values.springEnd) {
    nextTerms.push({ academicYear: values.academicYear, semester: SPRING_LABEL, startDate: values.springStart, midtermDate: values.springMidterm, endDate: values.springEnd });
  }
  return nextTerms;
}

function calendarDateError(values: AcademicYearRow): string | null {
  const semesters = [
    { label: '秋季学期', start: values.fallStart, midterm: values.fallMidterm, end: values.fallEnd },
    { label: '春季学期', start: values.springStart, midterm: values.springMidterm, end: values.springEnd }
  ];
  for (const semester of semesters) {
    if (semester.midterm < semester.start || semester.midterm > semester.end) return `${semester.label}的期中日期必须在学期起止日期内。`;
  }
  return null;
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
    fallMidterm: `${startYear}-11-01`,
    fallEnd: `${startYear + 1}-01-15`,
    springStart: `${startYear + 1}-02-01`,
    springMidterm: `${startYear + 1}-04-30`,
    springEnd: `${startYear + 1}-07-15`
  };
}

function DateField(props: { value?: string; onChange?: (value: string) => void }) {
  return <Input type="date" value={props.value ?? ''} onChange={(event) => props.onChange?.(event.target.value)} />;
}

export function AcademicCalendarCard({
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
    form.setFieldsValue({
      ...row,
      fallMidterm: row.fallMidterm || row.fallStart,
      springMidterm: row.springMidterm || row.springStart
    });
    setModalOpen(true);
  }

  function close() {
    setModalOpen(false);
    setOriginalYear(null);
  }

  function submit(values: AcademicYearRow) {
    const error = calendarDateError(values);
    if (error) {
      message.error(error);
      return;
    }
    onSave(JSON.stringify(upsertYearRow(terms, originalYear, values)));
    close();
  }

  function removeYear(academicYear: string) {
    onSave(JSON.stringify(terms.filter((term) => term.academicYear !== academicYear)));
  }

  return (
    <Card title="学年校历" extra={<Button type="primary" size="small" icon={<PlusOutlined />} onClick={openAdd}>新增学年</Button>}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        每学年一行，维护秋季、春季学期的起止日期及各自的期中日期。可以提前配好下一学年；已开通记录不受影响。
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
            { title: '秋季学期', render: (_, row) => row.fallStart && row.fallEnd ? <Space direction="vertical" size={0}><span>{row.fallStart} 至 {row.fallEnd}</span><Typography.Text type="secondary">期中 {row.fallMidterm || '未设置'}</Typography.Text></Space> : <Typography.Text type="secondary">未设置</Typography.Text> },
            { title: '春季学期', render: (_, row) => row.springStart && row.springEnd ? <Space direction="vertical" size={0}><span>{row.springStart} 至 {row.springEnd}</span><Typography.Text type="secondary">期中 {row.springMidterm || '未设置'}</Typography.Text></Space> : <Typography.Text type="secondary">未设置</Typography.Text> },
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
          <Form.Item name="fallMidterm" label="秋季学期期中" rules={[{ required: true, message: '请选择期中日期' }]}>
            <DateField />
          </Form.Item>
          <Space.Compact block>
            <Form.Item name="springStart" label="春季学期开始" style={{ width: '50%' }} rules={[{ required: true, message: '请选择开始日期' }]}>
              <DateField />
            </Form.Item>
            <Form.Item name="springEnd" label="春季学期结束" style={{ width: '50%' }} rules={[{ required: true, message: '请选择结束日期' }]}>
              <DateField />
            </Form.Item>
          </Space.Compact>
          <Form.Item name="springMidterm" label="春季学期期中" rules={[{ required: true, message: '请选择期中日期' }]}>
            <DateField />
          </Form.Item>
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

export function SubjectMetadataCard() {
  const [form] = Form.useForm<SubjectMetadataUpdateRequest>();
  const [editing, setEditing] = useState<SubjectMetadata | null>(null);
  const queryClient = useQueryClient();
  const subjects = useQuery({ queryKey: ['subjects'], queryFn: () => getData<SubjectMetadata[]>('/subjects') });
  const save = useMutation({
    mutationFn: (values: SubjectMetadataUpdateRequest) => putData<SubjectMetadata>(`/subjects/${editing?.id}`, values),
    onSuccess: () => {
      message.success('学科配置已保存。');
      setEditing(null);
      form.resetFields();
      queryClient.invalidateQueries({ queryKey: ['subjects'] });
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces'] });
      queryClient.invalidateQueries({ queryKey: ['subjects-for-schedule'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces-for-packages'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces-for-content'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces-for-questions'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: (error: Error) => message.error(error.message || '保存学科配置失败，请检查输入。')
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteData(`/subjects/${id}`),
    onSuccess: () => {
      message.success('学科已删除。');
      queryClient.invalidateQueries({ queryKey: ['subjects'] });
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces'] });
      queryClient.invalidateQueries({ queryKey: ['subjects-for-schedule'] });
      queryClient.invalidateQueries({ queryKey: ['grade-subjects'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces-for-packages'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces-for-content'] });
      queryClient.invalidateQueries({ queryKey: ['learning-spaces-for-questions'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: (error: Error) => message.error(error.message || '删除学科失败，请稍后重试。')
  });

  function openEdit(subject: SubjectMetadata) {
    setEditing(subject);
    form.setFieldsValue({ shortLabel: subject.shortLabel, color: subject.color, sortOrder: subject.sortOrder, status: subject.status });
  }

  return (
    <Card title="学科管理" extra={<ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => subjects.refetch()} />}>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        这里维护全系统共用的学科基础信息。启用后可用于新增课程、课程方案和年级目录；停用后不能新增，但不会删除历史课程或学生已有权限。非内置残留学科在未被使用时可以删除，内置学科只能停用。
      </Typography.Paragraph>
      {subjects.isLoading ? <Skeleton active /> : subjects.error ? <Alert type="error" message="学科配置加载失败，请稍后重试。" /> : (
        <Table
          rowKey="id"
          pagination={false}
          dataSource={subjects.data ?? []}
          columns={[
            { title: '学科', dataIndex: 'name', render: (value: string) => subjectLabel(value) },
            { title: '简称', dataIndex: 'shortLabel' },
            { title: '显示颜色', dataIndex: 'color', render: (color: string) => <Space size={8}><span aria-label={`颜色 ${color}`} style={{ width: 18, height: 18, borderRadius: '50%', background: color, border: '1px solid #d9d9d9', display: 'inline-block' }} /><Typography.Text>{color}</Typography.Text></Space> },
            { title: '排序', dataIndex: 'sortOrder' },
            { title: '状态', dataIndex: 'status', render: (status: SubjectMetadata['status']) => <Tag color={status === '启用' ? 'green' : 'default'}>{status}</Tag> },
            { title: '操作', width: 108, render: (_: unknown, row: SubjectMetadata) => (
              <Space size={4}>
                <ActionButton tooltip="编辑学科" icon={<EditOutlined />} onClick={() => openEdit(row)} />
                <Popconfirm title={`确定删除「${subjectLabel(row.name)}」？`} description="删除后不可恢复。内置学科或仍被课程、方案使用的学科会删除失败。" okText="删除" cancelText="取消" okButtonProps={{ danger: true, loading: remove.isPending }} onConfirm={() => remove.mutate(row.id)}>
                  <ActionButton danger tooltip="删除学科" icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            ) }
          ]}
        />
      )}
      <FormDrawer
        title={editing ? `编辑${subjectLabel(editing.name)}` : '编辑学科'}
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

function WechatSettingsCard() {
  const [form] = Form.useForm<WechatSettingsUpdateRequest>();
  const queryClient = useQueryClient();
  const config = useQuery({ queryKey: ['wechat-settings'], queryFn: () => getData<WechatSettings>('/wechat/settings') });
  const save = useMutation({
    mutationFn: (values: WechatSettingsUpdateRequest) => putData<WechatSettings>('/wechat/settings', values),
    onSuccess: () => {
      message.success('微信配置已保存。');
      form.setFieldsValue({ miniProgramAppSecret: '', officialAccountAppSecret: '', callbackToken: '', encodingAesKey: '' });
      queryClient.invalidateQueries({ queryKey: ['wechat-settings'] });
    },
    onError: (error: Error) => message.error(error.message || '微信配置保存失败。')
  });

  if (config.isLoading) return <Skeleton active />;
  if (config.error || !config.data) return <Alert type="error" message="微信配置加载失败，请稍后重试。" />;

  const current = config.data;
  const callbackUrl = current.callbackUrl || resolveApiUrl('/wechat/official-account/callback');
  return (
    <Form
      form={form}
      layout="vertical"
      requiredMark={false}
      initialValues={{
        miniProgramName: current.miniProgramName,
        miniProgramAppId: current.miniProgramAppId,
        officialAccountName: current.officialAccountName,
        officialAccountAppId: current.officialAccountAppId,
        officialAccountOriginalId: current.officialAccountOriginalId
      }}
      onFinish={(values) => save.mutate(values)}
    >
      <div className="wechat-settings-grid">
        <Card title="小程序配置">
          <Form.Item name="miniProgramName" label="小程序名称" rules={[{ required: true, message: '请输入小程序名称' }]}>
            <Input placeholder="例如：星线教育" />
          </Form.Item>
          <Form.Item name="miniProgramAppId" label="AppID" rules={[{ required: true, message: '请输入小程序 AppID' }]}>
            <Input placeholder="wx..." autoComplete="off" />
          </Form.Item>
          <Form.Item name="miniProgramAppSecret" label="AppSecret">
            <Input.Password
              placeholder={current.miniProgramSecretConfigured ? '已保存，留空不修改' : '请输入小程序 AppSecret'}
              autoComplete="new-password"
            />
          </Form.Item>
        </Card>

        <Card title="公众号配置">
          <Form.Item name="officialAccountName" label="公众号名称" rules={[{ required: true, message: '请输入公众号名称' }]}>
            <Input placeholder="例如：星线教育" />
          </Form.Item>
          <div className="wechat-form-row">
            <Form.Item name="officialAccountAppId" label="AppID" rules={[{ required: true, message: '请输入公众号 AppID' }]}>
              <Input placeholder="wx..." autoComplete="off" />
            </Form.Item>
            <Form.Item name="officialAccountOriginalId" label="原始 ID" rules={[{ required: true, message: '请输入公众号原始 ID' }]}>
              <Input placeholder="gh_..." autoComplete="off" />
            </Form.Item>
          </div>
          <Form.Item name="officialAccountAppSecret" label="AppSecret">
            <Input.Password placeholder={current.officialAccountSecretConfigured ? '已保存，留空不修改' : '请输入公众号 AppSecret'} autoComplete="new-password" />
          </Form.Item>
          <div className="wechat-form-row">
            <Form.Item name="callbackToken" label="回调 Token">
              <Input.Password placeholder={current.callbackTokenConfigured ? '已保存，留空不修改' : '请输入回调 Token'} autoComplete="new-password" />
            </Form.Item>
            <Form.Item name="encodingAesKey" label="EncodingAESKey">
              <Input.Password placeholder={current.encodingAesKeyConfigured ? '已保存，留空不修改' : '请输入 EncodingAESKey'} autoComplete="new-password" />
            </Form.Item>
          </div>
          <Form.Item label="微信服务器回调地址">
            <Space.Compact block>
              <Input readOnly value={callbackUrl} />
              <Button
                icon={<CopyOutlined />}
                onClick={async () => {
                  await navigator.clipboard.writeText(callbackUrl);
                  message.success('回调地址已复制。');
                }}
              >复制</Button>
            </Space.Compact>
          </Form.Item>
        </Card>
      </div>
      <div className="wechat-settings-actions">
        <Button onClick={() => form.resetFields()}>取消</Button>
        <Button type="primary" htmlType="submit" loading={save.isPending}>保存配置</Button>
      </div>
    </Form>
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

  function openEdit(row: { key: string; value: string }) {
    setEditing(row);
    form.setFieldsValue(row);
  }

  const allEntries = Object.entries(settings.data ?? {}).filter(([key]) => key !== CALENDAR_KEY && key !== 'academicYear' && key !== 'academicPeriods');
  const toRows = (keys: string[]) => allEntries.filter(([key]) => keys.includes(key)).map(([key, value]) => ({ key, value })).sort((a, b) => keys.indexOf(a.key) - keys.indexOf(b.key));
  const contentRows = toRows(contentKeys);

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>系统设置</Typography.Title>
          <Typography.Text type="secondary">维护资料保护和微信接入配置。</Typography.Text>
        </div>
      </div>
      {settings.isLoading ? (
        <Skeleton active />
      ) : settings.error ? (
        <Alert type="error" message="系统设置加载失败，请稍后重试。" />
      ) : (
        <>
          <Tabs
            defaultActiveKey="protection"
            items={[
              {
                key: 'protection',
                label: <span><SafetyOutlined /> 资料保护</span>,
                children: <FlatSettingsCard title="资料保护" rows={contentRows} onEdit={openEdit} />
              },
              {
                key: 'integration',
                label: <span><LinkOutlined /> 小程序与公众号</span>,
                children: <WechatSettingsCard />
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
            <Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)}>
              <Form.Item name="key" hidden><Input /></Form.Item>
              <Form.Item name="value" label="当前值" rules={[{ required: true, message: '请输入设置值' }]}>
                <Input.TextArea rows={4} />
              </Form.Item>
            </Form>
          </FormDrawer>
        </>
      )}
    </div>
  );
}
