import { Alert, Button, Card, Empty, Form, Input, Modal, Popconfirm, Skeleton, Space, Table, Tabs, Typography, message } from 'antd';
import { CalendarOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined, ReloadOutlined, SafetyOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import type { SettingUpdateRequest } from '../../types/starline';

const CALENDAR_KEY = 'academicCalendar';
const FALL_LABEL = 'S1 第一学期';
const SPRING_LABEL = 'S2 第二学期';

// 三个 Tab 分组：系统设置项一多，摊平成一张大表格谁都懒得找。分组按“运营会想在什么场景下打开这一项”来划，
// 不按数据类型分——校历天天要看，接入状态一年调一次，放在一起只会互相淹没。
const contentKeys = ['academicYear', 'grades', 'semesters', 'academicPeriods', 'watermarkRule', 'downloadPolicy'];
const integrationKeys = ['miniProgramDomainStatus', 'officialAccountBindingStatus', 'templateMessageStatus', 'miniProgramSubscribeStatus', 'productionApiDomain'];

const labels: Record<string, string> = {
  academicYear: '当前学年',
  grades: '适用年级',
  semesters: '学期设置',
  academicPeriods: '期中/期末时间段',
  watermarkRule: '水印规则',
  downloadPolicy: '下载规则',
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
        每学年一行，填秋季、春季学期的起止日期即可，可以提前配好下一学年。新开通的学习套餐默认在当前学年最后一天到期，开通当天即生效；已开通的记录不受影响，单个学生的有效期仍可在开通时单独调整。
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

      <Modal
        title={originalYear ? `编辑${originalYear}` : '新增学年'}
        open={modalOpen}
        onCancel={close}
        footer={<Space><Button onClick={close}>取消</Button><Button type="primary" loading={saving} onClick={() => form.submit()}>保存</Button></Space>}
        destroyOnHidden
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
      </Modal>
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
    form.setFieldsValue(row);
  }

  const allEntries = Object.entries(settings.data ?? {}).filter(([key]) => key !== CALENDAR_KEY);
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
                children: <FlatSettingsCard rows={[...contentRows, ...otherRows]} onEdit={openEdit} />
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

          <Modal
            title={`编辑${editing ? labels[editing.key] ?? editing.key : '设置'}`}
            open={Boolean(editing)}
            onCancel={() => setEditing(null)}
            footer={<Space><Button onClick={() => setEditing(null)}>取消</Button><Button type="primary" loading={save.isPending} onClick={() => form.submit()}>保存</Button></Space>}
          >
            <Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)}>
              <Form.Item name="key" hidden><Input /></Form.Item>
              <Form.Item name="value" label="当前值" rules={[{ required: true, message: '请输入设置值' }]}>
                <Input.TextArea rows={4} />
              </Form.Item>
            </Form>
          </Modal>
        </>
      )}
    </div>
  );
}
