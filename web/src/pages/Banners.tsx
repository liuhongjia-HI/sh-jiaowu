import { DeleteOutlined, EditOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Empty, Form, Input, InputNumber, Popconfirm, Radio, Skeleton, Space, Switch, Table, Tag, Typography, Upload, message } from 'antd';
import type { UploadFile } from 'antd';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteData, getData, postData, postForm, putData, resolveAssetUrl } from '../services/http';
import { FormDrawer } from '../components/FormDrawer';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../components/ListViews';
import type { Banner, BannerLinkType, BannerUpsertRequest } from '../types/starline';

type BannerFormValues = {
  title: string;
  linkType: BannerLinkType;
  linkValue: string;
  sortOrder: number;
  startsAt?: string;
  endsAt?: string;
  enabled: boolean;
  fileList: UploadFile[];
};

const statusColors: Record<string, string> = {
  生效中: 'green',
  未开始: 'blue',
  已结束: 'default',
  已停用: 'default'
};

const MAX_BANNER_IMAGE_SIZE = 5 * 1024 * 1024;

export default function Banners() {
  const [form] = Form.useForm<BannerFormValues>();
  const [editing, setEditing] = useState<Banner | null>(null);
  const [open, setOpen] = useState(false);
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:banners');
  const linkType = Form.useWatch('linkType', form);
  const queryClient = useQueryClient();

  const banners = useQuery({ queryKey: ['banners'], queryFn: () => getData<Banner[]>('/banners') });

  // 图片先单独上传拿地址，再连同其它字段一起提交：调整排序、生效时间或下线
  // 不需要重新选图，编辑弹窗打开时也不用把图片重新灌回 Upload 组件。
  const uploadImage = useMutation({
    mutationFn: (file: File) => {
      const data = new FormData();
      data.append('file', file);
      return postForm<{ imageUrl: string }>('/banners/upload', data);
    },
    onError: (error: any) => message.error(error.response?.data?.message || '图片上传失败，请重新选择。')
  });

  const saveBanner = useMutation({
    mutationFn: async (values: BannerFormValues) => {
      let imageUrl = editing?.imageUrl ?? '';
      const file = values.fileList?.[0]?.originFileObj as File | undefined;
      if (file) {
        imageUrl = (await uploadImage.mutateAsync(file)).imageUrl;
      }
      if (!imageUrl) throw new Error('请上传轮播图图片');
      const body: BannerUpsertRequest = {
        imageUrl,
        title: values.title ?? '',
        linkType: values.linkType,
        linkValue: values.linkType === 'none' ? '' : (values.linkValue ?? ''),
        sortOrder: values.sortOrder ?? 0,
        startsAt: values.startsAt ?? '',
        endsAt: values.endsAt ?? '',
        enabled: Boolean(values.enabled)
      };
      if (editing) return putData<Banner>(`/banners/${editing.id}`, body);
      return postData<Banner>('/banners', body);
    },
    onSuccess: () => {
      message.success(editing ? '轮播图已保存' : '轮播图已新增');
      setOpen(false);
      setEditing(null);
      queryClient.invalidateQueries({ queryKey: ['banners'] });
    },
    onError: (error: any) => message.error(error.message || error.response?.data?.message || '保存失败，请检查图片和跳转设置。')
  });

  const removeBanner = useMutation({
    mutationFn: (id: string) => deleteData(`/banners/${id}`),
    onSuccess: () => {
      message.success('轮播图已删除');
      queryClient.invalidateQueries({ queryKey: ['banners'] });
    },
    onError: (error: any) => message.error(error.response?.data?.message || '删除失败，请稍后重试。')
  });

  function openCreate() {
    setEditing(null);
    form.setFieldsValue({ title: '', linkType: 'none', linkValue: '', sortOrder: 0, startsAt: '', endsAt: '', enabled: true, fileList: [] });
    setOpen(true);
  }

  function openEdit(banner: Banner) {
    setEditing(banner);
    form.setFieldsValue({
      title: banner.title,
      linkType: banner.linkType,
      linkValue: banner.linkValue,
      sortOrder: banner.sortOrder,
      startsAt: banner.startsAt ?? '',
      endsAt: banner.endsAt ?? '',
      enabled: banner.enabled,
      fileList: []
    });
    setOpen(true);
  }

  if (banners.isLoading) return <Skeleton active />;
  if (banners.error) return <Alert type="error" message="轮播图加载失败，请稍后重试。" />;

  const rows = banners.data ?? [];

  return (
    <div className="page-stack">
      <div className="page-heading">
        <div>
          <Typography.Title level={3}>轮播图管理</Typography.Title>
          <Typography.Text type="secondary">维护学生端小程序首页的轮播图、跳转目标和生效时间段。</Typography.Text>
        </div>
        <div className="page-heading-actions">
          <ListViewToggle storageKey="starline:list-view:banners" value={viewMode} onChange={setViewMode} />
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增轮播图</Button>
        </div>
      </div>

      <Card>
        {viewMode === 'card' ? (
          <CardList
            rows={rows}
            rowKey={(record) => record.id}
            emptyText="还没有轮播图，新增一张展示到学生端首页。"
            renderCard={(record) => (
              <InfoCard
                title={record.title || '（未命名）'}
                subtitle={bannerLinkText(record)}
                status={<Tag color={statusColors[record.status] ?? 'default'}>{record.status}</Tag>}
                fields={[
                  { label: '预览', value: <img src={resolveAssetUrl(record.imageUrl)} alt={record.title} className="banner-thumb" />, fullWidth: true },
                  { label: '排序', value: record.sortOrder },
                  { label: '生效时间段', value: bannerRangeText(record) }
                ]}
                actions={(
                  <>
                    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />
                    <Popconfirm title="删除这张轮播图？" description="删除后学生端首页立即不再展示，不可恢复。" okText="删除" cancelText="取消" onConfirm={() => removeBanner.mutate(record.id)}>
                      <ActionButton tooltip="删除" icon={<DeleteOutlined />} loading={removeBanner.isPending} />
                    </Popconfirm>
                  </>
                )}
              />
            )}
          />
        ) : rows.length === 0 ? (
          <Empty description="还没有轮播图，新增一张展示到学生端首页。" />
        ) : (
          <Table
            rowKey="id"
            dataSource={rows}
            pagination={false}
            columns={[
              { title: '预览', width: 140, render: (_, record) => <img src={resolveAssetUrl(record.imageUrl)} alt={record.title} className="banner-thumb" /> },
              { title: '标题', dataIndex: 'title', render: (value: string) => value || <Typography.Text type="secondary">-</Typography.Text> },
              { title: '跳转', render: (_, record) => bannerLinkText(record) },
              { title: '生效时间段', render: (_, record) => bannerRangeText(record) },
              { title: '排序', dataIndex: 'sortOrder', width: 80 },
              { title: '状态', dataIndex: 'status', width: 100, render: (value: string) => <Tag color={statusColors[value] ?? 'default'}>{value}</Tag> },
              {
                title: '操作',
                width: 96,
                render: (_, record) => (
                  <Space size={4}>
                    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(record)} />
                    <Popconfirm title="删除这张轮播图？" description="删除后学生端首页立即不再展示，不可恢复。" okText="删除" cancelText="取消" onConfirm={() => removeBanner.mutate(record.id)}>
                      <ActionButton tooltip="删除" icon={<DeleteOutlined />} loading={removeBanner.isPending} />
                    </Popconfirm>
                  </Space>
                )
              }
            ]}
          />
        )}
      </Card>

      <FormDrawer
        title={editing ? '编辑轮播图' : '新增轮播图'}
        open={open}
        onCancel={() => setOpen(false)}
        onSubmit={() => form.submit()}
        submitting={saveBanner.isPending || uploadImage.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveBanner.mutate(values)}>
          <Form.Item
            name="fileList"
            label="轮播图图片"
            valuePropName="fileList"
            getValueFromEvent={(event) => event?.fileList ?? []}
            rules={[{
              validator: (_, value) => (value?.length > 0 || editing?.imageUrl) ? Promise.resolve() : Promise.reject(new Error('请上传轮播图图片'))
            }]}
            extra="建议尺寸 750×350，JPG 或 PNG，5MB 以内。"
          >
            <Upload
              beforeUpload={(file) => {
                if (file.size > MAX_BANNER_IMAGE_SIZE) {
                  message.error('轮播图不能超过 5MB，请压缩图片后重试。');
                  return Upload.LIST_IGNORE;
                }
                return false;
              }}
              maxCount={1}
              accept=".jpg,.jpeg,.png"
              listType="picture-card"
            >
              {editing?.imageUrl ? (
                <img src={resolveAssetUrl(editing.imageUrl)} alt="当前轮播图" className="banner-upload-preview" />
              ) : (
                <div><UploadOutlined /><div style={{ marginTop: 8 }}>选择图片</div></div>
              )}
            </Upload>
          </Form.Item>
          <Form.Item name="title" label="标题">
            <Input placeholder="仅用于后台管理识别，学生端不一定展示" />
          </Form.Item>
          <Form.Item name="linkType" label="点击跳转" rules={[{ required: true }]}>
            <Radio.Group optionType="button" buttonStyle="solid">
              <Radio.Button value="none">不跳转</Radio.Button>
              <Radio.Button value="page">小程序内页</Radio.Button>
              <Radio.Button value="url">外部链接</Radio.Button>
            </Radio.Group>
          </Form.Item>
          {linkType && linkType !== 'none' && (
            <Form.Item
              name="linkValue"
              label={linkType === 'page' ? '页面路径' : '外部链接地址'}
              rules={[{ required: true, message: linkType === 'page' ? '请填写页面路径' : '请填写链接地址' }]}
            >
              <Input placeholder={linkType === 'page' ? '例如：/pages/study/index' : '例如：https://example.com'} />
            </Form.Item>
          )}
          <Space.Compact block>
            <Form.Item name="startsAt" label="生效开始" style={{ width: '50%' }} extra="不填表示立即生效。">
              <DateField />
            </Form.Item>
            <Form.Item name="endsAt" label="生效结束" style={{ width: '50%' }} extra="不填表示长期有效。">
              <DateField />
            </Form.Item>
          </Space.Compact>
          <Form.Item name="sortOrder" label="排序" extra="数字越小越靠前。" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </FormDrawer>
    </div>
  );
}

function DateField(props: { value?: string; onChange?: (value: string) => void }) {
  return <Input type="date" value={props.value ?? ''} onChange={(event) => props.onChange?.(event.target.value)} />;
}

function bannerLinkText(record: Banner) {
  if (record.linkType === 'none' || !record.linkValue) return <Typography.Text type="secondary">不跳转</Typography.Text>;
  return `${record.linkType === 'page' ? '页面' : '链接'}：${record.linkValue}`;
}

function bannerRangeText(record: Banner) {
  if (!record.startsAt && !record.endsAt) return <Typography.Text type="secondary">长期有效</Typography.Text>;
  return `${record.startsAt || '不限'} ~ ${record.endsAt || '不限'}`;
}
