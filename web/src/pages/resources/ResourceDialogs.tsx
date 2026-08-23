import { Alert, Button, Card, Checkbox, Empty, Form, Input, InputNumber, Modal, Pagination, Radio, Select, Skeleton, Space, Table, Tag, Typography, Upload, message } from 'antd';
import type { TableColumnsType, UploadFile } from 'antd';
import { CheckCircleOutlined, DeleteOutlined, DownloadOutlined, EditOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import type React from 'react';
import { getData, http, postData, postForm, putData } from '../../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../../components/ListViews';
import { DEFAULT_ACADEMIC_YEAR, academicYearForDate, formatLearningSpace, phaseLabel, semesterLabel, semesterOptions, subjectOptions, gradeOptions, subjectsForGrade } from '../../utils/curriculum';
import type { Course, CourseUpsertRequest, CurrentUser, Homework, HomeworkSubmissionSummary, HomeworkUpdateRequest, LearningSpace, Material, MaterialUpdateRequest, NoticeCreateRequest, PackageUpsertRequest, QuestionBankItem, QuestionBankUpsertRequest, Review, ReviewCompleteRequest, SettingUpdateRequest, StudyPackage } from '../../types/starline';

type Kind = 'packages' | 'content' | 'questions' | 'materials' | 'homework' | 'review' | 'notices' | 'logs' | 'settings';
type UploadKind = Extract<Kind, 'materials' | 'homework'>;
type PackageFormValues = PackageUpsertRequest;
type NoticeFormValues = NoticeCreateRequest;
export type CourseFormValues = CourseUpsertRequest & {
  grade?: string;
  subject?: string;
};
type SettingFormValues = SettingUpdateRequest;
type ContentFormValues = {
  title: string;
  courseId: string;
  chapter?: string;
  deadline?: string;
  status: string;
  questionIds?: string[];
};
type QuestionFormValues = QuestionBankUpsertRequest;

const CONTENT_TYPE_NAME: Record<string, string> = {
  course: '课程',
  question: '题',
  handout: '学习资料'
};

function packageContentLabel(codes?: string[]) {
  const order = ['course', 'question', 'handout'];
  return order.filter((code) => codes?.includes(code)).map((code) => CONTENT_TYPE_NAME[code]).join('+');
}

function autoPackageName(values: {
  academicYear?: string;
  grade?: string;
  subject?: string;
  semester?: string;
  learningSpaceIds?: string[];
  contentTypeCodes?: string[];
  learningSpaces: LearningSpace[];
}) {
  const { academicYear, grade, subject, semester, learningSpaceIds, contentTypeCodes, learningSpaces } = values;
  const parts = [academicYear, grade, semesterLabel(semester), subject].filter(Boolean).map(String);
  const selectedPhases = learningSpaces
    .filter((space) => learningSpaceIds?.includes(space.id))
    .map((space) => phaseLabel(space.phase))
    .filter(Boolean);
  const phases = Array.from(new Set(selectedPhases));
  if (phases.length > 0) parts.push(phases.join('+'));
  const content = packageContentLabel(contentTypeCodes);
  if (content) parts.push(content);
  return parts.join(' ');
}

function uploadFileListFromEvent(event: { fileList?: UploadFile[] }) {
  return event?.fileList ?? [];
}

function uploadFileTitle(file: UploadFile) {
  return String(file.name || '文件');
}

import { RichTextInput, courseSelectOptions, optionFromValues, questionAnswerOptions, questionTitle, questionTypeLabel, questionsForCourse, settingLabel, statusTag, uniqueValues } from './resource-shared';

function QuestionCheckboxGroup({
  value = [],
  onChange,
  selectedCourse,
  questions
}: {
  value?: string[];
  onChange?: (values: string[]) => void;
  selectedCourse?: Course;
  questions: QuestionBankItem[];
}) {
  if (!selectedCourse) {
    return <Typography.Text type="secondary">请先选择课程范围</Typography.Text>;
  }
  if (questions.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有匹配课程范围的启用题目，请先到题库出题。" />;
  }
  return (
    <Checkbox.Group value={value} onChange={(checkedValues) => onChange?.(checkedValues.map(String))} style={{ width: '100%' }}>
      <Space direction="vertical" size={8} style={{ width: '100%' }}>
        {questions.map((question) => (
          <Checkbox key={question.id} value={question.id}>
            <Space direction="vertical" size={0}>
              <Typography.Text>{questionTitle(question)}</Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {question.grade} · {semesterLabel(question.semester)} · {question.subject} · {questionTypeLabel(question.type)} · {question.score} 分
              </Typography.Text>
            </Space>
          </Checkbox>
        ))}
      </Space>
    </Checkbox.Group>
  );
}

export function HomeworkSubmissionDialog({
  homework,
  summary,
  loading,
  error,
  onCancel
}: {
  homework: Homework | null;
  summary?: HomeworkSubmissionSummary;
  loading: boolean;
  error: boolean;
  onCancel: () => void;
}) {
  const rows = summary?.students ?? [];
  return (
    <Modal
      title={summary?.homeworkTitle || homework?.title || '提交明细'}
      open={Boolean(homework)}
      footer={null}
      onCancel={onCancel}
      width={760}
      destroyOnHidden
    >
      {loading && <Skeleton active />}
      {error && <Alert type="error" showIcon message="提交明细加载失败，请稍后重试。" />}
      {!loading && !error && summary && (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message={`应提交 ${summary.totalNum} 人，已提交 ${summary.submittedNum} 人`}
          />
          <Table
            rowKey="studentId"
            size="small"
            dataSource={rows}
            pagination={{ pageSize: 8 }}
            columns={[
              { title: '学生姓名', dataIndex: 'studentName' },
              { title: '手机号', dataIndex: 'phone', width: 140 },
              { title: '提交时间', dataIndex: 'submittedAt', width: 170, render: (value) => value || '-' },
              { title: '批改状态', dataIndex: 'reviewStatus', width: 110, render: (value) => statusTag(String(value || '未提交')) }
            ]}
          />
        </Space>
      )}
    </Modal>
  );
}

export function ReviewBoard({ rows, onOpen }: { rows: Review[]; onOpen: (record: Record<string, unknown>) => void }) {
  const columns = ['待批改', '待复核', '已批改'];
  const statusOf = (review: Review) => {
    if (review.status === '待复核') return '待复核';
    if (review.status === '已批改') return '已批改';
    return '待批改';
  };
  if (rows.length === 0) return <Empty description="暂时没有待批改练习。" />;
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 16 }}>
      {columns.map((status) => {
        const items = rows.filter((row) => statusOf(row) === status);
        return (
          <div key={status} style={{ minWidth: 0 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <Typography.Text strong>{status}</Typography.Text>
              <Tag>{items.length}</Tag>
            </div>
            <Space direction="vertical" size={10} style={{ width: '100%' }}>
              {items.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无" />
              ) : items.map((review) => (
                <Card key={review.id} size="small">
                  <Space direction="vertical" size={6} style={{ width: '100%' }}>
                    <Typography.Text strong>{review.studentName}</Typography.Text>
                    <Typography.Text type="secondary">{review.homework}</Typography.Text>
                    <Space wrap>
                      <Tag color="blue">{review.packageName}</Tag>
                      <Tag color="green">系统评分 {review.systemScore}</Tag>
                    </Space>
                    <Button size="small" type="primary" onClick={() => onOpen(review as unknown as Record<string, unknown>)}>
                      填写反馈
                    </Button>
                  </Space>
                </Card>
              ))}
            </Space>
          </div>
        );
      })}
    </div>
  );
}

