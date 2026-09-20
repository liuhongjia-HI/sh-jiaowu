import { BookOutlined, CalendarOutlined, EditOutlined, TagsOutlined } from '@ant-design/icons';
import { Alert, Card, Form, Input, Skeleton, Space, Tabs, Typography, message } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { FormDrawer } from '../../components/FormDrawer';
import { ActionButton } from '../../components/ListViews';
import { getData, putData } from '../../services/http';
import type { SettingUpdateRequest } from '../../types/starline';
import GradeSubjects from '../GradeSubjects';
import { AcademicCalendarCard, SubjectMetadataCard } from './SettingsPage';

const tabs = ['catalog', 'subjects', 'calendar'] as const;
type TeachingSettingsTab = typeof tabs[number];

function SemesterSettingsCard({ value, onSave, saving }: { value?: string; onSave: (value: string) => void; saving: boolean }) {
  const [form] = Form.useForm<{ value: string }>();
  const [open, setOpen] = useState(false);

  return <Card
    title="学期设置"
    extra={<ActionButton tooltip="编辑学期设置" icon={<EditOutlined />} onClick={() => { form.setFieldsValue({ value: value ?? '' }); setOpen(true); }} />}
  >
    <Typography.Paragraph type="secondary">课程、题库和课程方案使用的学期选项。学年起止与期中日期在下方校历中维护。</Typography.Paragraph>
    <Space><Typography.Text type="secondary">当前学期：</Typography.Text><Typography.Text>{value || 'S1 / S2'}</Typography.Text></Space>
    <FormDrawer title="编辑学期设置" open={open} onCancel={() => setOpen(false)} onSubmit={() => form.submit()} submitting={saving}>
      <Form form={form} layout="vertical" onFinish={({ value: next }) => { onSave(next); setOpen(false); }}>
        <Form.Item name="value" label="学期" extra="多个学期使用 / 分隔，例如 S1 / S2。" rules={[{ required: true, message: '请输入学期设置' }]}>
          <Input placeholder="S1 / S2" />
        </Form.Item>
      </Form>
    </FormDrawer>
  </Card>;
}

export default function TeachingSettingsPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab');
  const activeTab: TeachingSettingsTab = tabs.includes(requestedTab as TeachingSettingsTab) ? requestedTab as TeachingSettingsTab : 'catalog';
  const client = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => getData<Record<string, string>>('/settings') });
  const save = useMutation({
    mutationFn: (values: SettingUpdateRequest) => putData<Record<string, string>>('/settings', values),
    onSuccess: () => {
      message.success('教学配置已保存。');
      client.invalidateQueries({ queryKey: ['settings'] });
      client.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: (error: Error) => message.error(error.message || '保存教学配置失败，请检查设置值。')
  });

  function selectTab(tab: string) {
    const next = new URLSearchParams(searchParams);
    next.set('tab', tab);
    if (tab !== 'catalog') next.delete('grade');
    setSearchParams(next, { replace: true });
  }

  return <div className="page-stack">
    <div className="page-heading">
      <div>
        <Typography.Title level={3}>教学配置</Typography.Title>
        <Typography.Text type="secondary">统一维护年级课程目录、学科基础信息和学年校历。</Typography.Text>
      </div>
    </div>
    {settings.isLoading ? <Skeleton active /> : settings.error ? <Alert type="error" message="教学配置加载失败，请稍后重试。" /> : (
      <Tabs activeKey={activeTab} onChange={selectTab} items={[
        { key: 'catalog', label: <span><BookOutlined /> 年级课程目录</span>, children: <GradeSubjects /> },
        { key: 'subjects', label: <span><TagsOutlined /> 学科管理</span>, children: <SubjectMetadataCard /> },
        { key: 'calendar', label: <span><CalendarOutlined /> 学年校历</span>, children: <Space direction="vertical" size={16} style={{ width: '100%' }}>
          <SemesterSettingsCard value={settings.data?.semesters} saving={save.isPending} onSave={(value) => save.mutate({ key: 'semesters', value })} />
          <AcademicCalendarCard rawValue={settings.data?.academicCalendar} saving={save.isPending} onSave={(value) => save.mutate({ key: 'academicCalendar', value })} />
        </Space> }
      ]} />
    )}
  </div>;
}
