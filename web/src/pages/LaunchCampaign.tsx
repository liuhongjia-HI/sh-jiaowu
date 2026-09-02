import { EditOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, DatePicker, Descriptions, Empty, Form, Image, Input, InputNumber, Select, Skeleton, Space, Switch, Tag, Typography, Upload, message } from 'antd';
import type { UploadFile } from 'antd';
import dayjs from 'dayjs';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { FormDrawer } from '../components/FormDrawer';
import { getData, postForm, putData, resolveAssetUrl } from '../services/http';
import type { SettingUpdateRequest } from '../types/starline';

type LaunchCampaign = {
  enabled: boolean;
  templateType: 'generic' | 'small_class_reservation';
  title: string;
  message: string;
  subMessage: string;
  imageUrl: string;
  primaryActionText: string;
  timeOptions: string[];
  frequency: 'once' | 'daily' | 'every_entry';
  priority: number;
  startsAt: string;
  endsAt: string;
};

type CampaignFormValues = Omit<LaunchCampaign, 'timeOptions'> & { timeOptionsText: string; fileList: UploadFile[] };

const defaultCampaign: LaunchCampaign = {
  enabled: false,
  templateType: 'generic',
  title: '',
  message: '',
  subMessage: '',
  imageUrl: '',
  primaryActionText: '立即了解',
  timeOptions: [],
  frequency: 'once',
  priority: 0,
  startsAt: '',
  endsAt: ''
};

const MAX_CAMPAIGN_IMAGE_SIZE = 5 * 1024 * 1024;

function parseCampaign(raw?: string): LaunchCampaign {
  try {
    const value = JSON.parse(raw || '{}') as Partial<LaunchCampaign>;
    return {
      ...defaultCampaign,
      ...value,
      enabled: Boolean(value.enabled),
      templateType: value.templateType === 'small_class_reservation' ? 'small_class_reservation' : 'generic',
      timeOptions: Array.isArray(value.timeOptions) ? value.timeOptions.filter((item): item is string => typeof item === 'string') : [],
      priority: Number.isFinite(Number(value.priority)) ? Number(value.priority) : 0
    };
  } catch {
    return defaultCampaign;
  }
}

function campaignFormValues(campaign: LaunchCampaign): CampaignFormValues {
  return { ...campaign, timeOptionsText: campaign.timeOptions.join('\n'), fileList: [] };
}

function hasCampaignContent(campaign: LaunchCampaign) {
  return Boolean(campaign.title || campaign.message || campaign.startsAt || campaign.endsAt);
}

function frequencyLabel(value: LaunchCampaign['frequency']) {
  return ({ once: '活动期间一次', daily: '每天一次', every_entry: '每次进入首页' })[value];
}

function templateLabel(value: LaunchCampaign['templateType']) {
  return value === 'small_class_reservation' ? '小班课预约' : '通用图文';
}

function rangeLabel(campaign: LaunchCampaign) {
  if (!campaign.startsAt && !campaign.endsAt) return '长期有效';
  return `${campaign.startsAt || '立即开始'} 至 ${campaign.endsAt || '长期投放'}`;
}

