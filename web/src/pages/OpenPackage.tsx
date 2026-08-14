import { Alert, Button, Card, Col, Form, message, Row, Select, Skeleton, Space, Tag, Typography } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo } from 'react';
import { getData, postData } from '../services/http';
import type { GrantCreateRequest, GrantPreview, Student, StudyPackage } from '../types/starline';

type GrantFormValues = {
  studentId: string;
  packageId: string;
  startsAt?: string;
  endsAt?: string;
};

export default function OpenPackage() {
  const [form] = Form.useForm<GrantFormValues>();
  const queryClient = useQueryClient();
  const students = useQuery({ queryKey: ['students'], queryFn: () => getData<Student[]>('/students') });
  const packages = useQuery({ queryKey: ['packages'], queryFn: () => getData<StudyPackage[]>('/packages') });
  const studentId = Form.useWatch('studentId', form);
  const packageId = Form.useWatch('packageId', form);
  const startsAt = Form.useWatch('startsAt', form);
  const selectedStudent = useMemo(() => (students.data ?? []).find((item) => item.id === studentId), [students.data, studentId]);
  const availablePackages = useMemo(
    () => (packages.data ?? []).filter((item) => item.status === '启用' && item.grade === selectedStudent?.grade),
    [packages.data, selectedStudent?.grade]
  );
  const preview = useQuery({
    queryKey: ['grant-preview', studentId, packageId],
    enabled: Boolean(studentId && packageId),
    queryFn: () => getData<GrantPreview>('/grants/preview', { studentId, packageId })
  });
  useEffect(() => {
    if (!preview.data || preview.data.alreadyOpened) return;
    if (!form.getFieldValue('startsAt')) form.setFieldValue('startsAt', preview.data.startsAtDefault);
    if (!form.getFieldValue('endsAt')) form.setFieldValue('endsAt', preview.data.endsAtDefault);
  }, [form, preview.data]);
  const createGrant = useMutation({
    mutationFn: (values: GrantFormValues) => postData<GrantPreview>('/grants', grantBody(values)),
    onSuccess: (result) => {
      message.success(result.alreadyOpened ? '该学生已开通过这个套餐，学习权限保持有效。' : '学习套餐已开通，学习权限已同步。');
      queryClient.invalidateQueries({ queryKey: ['students'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
      queryClient.invalidateQueries({ queryKey: ['grant-preview', studentId, packageId] });
    },
    onError: (err: Error) => message.error(err.message || '开通失败，请检查学生和学习套餐。')
  });

  if (students.isLoading || packages.isLoading) return <Skeleton active />;
  if (students.error || packages.error) return <Alert type="error" message="开通页面加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div>
        <Typography.Title level={3}>开通套餐</Typography.Title>
        <Typography.Text type="secondary">确认课程、资料和练习后开通。</Typography.Text>
      </div>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={10}>
          <Card title="选择学生和套餐">
            <Form form={form} layout="vertical" onFinish={(values) => createGrant.mutate(values)}>
              <Form.Item name="studentId" label="学生" rules={[{ required: true, message: '请选择学生' }]}>
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder="选择学生"
                  options={(students.data ?? []).map((item) => ({ label: `${item.name} · ${item.grade}`, value: item.id }))}
                  onChange={() => form.setFieldsValue({ packageId: undefined, startsAt: undefined, endsAt: undefined })}
                />
              </Form.Item>
              <Form.Item name="packageId" label="学习套餐" rules={[{ required: true, message: '请选择套餐' }]}>
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder={selectedStudent ? '选择学习套餐' : '请先选择学生'}
                  disabled={!selectedStudent}
                  options={availablePackages.map((item) => ({ label: packageOptionLabel(item), value: item.id }))}
                  notFoundContent={selectedStudent ? '该学生所在年级暂无启用套餐，请先到学习套餐中创建或启用对应年级套餐。' : '请先选择学生，再选择该年级可用套餐。'}
                  onChange={() => form.setFieldsValue({ startsAt: undefined, endsAt: undefined })}
                />
              </Form.Item>
              <Row gutter={12}>
                <Col span={12}>
                  <Form.Item name="startsAt" label="开始日期" rules={[{ required: true, message: '请选择开始日期' }]}>
                    <InputDate disabled={!packageId || preview.data?.alreadyOpened} />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="endsAt"
                    label="结束日期"
                    dependencies={['startsAt']}
                    rules={[
                      { required: true, message: '请选择结束日期' },
                      ({ getFieldValue }) => ({
                        validator(_, value) {
                          const start = getFieldValue('startsAt');
                          if (!value || !start || value >= start) return Promise.resolve();
                          return Promise.reject(new Error('结束日期不能早于开始日期'));
                        }
                      })
                    ]}
                  >
                    <InputDate disabled={!packageId || preview.data?.alreadyOpened} min={startsAt} />
                  </Form.Item>
                </Col>
              </Row>
              {selectedStudent && availablePackages.length === 0 && (
                <Alert type="warning" showIcon message="该学生所在年级暂无启用套餐，请先到学习套餐中创建或启用对应年级套餐。" style={{ marginBottom: 16 }} />
              )}
              <Button type="primary" block loading={createGrant.isPending} disabled={!preview.data || preview.data.alreadyOpened || !packageId} onClick={() => form.submit()}>
                {preview.data?.alreadyOpened ? '已开通' : '确认开通'}
              </Button>
            </Form>
          </Card>
        </Col>
        <Col xs={24} lg={14}>
          <Card title="本次学习权限预览">
            {preview.isLoading && <Skeleton active />}
            {!preview.data && !preview.isLoading && <Alert type="info" message="请选择学生和学习套餐后查看本次学习权限。" />}
            {preview.data && (
              <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                <Alert
                  type={preview.data.alreadyOpened ? 'info' : 'success'}
                  showIcon
                  message={preview.data.alreadyOpened ? `${preview.data.studentName} 已开通：${preview.data.packageName}` : `${preview.data.studentName} 将开通：${preview.data.packageName}`}
                  description={preview.data.alreadyOpened ? `当前有效期：${preview.data.existingStartsAt || '-'} 至 ${preview.data.existingUntil || '-'}` : `默认有效期：${preview.data.effectiveDefault}`}
                />
                <PreviewTags title="适用课程范围" values={preview.data.learningSpaces} color="cyan" />
                <PreviewTags title="包含学习内容" values={preview.data.contentTypes} color="geekblue" />
                <PreviewTags title="开放课程" values={preview.data.openCourses} color="blue" />
                <PreviewTags title="开放资料" values={preview.data.openMaterials} color="green" />
                <PreviewTags title="开放练习" values={preview.data.openHomework} color="purple" />
                <PreviewTags title="不会开放" values={preview.data.blockedContent} color="default" />
              </Space>
            )}
          </Card>
        </Col>
      </Row>
    </div>
  );
}

function grantBody(values: GrantFormValues): GrantCreateRequest {
  return {
    studentId: values.studentId,
    packageId: values.packageId,
    startsAt: values.startsAt,
    endsAt: values.endsAt
  };
}

function InputDate(props: { disabled?: boolean; min?: string; value?: string; onChange?: (event: any) => void }) {
  return <input className="ant-input" type="date" disabled={props.disabled} min={props.min} value={props.value || ''} onChange={props.onChange} />;
}

function packageOptionLabel(item: StudyPackage) {
  return [item.name, item.subject, item.semester, item.packageType].filter(Boolean).join(' · ');
}

function PreviewTags({ title, values, color }: { title: string; values: string[]; color: string }) {
  return (
    <div>
      <Typography.Text strong>{title}</Typography.Text>
      <div style={{ marginTop: 8 }}>
        {values.length === 0 ? <Typography.Text type="secondary">无</Typography.Text> : values.map((value) => <Tag key={value} color={color}>{value}</Tag>)}
      </div>
    </div>
  );
}
