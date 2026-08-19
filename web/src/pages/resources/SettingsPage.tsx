import { Alert, Button, Card, Empty, Form, Input, Modal, Popconfirm, Skeleton, Space, Table, Typography, message } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import type { SettingUpdateRequest } from '../../types/starline';

const CALENDAR_KEY = 'academicCalendar';

const labels: Record<string, string> = {
  academicYear: '当前学年',
  grades: '适用年级',
  semesters: '学期设置',
  academicPeriods: '期中/期末时间段',
  watermarkRule: '水印规则',
  downloadPolicy: '下载规则',
  miniProgramDomainStatus: '小程序域名状态',
  productionApiDomain: '生产接口域名',
  officialAccountBindingStatus: '公众号绑定状态',
  templateMessageStatus: '模板消息状态'
};
const order = Object.keys(labels);

type AcademicTerm = {
  academicYear: string;
  semester: string;
  startDate: string;
  endDate: string;
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

function sortTerms(terms: AcademicTerm[]): AcademicTerm[] {
  return [...terms].sort((a, b) => (b.academicYear + a.startDate).localeCompare(a.academicYear + b.startDate));
}

export default function SettingsPage() {
  const [form] = Form.useForm<SettingUpdateRequest>();
  const [termForm] = Form.useForm<AcademicTerm & { originalIndex?: number }>();
  const [editing, setEditing] = useState<Record<string, string> | null>(null);
  const [termModalOpen, setTermModalOpen] = useState(false);
  // null = 新增；非 null = 正在编辑 terms（排序后的展示数组）里的第几条。
  const [editingTermIndex, setEditingTermIndex] = useState<number | null>(null);
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => getData<Record<string, string>>('/settings') });

  const save = useMutation({
    mutationFn: (values: SettingUpdateRequest) => putData<Record<string, string>>('/settings', values),
    onSuccess: () => {
      message.success('系统设置已保存。');
      setEditing(null);
      closeTermModal();
      form.resetFields();
      termForm.resetFields();
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: () => message.error('保存设置失败，请检查设置值。')
  });

  const terms = sortTerms(parseCalendar(settings.data?.[CALENDAR_KEY]));

  function saveTerms(nextTerms: AcademicTerm[]) {
    save.mutate({ key: CALENDAR_KEY, value: JSON.stringify(nextTerms) });
  }

  function openTermModal(index: number | null) {
    setEditingTermIndex(index);
    setTermModalOpen(true);
    if (index === null) {
      termForm.resetFields();
    } else {
      termForm.setFieldsValue(terms[index]);
    }
  }

  function closeTermModal() {
    setTermModalOpen(false);
    setEditingTermIndex(null);
  }

  function submitTerm(values: AcademicTerm) {
    const base = parseCalendar(settings.data?.[CALENDAR_KEY]);
    if (editingTermIndex === null) {
      saveTerms([...base, values]);
      return;
    }
    // editingTermIndex 是排序后表格里的下标，落回未排序的原始数组前先按同样的排序取出目标项，
    // 避免排序后下标和原始数组下标对不上、改错行。
    const target = terms[editingTermIndex];
    const originalIndex = base.findIndex((item) => item === target || (item.academicYear === target.academicYear && item.semester === target.semester && item.startDate === target.startDate && item.endDate === target.endDate));
    const next = [...base];
    if (originalIndex >= 0) {
      next[originalIndex] = values;
    } else {
      next.push(values);
    }
    saveTerms(next);
  }

  function removeTerm(index: number) {
    const target = terms[index];
    const base = parseCalendar(settings.data?.[CALENDAR_KEY]);
    const next = base.filter((item) => !(item.academicYear === target.academicYear && item.semester === target.semester && item.startDate === target.startDate && item.endDate === target.endDate));
    saveTerms(next);
  }

  const rows = Object.entries(settings.data ?? {})
    .filter(([key]) => key !== CALENDAR_KEY)
    .map(([key, value]) => ({ key, value }))
    .sort((a, b) => order.indexOf(a.key) - order.indexOf(b.key));

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
          <Card
            title="校历"
            extra={<Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => openTermModal(null)}>新增学期</Button>}
          >
            <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
              按学年、按学期维护起止日期，可以提前配好下一学年的校历。新开通的学习套餐默认在当前学年最晚一个学期的结束日到期，开通当天即生效；已开通的记录不受影响，单个学生的有效期仍可在开通时单独调整。
            </Typography.Paragraph>
            {terms.length === 0 ? (
              <Empty description="还没有配置校历，点击右上角新增学期。" />
            ) : (
              <Table
                rowKey={(row, index) => `${row.academicYear}-${row.semester}-${index}`}
                pagination={false}
                dataSource={terms}
                columns={[
                  { title: '学年', dataIndex: 'academicYear' },
                  { title: '学期', dataIndex: 'semester' },
                  { title: '开始日期', dataIndex: 'startDate' },
                  { title: '结束日期', dataIndex: 'endDate' },
                  {
                    title: '操作',
                    render: (_, __, index) => (
                      <Space>
                        <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openTermModal(index)} />
                        <Popconfirm title="删除这个学期？" okText="删除" cancelText="取消" onConfirm={() => removeTerm(index)}>
                          <ActionButton tooltip="删除" icon={<DeleteOutlined />} />
                        </Popconfirm>
                      </Space>
                    )
                  }
                ]}
              />
            )}
          </Card>

          <Card extra={<ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => settings.refetch()} />}>
            {rows.length === 0 ? (
              <Empty description="还没有系统设置。" />
            ) : (
              <Table
                rowKey="key"
                pagination={false}
                dataSource={rows}
                columns={[
                  { title: '设置项', dataIndex: 'key', render: (key) => labels[key] ?? key },
                  { title: '当前值', dataIndex: 'value' },
                  {
                    title: '操作',
                    render: (_, row) => (
                      <ActionButton
                        tooltip="编辑"
                        icon={<EditOutlined />}
                        onClick={() => {
                          setEditing(row);
                          form.setFieldsValue(row);
                        }}
                      />
                    )
                  }
                ]}
              />
            )}
          </Card>

          <Modal
            title={`编辑${editing ? labels[editing.key] ?? editing.key : '设置'}`}
            open={Boolean(editing)}
            onCancel={() => setEditing(null)}
            footer={
              <Space>
                <Button onClick={() => setEditing(null)}>取消</Button>
                <Button type="primary" loading={save.isPending} onClick={() => form.submit()}>保存</Button>
              </Space>
            }
          >
            <Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)}>
              <Form.Item name="key" hidden><Input /></Form.Item>
              <Form.Item name="value" label="当前值" rules={[{ required: true, message: '请输入设置值' }]}>
                <Input.TextArea rows={4} />
              </Form.Item>
            </Form>
          </Modal>

          <Modal
            title={editingTermIndex === null ? '新增学期' : '编辑学期'}
            open={termModalOpen}
            onCancel={closeTermModal}
            footer={
              <Space>
                <Button onClick={closeTermModal}>取消</Button>
                <Button type="primary" loading={save.isPending} onClick={() => termForm.submit()}>保存</Button>
              </Space>
            }
            destroyOnHidden
          >
            <Form form={termForm} layout="vertical" onFinish={submitTerm}>
              <Form.Item name="academicYear" label="学年" rules={[{ required: true, message: '请输入学年，例如 2026.2027学年' }]}>
                <Input placeholder="例如：2026.2027学年" />
              </Form.Item>
              <Form.Item name="semester" label="学期" rules={[{ required: true, message: '请输入学期，例如 S1 第一学期' }]}>
                <Input placeholder="例如：S1 第一学期" />
              </Form.Item>
              <Form.Item name="startDate" label="开始日期" rules={[{ required: true, pattern: /^\d{4}-\d{2}-\d{2}$/, message: '格式为 YYYY-MM-DD' }]}>
                <Input placeholder="YYYY-MM-DD" />
              </Form.Item>
              <Form.Item name="endDate" label="结束日期" rules={[{ required: true, pattern: /^\d{4}-\d{2}-\d{2}$/, message: '格式为 YYYY-MM-DD' }]}>
                <Input placeholder="YYYY-MM-DD" />
              </Form.Item>
            </Form>
          </Modal>
        </>
      )}
    </div>
  );
}