export default function LaunchCampaignPage() {
  const [form] = Form.useForm<CampaignFormValues>();
  const [editing, setEditing] = useState(false);
  const queryClient = useQueryClient();
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => getData<Record<string, string>>('/settings') });
  const campaign = parseCampaign(settings.data?.launchCampaign);
  const hasContent = hasCampaignContent(campaign);

  const uploadImage = useMutation({
    mutationFn: (file: File) => {
      const data = new FormData();
      data.append('file', file);
      return postForm<{ imageUrl: string }>('/launch-campaign/upload', data);
    },
    onError: (error: Error) => message.error(error.message || '图片上传失败，请重新选择。')
  });

  const save = useMutation({
    mutationFn: async (values: CampaignFormValues) => {
      let imageUrl = values.imageUrl || '';
      const file = values.fileList?.[0]?.originFileObj as File | undefined;
      if (file) imageUrl = (await uploadImage.mutateAsync(file)).imageUrl;
      const { fileList: _fileList, timeOptionsText, ...campaignValues } = values;
      return putData<Record<string, string>>('/settings', {
        key: 'launchCampaign',
        value: JSON.stringify({ ...campaignValues, imageUrl, timeOptions: timeOptionsText.split('\n').map((item) => item.trim()).filter(Boolean) })
      } satisfies SettingUpdateRequest);
    },
    onSuccess: (_, values) => {
      message.success(values.enabled ? '开屏活动已发布。' : '开屏活动已保存，当前未投放。');
      setEditing(false);
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: (error: Error) => message.error(error.message || '保存开屏活动失败，请稍后重试。')
  });

  function openEditor() {
    form.setFieldsValue(campaignFormValues(campaign));
    setEditing(true);
  }

  function submit(values: CampaignFormValues) {
    if (values.startsAt && values.endsAt && values.startsAt >= values.endsAt) {
      message.error('结束时间必须晚于开始时间。');
      return;
    }
    save.mutate(values);
  }

  if (settings.isLoading) return <Skeleton active />;
  if (settings.error) return <Alert type="error" message="开屏活动加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>开屏活动</Typography.Title>
          <Typography.Text type="secondary">配置学生进入小程序首页时看到的活动内容和投放时间。</Typography.Text>
        </div>
        {hasContent && <Button type="primary" icon={<EditOutlined />} onClick={openEditor}>编辑活动</Button>}
      </div>

      {!hasContent ? (
        <Card>
          <Empty description="还没有开屏活动，创建后可按时间投放到学生端首页。">
            <Button type="primary" icon={<PlusOutlined />} onClick={openEditor}>新建开屏活动</Button>
          </Empty>
        </Card>
      ) : (
        <Card
          title="当前开屏活动"
          extra={<Space><Tag color={campaign.enabled ? 'green' : 'default'}>{campaign.enabled ? '投放中' : '未投放'}</Tag><Switch checked={campaign.enabled} checkedChildren="投放中" unCheckedChildren="未投放" aria-label="开屏活动投放状态" loading={save.isPending} onChange={(enabled) => save.mutate(campaignFormValues({ ...campaign, enabled }))} /></Space>}
        >
          <Descriptions column={{ xs: 1, sm: 2 }} size="small">
            <Descriptions.Item label="活动名称">{campaign.title || '未命名活动'}</Descriptions.Item>
            <Descriptions.Item label="活动类型">{templateLabel(campaign.templateType)}</Descriptions.Item>
            <Descriptions.Item label="投放时间" span={2}>{rangeLabel(campaign)}</Descriptions.Item>
            <Descriptions.Item label="展示频次">{frequencyLabel(campaign.frequency)}</Descriptions.Item>
            <Descriptions.Item label="优先级">{campaign.priority}</Descriptions.Item>
            <Descriptions.Item label="活动说明" span={2}>{campaign.message || '—'}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      <FormDrawer title={hasContent ? '编辑开屏活动' : '新建开屏活动'} open={editing} onCancel={() => setEditing(false)} onSubmit={() => form.submit()} submitting={save.isPending || uploadImage.isPending} submitText="保存活动">
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item name="enabled" label="保存后立即投放" valuePropName="checked"><Switch checkedChildren="立即投放" unCheckedChildren="暂不投放" /></Form.Item>
          <Form.Item name="templateType" label="活动类型" rules={[{ required: true, message: '请选择活动类型' }]}><Select options={[{ value: 'generic', label: '通用图文' }, { value: 'small_class_reservation', label: '小班课预约' }]} /></Form.Item>
          <Form.Item name="title" label="活动名称" rules={[{ required: true, whitespace: true, message: '请输入活动名称' }]}><Input maxLength={40} showCount placeholder="例如：秋季小班课预约" /></Form.Item>
          <Form.Item name="message" label="活动说明"><Input.TextArea rows={3} maxLength={120} showCount placeholder="简要说明活动内容和用户可获得的权益" /></Form.Item>
          <Form.Item name="subMessage" label="补充说明"><Input.TextArea rows={2} maxLength={80} showCount /></Form.Item>
          <Form.Item name="imageUrl" noStyle><Input type="hidden" /></Form.Item>
          <Form.Item
            name="fileList"
            label="活动图片"
            valuePropName="fileList"
            getValueFromEvent={(event) => event?.fileList ?? []}
            extra="可选，JPG 或 PNG，5MB 以内。"
          >
            <Upload
              beforeUpload={(file) => {
                if (file.size > MAX_CAMPAIGN_IMAGE_SIZE) {
                  message.error('活动图片不能超过 5MB，请压缩图片后重试。');
                  return Upload.LIST_IGNORE;
                }
                return false;
              }}
              maxCount={1}
              accept=".jpg,.jpeg,.png"
              listType="picture-card"
            >
              {form.getFieldValue('imageUrl') ? (
                <Image preview={false} src={resolveAssetUrl(form.getFieldValue('imageUrl'))} alt="当前活动图片" width={96} height={96} style={{ objectFit: 'cover' }} />
              ) : (
                <div><UploadOutlined /><div style={{ marginTop: 8 }}>选择图片</div></div>
              )}
            </Upload>
          </Form.Item>
          <Form.Item name="primaryActionText" label="按钮文案" rules={[{ required: true, whitespace: true, message: '请输入按钮文案' }]}><Input maxLength={12} /></Form.Item>
          <Form.Item name="timeOptionsText" label="可选上课时间"><Input.TextArea rows={3} placeholder="每行一个，例如：\n工作日晚上\n周六上午" /></Form.Item>
          <Space align="start" wrap>
            <Form.Item name="frequency" label="展示频次" rules={[{ required: true }]}><Select style={{ width: 160 }} options={[{ value: 'once', label: '活动期间一次' }, { value: 'daily', label: '每天一次' }, { value: 'every_entry', label: '每次进入首页' }]} /></Form.Item>
            <Form.Item name="priority" label="优先级"><InputNumber min={0} max={99} /></Form.Item>
          </Space>
          <Space align="start" wrap>
            <Form.Item name="startsAt" label="开始时间" extra="不填表示立即开始"><DateTimeField /></Form.Item>
            <Form.Item name="endsAt" label="结束时间" extra="不填表示长期投放"><DateTimeField /></Form.Item>
          </Space>
        </Form>
      </FormDrawer>
    </div>
  );
}

function DateTimeField(props: { value?: string; onChange?: (value: string) => void }) {
  const value = props.value ? dayjs(props.value) : null;
  return <DatePicker showTime format="YYYY-MM-DD HH:mm" value={value?.isValid() ? value : null} onChange={(date) => props.onChange?.(date ? date.format('YYYY-MM-DD HH:mm:ss') : '')} style={{ width: 220 }} placeholder="请选择日期和时间" />;
}