export function SettingDialog({
  form,
  setting,
  loading,
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<SettingFormValues>>[0];
  setting: SettingFormValues | null;
  loading: boolean;
  onCancel: () => void;
  onSubmit: (values: SettingFormValues) => void;
}) {
  return (
    <Modal
      title="编辑系统设置"
      open={Boolean(setting)}
      okText="保存"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
      styles={{ body: { maxHeight: 'calc(100vh - 240px)', overflowY: 'auto', paddingRight: 4 } }}
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="key" label="设置项">
          <Select
            disabled
            options={setting ? [{ label: settingLabel(setting.key), value: setting.key }] : []}
          />
        </Form.Item>
        <Form.Item name="value" label="当前值" rules={[{ required: true, message: '请输入设置值' }]}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} placeholder="请输入设置值" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export function CourseDialog({
  form,
  open,
  editing,
  loading,
  learningSpaces,
  allowedLearningSpaceIds,
  unrestricted,
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<CourseFormValues>>[0];
  open: boolean;
  editing: boolean;
  loading: boolean;
  learningSpaces: LearningSpace[];
  allowedLearningSpaceIds: string[];
  unrestricted: boolean;
  onCancel: () => void;
  onSubmit: (values: CourseFormValues) => void;
}) {
  const lastAutoName = useRef('');
  const availableSpaces = useMemo(
    () => learningSpaces.filter((space) => unrestricted || allowedLearningSpaceIds.includes(space.id)),
    [allowedLearningSpaceIds, learningSpaces, unrestricted]
  );
  const grade = Form.useWatch('grade', form);
  const subject = Form.useWatch('subject', form);
  const selectedSpaceId = Form.useWatch('learningSpaceId', form);
  const gradeSelectOptions = gradeOptions().filter((option) => availableSpaces.some((space) => space.grade === option.value));
  const subjectSelectOptions = subjectOptions(grade).filter((option) => (
    availableSpaces.some((space) => space.grade === grade && space.subject === option.value)
  ));
  const spaceOptions = availableSpaces
    .filter((space) => (!grade || space.grade === grade) && (!subject || space.subject === subject))
    .map((space) => ({ label: formatLearningSpace(space), value: space.id }));
  const hasSpaceOptions = spaceOptions.length > 0;
  const selectedSpace = availableSpaces.find((space) => space.id === selectedSpaceId);
  const autoCourseName = selectedSpace
    ? `${selectedSpace.grade}${selectedSpace.subject} ${selectedSpace.semester} ${selectedSpace.phase} 课程`
    : grade && subject
      ? `${grade}${subject}课程`
      : '';

  useEffect(() => {
    if (!autoCourseName || editing) return;
    const currentName = String(form.getFieldValue('name') || '').trim();
    if (currentName && currentName !== lastAutoName.current) return;
    form.setFieldValue('name', autoCourseName);
    lastAutoName.current = autoCourseName;
  }, [autoCourseName, editing, form]);

  useEffect(() => {
    if (!open) {
      lastAutoName.current = '';
      return;
    }
    if (!selectedSpaceId) return;
    const space = availableSpaces.find((item) => item.id === selectedSpaceId);
    if (!space) return;
    if (form.getFieldValue('grade') !== space.grade || form.getFieldValue('subject') !== space.subject) {
      form.setFieldsValue({ grade: space.grade, subject: space.subject });
    }
  }, [availableSpaces, form, open, selectedSpaceId]);

  return (
    <Modal
      title={editing ? '编辑课程' : '新增课程'}
      open={open}
      okText="保存"
      cancelText="取消"
      confirmLoading={loading}
      okButtonProps={{ disabled: !hasSpaceOptions }}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      {!availableSpaces.length && (
        <Alert
          type="info"
          showIcon
          message="当前账号还没有可维护的课程范围，请联系管理员分配年级和学科。"
          style={{ marginBottom: 16 }}
        />
      )}
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="name" label="课程名称" rules={[{ required: true, message: '请输入课程名称' }]}>
          <Input placeholder="例如：五年级英语 S1 Q1 阅读课程" />
        </Form.Item>
        <Space.Compact block>
          <Form.Item name="grade" label="年级" rules={[{ required: true, message: '请选择年级' }]} style={{ width: '50%' }}>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="选择年级"
              disabled={!availableSpaces.length}
              options={gradeSelectOptions}
              onChange={() => {
                form.setFieldsValue({ subject: undefined, learningSpaceId: undefined });
              }}
            />
          </Form.Item>
          <Form.Item name="subject" label="科目" rules={[{ required: true, message: '请选择科目' }]} style={{ width: '50%' }}>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="选择科目"
              disabled={!grade}
              options={subjectSelectOptions}
              onChange={() => {
                form.setFieldValue('learningSpaceId', undefined);
              }}
            />
          </Form.Item>
        </Space.Compact>
        <Form.Item name="learningSpaceId" label="课程范围" rules={[{ required: true, message: '请选择课程范围' }]}>
          <Select
            showSearch
            optionFilterProp="label"
            placeholder={subject ? '选择学期和阶段' : '请先选择年级和科目'}
            disabled={!grade || !subject || !hasSpaceOptions}
            notFoundContent={grade && subject ? '暂无匹配课程范围' : '请先选择年级和科目'}
            options={spaceOptions}
          />
        </Form.Item>
        <Form.Item name="chapterCount" label="章节数" rules={[{ required: true, message: '请输入章节数' }]}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Select
            options={[
              { label: '启用', value: '启用' },
              { label: '草稿', value: '草稿' },
              { label: '停用', value: '停用' }
            ]}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export function NoticeDialog({
  form,
  open,
  loading,
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<NoticeFormValues>>[0];
  open: boolean;
  loading: boolean;
  onCancel: () => void;
  onSubmit: (values: NoticeFormValues) => void;
}) {
  const channel = Form.useWatch('channel', form);
  return (
    <Modal
      title="发送通知"
      open={open}
      okText="发送"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="type" label="通知类型" rules={[{ required: true, message: '请选择通知类型' }]}>
          <Select
            options={[
              { label: '通知', value: '通知' },
              { label: '练习提醒', value: '练' },
              { label: '批改反馈', value: '评' },
              { label: '课程到期', value: '权' },
              { label: '资料更新', value: '资' }
            ]}
          />
        </Form.Item>
        <Form.Item name="channel" label="发送渠道" rules={[{ required: true, message: '请选择发送渠道' }]}>
          <Select
            options={[
              { label: '站内通知', value: '站内通知' },
              { label: '公众号模板消息', value: '公众号模板消息' }
            ]}
          />
        </Form.Item>
        {channel === '公众号模板消息' && (
          <Form.Item name="recipientOpenId" label="公众号接收人 openid">
            <Input placeholder="可留空；接收对象精确填写学生姓名或手机号时自动匹配" />
          </Form.Item>
        )}
        <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入通知标题' }]}>
          <Input placeholder="例如：英语阅读挑战已发布" />
        </Form.Item>
        <Form.Item name="target" label="接收对象" rules={[{ required: true, message: '请输入接收对象' }]}>
          <Input placeholder="例如：五年级英语班 / 小明 / 全部学生" />
        </Form.Item>
        <Form.Item name="summary" label="通知内容" rules={[{ required: true, message: '请输入通知内容' }]}>
          <Input.TextArea rows={4} placeholder="用学生和家长能看懂的话说明要做什么。" />
        </Form.Item>
        <Space.Compact block>
          <Form.Item name="relatedType" label="关联类型" style={{ width: '50%' }}>
            <Select
              allowClear
              placeholder="可选"
              options={[
                { label: '课程', value: 'course' },
                { label: '练习', value: 'homework' },
                { label: '资料', value: 'material' },
                { label: '批改', value: 'review' }
              ]}
            />
          </Form.Item>
          <Form.Item name="relatedId" label="关联对象" style={{ width: '50%' }}>
            <Input placeholder="可填写业务 ID" />
          </Form.Item>
        </Space.Compact>
      </Form>
    </Modal>
  );
}

export function PackageDialog({
  form,
  open,
  editing,
  loading,
  learningSpaces,
  academicYearOptions,
  semesterOptions,
  initialValues,
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<PackageFormValues>>[0];
  open: boolean;
  editing: boolean;
  loading: boolean;
  learningSpaces: LearningSpace[];
  academicYearOptions: Array<{ label: string; value: string }>;
  semesterOptions: Array<{ label: string; value: string }>;
  initialValues: PackageFormValues;
  onCancel: () => void;
  onSubmit: (values: PackageFormValues) => void;
}) {
  const grade = Form.useWatch('grade', form);
  const subject = Form.useWatch('subject', form);
  const semester = Form.useWatch('semester', form);
  const academicYear = Form.useWatch('academicYear', form);
  const learningSpaceIds = Form.useWatch('learningSpaceIds', form);
  const contentTypeCodes = Form.useWatch('contentTypeCodes', form);
  const [autoNameEnabled, setAutoNameEnabled] = useState(!editing);
  const lastAutoName = useRef('');
  const spaceOptions = learningSpaces
    .filter((space) => (!grade || space.grade === grade) && (!subject || space.subject === subject) && (!semester || space.semester === semester))
    .map((space) => ({ label: `${phaseLabel(space.phase)} · ${formatLearningSpace(space)}`, value: space.id }));

  useEffect(() => {
    if (!open || editing || !autoNameEnabled) return;
    const generated = autoPackageName({ academicYear, grade, subject, semester, learningSpaceIds, contentTypeCodes, learningSpaces });
    if (!generated) return;
    lastAutoName.current = generated;
    form.setFieldValue('name', generated);
  }, [academicYear, autoNameEnabled, contentTypeCodes, editing, form, grade, learningSpaceIds, learningSpaces, open, semester, subject]);

  return (
    <Modal
      title={editing ? '编辑课程方案' : '新增课程方案'}
      open={open}
      okText="保存"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      afterOpenChange={(visible) => {
        if (visible) {
          form.setFieldsValue(initialValues);
          lastAutoName.current = initialValues.name || '';
          setAutoNameEnabled(!editing);
        }
      }}
      destroyOnHidden
      width={720}
    >
        <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="name" label="方案名称" rules={[{ required: true, message: '请输入方案名称' }]}>
          <Input
            placeholder="下面选完后自动生成，也可以手动修改"
            onChange={(event) => {
              const value = event.target.value;
              setAutoNameEnabled(!value.trim() || value === lastAutoName.current);
            }}
          />
        </Form.Item>
        <Space size={12} align="start" wrap style={{ width: '100%' }}>
          <Form.Item name="academicYear" label="学年" rules={[{ required: true, message: '请选择学年' }]}>
            <Select style={{ width: 160 }} options={academicYearOptions} />
          </Form.Item>
          <Form.Item name="grade" label="年级" rules={[{ required: true, message: '请选择年级' }]}>
            <Select
              style={{ width: 150 }}
              options={gradeOptions()}
              onChange={(value) => {
                const currentSubject = form.getFieldValue('subject');
                if (currentSubject && !subjectsForGrade(value).includes(currentSubject)) {
                  form.setFieldValue('subject', undefined);
                }
                form.setFieldValue('learningSpaceIds', []);
              }}
            />
          </Form.Item>
          <Form.Item name="subject" label="学科" rules={[{ required: true, message: '请选择学科' }]}>
            <Select style={{ width: 150 }} options={subjectOptions(grade)} onChange={() => form.setFieldValue('learningSpaceIds', [])} />
          </Form.Item>
          <Form.Item name="semester" label="学期" rules={[{ required: true, message: '请选择学期' }]}>
            <Select
              style={{ width: 150 }}
              options={semesterOptions}
              onChange={() => form.setFieldValue('learningSpaceIds', [])}
            />
          </Form.Item>
        </Space>
        <Form.Item name="learningSpaceIds" label="开放学习空间" rules={[{ required: true, message: '请选择开放学习空间' }]}>
          <Select
            mode="multiple"
            showSearch
            optionFilterProp="label"
            placeholder="选择 Q1 期中、Q2 期末等学习空间"
            options={spaceOptions}
          />
        </Form.Item>
        <Form.Item name="contentTypeCodes" label="开放内容" rules={[{ required: true, message: '请选择开放内容' }]}>
          <Select
            mode="multiple"
            options={[
              { label: '课程', value: 'course' },
              { label: '题', value: 'question' },
              { label: '学习资料', value: 'handout' }
            ]}
          />
        </Form.Item>
        <Space size={12} align="start" wrap style={{ width: '100%' }}>
          <Form.Item name="phaseScope" label="适用阶段">
            <Input style={{ width: 180 }} placeholder="全学期" />
          </Form.Item>
          <Form.Item name="packageType" label="方案类型">
            <Input style={{ width: 180 }} placeholder="不填则按开放内容自动生成" />
          </Form.Item>
          <Form.Item name="status" label="状态">
            <Select
              style={{ width: 140 }}
              options={[
                { label: '启用', value: '启用' },
                { label: '草稿', value: '草稿' },
                { label: '停用', value: '停用' }
              ]}
            />
          </Form.Item>
        </Space>
        <Form.Item name="summary" label="说明">
          <Input.TextArea rows={3} placeholder="说明这个方案适合什么学生、包含哪些课程内容。" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export function ReviewDialog({
  form,
  review,
  loading,
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<ReviewCompleteRequest>>[0];
  review: Review | null;
  loading: boolean;
  onCancel: () => void;
  onSubmit: (values: ReviewCompleteRequest) => void;
}) {
  return (
    <Modal
      title="填写批改反馈"
      open={Boolean(review)}
      okText="发送反馈"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Alert
          type="info"
          showIcon
          message={review ? `${review.studentName} · ${review.homework}` : ''}
          style={{ marginBottom: 16 }}
        />
        <Form.Item name="score" label="分数" rules={[{ required: true, message: '请输入分数' }]}>
          <Input type="number" min={0} max={100} />
        </Form.Item>
        <Form.Item name="teacherComment" label="老师建议" rules={[{ required: true, message: '请输入给学生看的建议' }]}>
          <RichTextInput placeholder="例如：阅读理解整体不错，注意把答案依据写完整。" />
        </Form.Item>
        <Form.Item name="reward" label="奖励">
          <Input placeholder="例如：阅读小星星" />
        </Form.Item>
        <Form.Item name="finalStatus" label="批改状态" initialValue="已批改">
          <Select
            options={[
              { label: '待复核', value: '待复核' },
              { label: '已批改', value: '已批改' }
            ]}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export function QuestionDialog({
  form,
  open,
  editing,
  loading,
  scopeLoading,
  scopeError,
  gradeOptions: scopedGradeOptions,
  semesterOptions,
  allowedSpaces,
  unrestricted,
  hasScope,
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<QuestionFormValues>>[0];
  open: boolean;
  editing: boolean;
  loading: boolean;
  scopeLoading: boolean;
  scopeError: boolean;
  gradeOptions: Array<{ label: string; value: string }>;
  semesterOptions: Array<{ label: string; value: string }>;
  allowedSpaces: LearningSpace[];
  unrestricted: boolean;
  hasScope: boolean;
  onCancel: () => void;
  onSubmit: (values: QuestionFormValues) => void;
}) {
  const type = Form.useWatch('type', form);
  const grade = Form.useWatch('grade', form);
  const semester = Form.useWatch('semester', form);
  const scopedSemesterOptions = unrestricted
    ? semesterOptions
    : semesterOptions.filter((option) => allowedSpaces.some((space) => (!grade || space.grade === grade) && space.semester === option.value));
  const scopedSubjectOptions = unrestricted
    ? subjectOptions(grade)
    : optionFromValues(uniqueValues(allowedSpaces
      .filter((space) => (!grade || space.grade === grade) && (!semester || space.semester === semester))
      .map((space) => space.subject)));
  const saveDisabled = loading || scopeLoading || scopeError || !hasScope;
  useEffect(() => {
    if (!open || editing || scopeLoading || scopeError || !hasScope) return;
    const nextValues: Partial<QuestionFormValues> = {};
    if (!form.getFieldValue('grade') && scopedGradeOptions.length === 1) {
      nextValues.grade = scopedGradeOptions[0].value;
    }
    if (!form.getFieldValue('semester') && scopedSemesterOptions.length === 1) {
      nextValues.semester = scopedSemesterOptions[0].value;
    }
    if (!form.getFieldValue('subject') && scopedSubjectOptions.length === 1) {
      nextValues.subject = scopedSubjectOptions[0].value;
    }
    if (Object.keys(nextValues).length > 0) {
      form.setFieldsValue(nextValues);
    }
  }, [editing, form, hasScope, open, scopeError, scopeLoading, scopedGradeOptions, scopedSemesterOptions, scopedSubjectOptions]);
  return (
    <Modal
      title={editing ? '编辑题库题目' : '新增题库题目'}
      open={open}
      okText="保存"
      cancelText="取消"
      className="question-dialog"
      confirmLoading={loading}
      okButtonProps={{ disabled: saveDisabled }}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        {scopeLoading && (
          <Alert type="info" showIcon message="正在读取可维护课程范围..." style={{ marginBottom: 16 }} />
        )}
        {scopeError && (
          <Alert type="error" showIcon message="课程范围加载失败，请刷新后再试。" style={{ marginBottom: 16 }} />
        )}
        {!scopeLoading && !scopeError && !hasScope && (
          <Alert type="warning" showIcon message="当前账号还没有可维护的课程范围，请联系管理员分配年级和学科。" style={{ marginBottom: 16 }} />
        )}
        <Space.Compact block>
          <Form.Item name="grade" rules={[{ required: true, message: '请选择年级' }]} style={{ width: '33%' }}>
            <Select
              placeholder="年级"
              options={scopedGradeOptions}
              disabled={saveDisabled}
              onChange={(nextGrade) => {
                const currentSemester = form.getFieldValue('semester');
                const nextSemesterOptions = unrestricted
                  ? semesterOptions
                  : semesterOptions.filter((option) => allowedSpaces.some((space) => space.grade === nextGrade && space.semester === option.value));
                if (currentSemester && !nextSemesterOptions.some((option) => option.value === currentSemester)) {
                  form.setFieldValue('semester', undefined);
                }
                form.setFieldValue('subject', undefined);
              }}
            />
          </Form.Item>
          <Form.Item name="semester" rules={[{ required: true, message: '请选择学期' }]} style={{ width: '33%' }}>
            <Select
              placeholder="学期"
              options={scopedSemesterOptions}
              disabled={saveDisabled}
              onChange={() => form.setFieldValue('subject', undefined)}
            />
          </Form.Item>
          <Form.Item name="subject" rules={[{ required: true, message: '请选择学科' }]} style={{ width: '34%' }}>
            <Select placeholder="学科" options={scopedSubjectOptions} disabled={saveDisabled} />
          </Form.Item>
        </Space.Compact>
        <Form.Item name="title" label="题目名称" rules={[{ required: true, whitespace: true, message: '请输入题目名称' }]}>
          <Input placeholder="例如：阅读理解第 1 题" maxLength={60} showCount />
        </Form.Item>
        <Form.Item name="type" label="题型" rules={[{ required: true, message: '请选择题型' }]}>
          <Select options={[
            { label: '单选题', value: 'single' },
            { label: '多选题', value: 'multiple' },
            { label: '判断题', value: 'judge' },
            { label: '填空题', value: 'fill' },
            { label: '简答题', value: 'text' }
          ]} onChange={(nextType) => {
            if (nextType === 'text' || nextType === 'fill') {
              form.setFieldsValue({ options: [], answer: undefined, answers: [] });
              return;
            }
            if (nextType === 'judge') {
              form.setFieldsValue({ options: ['正确', '错误'], answer: undefined, answers: [] });
              return;
            }
            form.setFieldsValue({ options: form.getFieldValue('options')?.length ? form.getFieldValue('options') : ['', '', '', ''], answer: undefined, answers: [] });
          }} />
        </Form.Item>
        <Form.Item name="stem" label="题干" rules={[{ required: true, message: '请输入题干' }]}>
          <RichTextInput placeholder="请输入学生看到的题目内容，可添加重点、列表或图片 URL。" />
        </Form.Item>
        {type !== 'text' && type !== 'fill' && (
          <>
            <Form.List
              name="options"
              rules={[{
                validator: async (_, options?: string[]) => {
                  const filledOptions = (options ?? []).map((item) => String(item || '').trim()).filter(Boolean);
                  if (filledOptions.length < 2) throw new Error('至少录入 2 个选项');
                }
              }]}
            >
              {(fields, { add, remove }, { errors }) => (
                <Form.Item label="选项" required>
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    {fields.map((field, index) => (
                      <Space.Compact key={field.key} block>
                        <div style={{ width: 44, display: 'flex', alignItems: 'center', justifyContent: 'center', border: '1px solid #d9e2ec', borderRight: 0, borderRadius: '8px 0 0 8px', background: '#f8fafc', color: '#64748b', fontWeight: 600 }}>
                          {String.fromCharCode(65 + index)}
                        </div>
                        <Form.Item
                          {...field}
                          noStyle
                          rules={[{ required: true, whitespace: true, message: `请输入选项 ${String.fromCharCode(65 + index)}` }]}
                        >
                          <Input disabled={type === 'judge'} placeholder={`请输入选项 ${String.fromCharCode(65 + index)} 内容`} />
                        </Form.Item>
                        <Button
                          icon={<DeleteOutlined />}
                          disabled={fields.length <= 2 || type === 'judge'}
                          onClick={() => {
                            const removedValue = form.getFieldValue(['options', field.name]);
                            remove(field.name);
                            if (type === 'single' && form.getFieldValue('answer') === removedValue) form.setFieldValue('answer', undefined);
                            if (type === 'multiple') {
                              const nextAnswers = (form.getFieldValue('answers') ?? []).filter((item: string) => item !== removedValue);
                              form.setFieldValue('answers', nextAnswers);
                            }
                          }}
                        />
                      </Space.Compact>
                    ))}
                    <Button type="dashed" icon={<PlusOutlined />} disabled={type === 'judge'} onClick={() => add('')} block>
                      添加选项
                    </Button>
                    <Form.ErrorList errors={errors} />
                  </Space>
                </Form.Item>
              )}
            </Form.List>
            {type === 'single' || type === 'judge' ? (
              <Form.Item label="正确答案" required>
                <Form.Item shouldUpdate noStyle>
                  {() => {
                    const answerOptions = questionAnswerOptions(form.getFieldValue('options'));
                    return (
                      <Form.Item name="answer" noStyle rules={[{ required: true, message: '请选择正确答案' }]}>
                        <Radio.Group options={answerOptions} />
                      </Form.Item>
                    );
                  }}
                </Form.Item>
              </Form.Item>
            ) : (
              <Form.Item label="正确答案" required>
                <Form.Item shouldUpdate noStyle>
                  {() => {
                    const answerOptions = questionAnswerOptions(form.getFieldValue('options'));
                    return (
                      <Form.Item name="answers" noStyle rules={[{ required: true, message: '请选择正确答案' }]}>
                        <Checkbox.Group options={answerOptions} />
                      </Form.Item>
                    );
                  }}
                </Form.Item>
              </Form.Item>
            )}
          </>
        )}
        {type === 'fill' && (
          <Form.Item name="answer" label="参考答案" rules={[{ required: true, whitespace: true, message: '请输入填空题参考答案' }]}>
            <Input placeholder="学生答案与参考答案一致时自动判分" />
          </Form.Item>
        )}
        <Space.Compact block>
          <Form.Item name="score" label="分值" style={{ width: '50%' }} rules={[{ required: true, message: '请输入分值' }]}>
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="status" label="状态" style={{ width: '50%' }}>
            <Select options={[
              { label: '启用', value: '启用' },
              { label: '草稿', value: '草稿' },
              { label: '停用', value: '停用' }
            ]} />
          </Form.Item>
        </Space.Compact>
      </Form>
    </Modal>
  );
}

export function UploadDialog({
  kind,
  open,
  loading,
  courses,
  questions,
  learningSpaces,
  onCancel,
  onSubmit
}: {
  kind: UploadKind;
  open: boolean;
  loading: boolean;
  courses: Course[];
  questions: QuestionBankItem[];
  learningSpaces: LearningSpace[];
  onCancel: () => void;
  onSubmit: (values: { title: string; courseId: string; chapter?: string; deadline?: string; questionIds?: string[]; fileList?: UploadFile[] }) => void;
}) {
  const [form] = Form.useForm();
  const courseId = Form.useWatch('courseId', form);
  const selectedCourse = courses.find((course) => course.id === courseId);
  const availableQuestions = questionsForCourse(selectedCourse, questions, learningSpaces);
  return (
    <Modal
      title={kind === 'materials' ? '上传学习资料' : '组卷发布小挑战'}
      open={open}
      okText={kind === 'materials' ? '上传' : '发布'}
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="title" label={kind === 'materials' ? '资料标题' : '练习标题'} rules={kind === 'homework' ? [{ required: true, message: '请输入标题' }] : []}>
          <Input placeholder={kind === 'materials' ? '单文件可填写；批量上传默认使用文件名' : '例如：五年级英语 S1 Q1 阅读练习'} />
        </Form.Item>
        <Form.Item name="courseId" label="课程范围" rules={[{ required: true, message: '请选择课程范围' }]}>
          <Select
            showSearch
            placeholder="选择学生可学习的课程"
            optionFilterProp="searchLabel"
            options={courseSelectOptions(courses)}
            onChange={() => {
              if (kind === 'homework') form.setFieldValue('questionIds', []);
            }}
          />
        </Form.Item>
        {kind === 'materials' ? (
          <Form.Item name="chapter" label="章节">
            <Input placeholder="不填则归为未分章节" />
          </Form.Item>
        ) : (
          <>
            <Alert
              type="info"
              showIcon
              message="先选课程，再从同年级、同学期、同学科的题库中手动选题组卷。"
              style={{ marginBottom: 16 }}
            />
            <Form.Item name="deadline" label="截止时间">
              <Input placeholder="例如：2026-10-30" />
            </Form.Item>
            <Form.Item name="questionIds" label="选择题目" rules={[{ required: true, message: '请选择题目' }]}>
              <QuestionCheckboxGroup selectedCourse={selectedCourse} questions={availableQuestions} />
            </Form.Item>
            {selectedCourse && availableQuestions.length === 0 && (
              <Alert type="warning" showIcon message="当前课程暂无可用题目，请先到题库新增同年级、同学期、同学科的启用题目。" />
            )}
          </>
        )}
        {kind === 'materials' && (
          <>
            <Form.Item
              name="fileList"
              label="文件"
              valuePropName="fileList"
              getValueFromEvent={uploadFileListFromEvent}
              rules={[
                { required: true, message: '请选择文件' },
                {
                  validator: (_, value?: UploadFile[]) => {
                    const oversized = (value ?? []).find((file) => Number(file.size || 0) > 50 * 1024 * 1024);
                    return oversized ? Promise.reject(new Error(`${uploadFileTitle(oversized)} 超过 50MB`)) : Promise.resolve();
                  }
                }
              ]}
            >
              <Upload beforeUpload={() => false} multiple accept=".pdf,.ppt,.pptx,.doc,.docx">
                <Button icon={<UploadOutlined />}>选择文件</Button>
              </Upload>
            </Form.Item>
            <Typography.Text type="secondary">支持 PDF、PPT、Word，可一次选择多个文件，单个文件不超过 50MB。</Typography.Text>
          </>
        )}
      </Form>
    </Modal>
  );
}

export function ContentEditDialog({
  kind,
  form,
  item,
  loading,
  courses,
  questions,
  learningSpaces,
  onCancel,
  onSubmit
}: {
  kind: UploadKind;
  form: ReturnType<typeof Form.useForm<ContentFormValues>>[0];
  item: Material | Homework | null;
  loading: boolean;
  courses: Course[];
  questions: QuestionBankItem[];
  learningSpaces: LearningSpace[];
  onCancel: () => void;
  onSubmit: (values: ContentFormValues) => void;
}) {
  const courseId = Form.useWatch('courseId', form);
  const selectedCourse = courses.find((course) => course.id === courseId);
  const availableQuestions = questionsForCourse(selectedCourse, questions, learningSpaces);
  return (
    <Modal
      title={kind === 'materials' ? '编辑学习资料' : '编辑小挑战组卷'}
      open={Boolean(item)}
      okText="保存"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="title" label={kind === 'materials' ? '资料标题' : '题目标题'} rules={[{ required: true, message: '请输入标题' }]}>
          <Input placeholder={kind === 'materials' ? '例如：五年级英语 S1 Q1 核心资料' : '例如：五年级英语 S1 Q1 阅读练习题'} />
        </Form.Item>
        <Form.Item name="courseId" label="课程范围" rules={[{ required: true, message: '请选择课程范围' }]}>
          <Select
            showSearch
            placeholder="选择学生可学习的课程"
            optionFilterProp="searchLabel"
            options={courseSelectOptions(courses)}
            onChange={() => {
              if (kind === 'homework') form.setFieldValue('questionIds', []);
            }}
          />
        </Form.Item>
        {kind === 'materials' ? (
          <Form.Item name="chapter" label="章节">
            <Input placeholder="不填则归为未分章节" />
          </Form.Item>
        ) : (
          <>
            <Form.Item name="deadline" label="截止时间">
              <Input placeholder="例如：2026-10-30" />
            </Form.Item>
            <Form.Item name="questionIds" label="选择题目" rules={[{ required: true, message: '请选择题目' }]}>
              <QuestionCheckboxGroup selectedCourse={selectedCourse} questions={availableQuestions} />
            </Form.Item>
            {selectedCourse && availableQuestions.length === 0 && (
              <Alert type="warning" showIcon message="当前课程暂无可用题目，请先到题库新增同年级、同学期、同学科的启用题目。" />
            )}
          </>
        )}
        <Form.Item name="status" label="状态">
          <Select
            options={kind === 'materials'
              ? [
                { label: '已发布', value: '已发布' },
                { label: '停用', value: '停用' }
              ]
              : [
                { label: '启用', value: '启用' },
                { label: '草稿', value: '草稿' },
                { label: '停用', value: '停用' }
              ]}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
