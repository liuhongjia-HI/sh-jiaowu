import { Alert, Button, Card, Checkbox, Empty, Form, Input, InputNumber, Modal, Pagination, Radio, Select, Skeleton, Space, Table, Tag, Typography, Upload, message } from 'antd';
import type { TableColumnsType, UploadFile } from 'antd';
import { CheckCircleOutlined, DeleteOutlined, DownloadOutlined, EditOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type React from 'react';
import { getData, http, postData, postForm, putData } from '../../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../../components/ListViews';
import { subjectAccent } from '../../utils/subject-colors';
import { DEFAULT_ACADEMIC_YEAR, academicYearForDate, formatLearningSpace, phaseLabel, semesterLabel, semesterOptions, subjectOptions, gradeOptions, subjectsForGrade } from '../../utils/curriculum';
import type { Course, CourseUpsertRequest, CurrentUser, Homework, HomeworkSubmissionSummary, HomeworkUpdateRequest, LearningSpace, Material, MaterialUpdateRequest, NoticeCreateRequest, PackageUpsertRequest, QuestionBankItem, QuestionBankUpsertRequest, Review, ReviewCompleteRequest, SettingUpdateRequest, StudyPackage } from '../../types/starline';

export type ResourceKind = 'packages' | 'content' | 'questions' | 'materials' | 'homework' | 'review' | 'notices' | 'logs' | 'settings';
type Kind = ResourceKind;
type UploadKind = Extract<Kind, 'materials' | 'homework'>;
type PackageFormValues = PackageUpsertRequest;
type NoticeFormValues = NoticeCreateRequest;
type CourseFormValues = CourseUpsertRequest;
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

export const config: Record<Kind, { title: string; desc: string; path: string }> = {
  packages: { title: '课程方案', desc: '维护年级、学科和自动开放内容。', path: '/packages' },
  content: { title: '课程内容', desc: '维护课程、章节和课节安排。', path: '/courses' },
  questions: { title: '题库', desc: '按年级、学期和学科维护可复用题目。', path: '/questions' },
  materials: { title: '课程讲义', desc: '维护课程讲义、图片和课件。', path: '/materials' },
  homework: { title: '课后练习', desc: '从题库选题组卷并发布到学习空间。', path: '/homework' },
  review: { title: '批改反馈', desc: '处理分数、评语和学习反馈。', path: '/reviews/pending' },
  notices: { title: '通知提醒', desc: '发送练习、批改、资料和到期提醒。', path: '/notices' },
  logs: { title: '操作记录', desc: '查看开通、访问和后台操作。', path: '/logs' },
  settings: { title: '系统设置', desc: '维护学年、水印、访问和提醒规则。', path: '/settings' }
};

export const emptyTextByKind: Record<Kind, string> = {
  packages: '还没有课程方案，先配置课程包含的资料和练习。',
  content: '还没有课程内容，先维护课程和章节。',
  questions: '还没有题库题目，先按年级、学期和学科新增题目。',
  materials: '还没有课程讲义，上传后会随课程方案自动开放。',
  homework: '还没有课后练习，先从题库选题组卷。',
  review: '暂时没有待批改练习。',
  notices: '还没有通知提醒。',
  logs: '还没有操作记录。',
  settings: '还没有系统设置。'
};

export const settingOrder = [
  'academicYear',
  'grades',
  'semesters',
  'watermarkRule',
  'downloadPolicy',
  'miniProgramDomainStatus',
  'productionApiDomain',
  'officialAccountBindingStatus',
  'templateMessageStatus'
];

export function columnsFor(kind: Kind, rows: Record<string, unknown>[], renderActions?: (record: Record<string, unknown>) => React.ReactNode): TableColumnsType<Record<string, unknown>> {
  if (kind === 'settings') {
    const columns: TableColumnsType<Record<string, unknown>> = [
      { title: '设置项', dataIndex: 'key', render: (value: unknown) => settingLabel(String(value || '')) },
      { title: '当前值', dataIndex: 'value' }
    ];
    if (renderActions) {
      columns.push({ title: '操作', fixed: 'right', width: 120, render: (_: unknown, record: Record<string, unknown>) => renderActions(record) });
    }
    return columns;
  }

  const customKeys: Partial<Record<Kind, string[]>> = {
    materials: ['title', 'course', 'fileType', 'fileName', 'previewStatus', 'ownerTeacherName', 'publishStatus'],
    questions: ['title', 'grade', 'semester', 'subject', 'type', 'stem', 'ownerTeacherName', 'status'],
    homework: ['title', 'course', 'questionNum', 'ownerTeacherName', 'deadline', 'publishStatus'],
    notices: ['type', 'title', 'target', 'channel', 'status', 'failureReason']
  };
  const keys = customKeys[kind] ?? Object.keys(rows[0] ?? {}).slice(0, 6);
  const columns: TableColumnsType<Record<string, unknown>> = keys.map((key) => ({
    title: labelOf(key),
    dataIndex: key,
    render: (value: unknown) => {
      if (Array.isArray(value)) return value.join('、');
      if (key === 'status' || key === 'accountStatus' || key === 'previewStatus') return statusTag(String(value || '-'));
      return displayFieldValue(key, value);
    }
  }));
  if (renderActions && (kind === 'packages' || kind === 'content' || kind === 'questions' || kind === 'materials' || kind === 'homework' || kind === 'review' || kind === 'notices')) {
    columns.push({ title: '操作', fixed: 'right', width: 120, render: (_: unknown, record: Record<string, unknown>) => renderActions(record) });
  }
  return columns;
}

export function titleFor(row: Record<string, unknown>) {
  if (row.key && row.value !== undefined && Object.keys(row).length === 2) return settingLabel(String(row.key));
  if (row.stem && !row.name && !row.title) return questionTitle(row as QuestionBankItem);
  return String(row.name ?? row.title ?? row.packageName ?? row.course ?? row.action ?? row.key ?? '未命名');
}

export function subtitleFor(kind: Kind, row: Record<string, unknown>) {
  if (kind === 'settings') return '系统设置';
  const parts = [row.grade, row.subject, row.course, row.ownerTeacherName, row.target].filter(Boolean);
  return parts.map(String).join(' · ') || undefined;
}

export function statusFor(row: Record<string, unknown>) {
  const value = row.previewStatus ?? row.status ?? row.publishStatus ?? row.accountStatus;
  if (!value) return undefined;
  return statusTag(String(value));
}

export function fieldsFor(kind: Kind, row: Record<string, unknown>) {
  if (kind === 'settings') return [{ label: '当前值', value: displayValue(row.value) }];

  const keysByKind: Partial<Record<Kind, string[]>> = {
    packages: ['academicYear', 'grade', 'semester', 'subject', 'phaseScope', 'packageType', 'openStudentNum'],
    content: ['grade', 'subject', 'chapterCount', 'materialNum', 'homeworkNum'],
    questions: ['grade', 'semester', 'subject', 'type', 'score', 'ownerTeacherName'],
    materials: ['course', 'fileType', 'fileName', 'ownerTeacherName', 'publishStatus', 'viewCount'],
    homework: ['course', 'questionNum', 'ownerTeacherName', 'deadline', 'submittedNum', 'totalNum'],
    review: ['studentName', 'packageName', 'homework', 'systemScore'],
    notices: ['type', 'target', 'channel', 'status', 'failureReason', 'retryCount'],
    logs: ['operator', 'action', 'target', 'time']
  };
  return (keysByKind[kind] ?? Object.keys(row).filter((key) => !['id', 'name', 'title', 'status'].includes(key)).slice(0, 6))
    .filter((key) => row[key] !== undefined && !Array.isArray(row[key]))
    .slice(0, 6)
    .map((key) => ({ label: labelOf(key), value: displayFieldValue(key, row[key]) }));
}

export function tagsFor(row: Record<string, unknown>) {
  const hiddenArrayKeys = new Set(['learningSpaceIds', 'contentTypeCodes']);
  const arrays = Object.entries(row)
    .filter(([key, value]) => Array.isArray(value) && !hiddenArrayKeys.has(key) && value.every((item) => ['string', 'number', 'boolean'].includes(typeof item)))
    .map(([key, value]) => [key, (value as Array<string | number | boolean>).map(String)] as [string, string[]]);
  if (arrays.length === 0) return undefined;
  return (
    <>
      {arrays.slice(0, 3).map(([key, values]) => (
        <div key={key} style={{ marginTop: 6 }}>
          <Typography.Text type="secondary">{labelOf(key)}：</Typography.Text>
          <TagGroup values={values} color="blue" />
        </div>
      ))}
    </>
  );
}

export function displayValue(value: unknown) {
  if (Array.isArray(value)) return value.join('、') || '-';
  if (value === null || value === undefined || value === '') return '-';
  return String(value);
}

export function displayFieldValue(key: string, value: unknown) {
  if (key === 'semester') return semesterLabel(String(value || '')) || '-';
  if (key === 'phase' || key === 'phaseScope') return phaseLabel(String(value || '')) || displayValue(value);
  return displayValue(value);
}

export function questionTypeLabel(type?: string) {
  if (type === 'single') return '单选';
  if (type === 'multiple') return '多选';
  if (type === 'judge') return '判断';
  if (type === 'fill') return '填空';
  if (type === 'text') return '简答';
  return type || '-';
}

export function questionAnswerOptions(options?: string[]) {
  return (options ?? [])
    .map((value, index) => ({ value: String(value || '').trim(), index }))
    .filter((item) => item.value)
    .map((item) => ({
      label: `${String.fromCharCode(65 + item.index)}. ${item.value}`,
      value: item.value
    }));
}

export function questionTitle(question: Pick<QuestionBankItem, 'title' | 'stem'>) {
  const title = String(question.title || '').trim();
  if (title) return title;
  const stem = richTextPlainText(question.stem);
  return stem.length > 24 ? `${stem.slice(0, 24)}...` : stem || '未命名';
}

export function questionSelectLabel(question: QuestionBankItem) {
  const stem = richTextPlainText(question.stem);
  const title = questionTitle(question);
  const stemSuffix = stem && stem !== title ? ` · ${stem}` : '';
  return `${question.grade} ${semesterLabel(question.semester)} ${question.subject} · ${questionTypeLabel(question.type)} · ${title}${stemSuffix}`;
}

export function normalizeQuestionForm(values: QuestionFormValues): QuestionBankUpsertRequest {
  const options = (values.options ?? []).map((item) => String(item).trim()).filter(Boolean);
  const answers = (values.answers ?? []).map((item) => String(item).trim()).filter(Boolean);
  return {
    ...values,
    title: String(values.title || '').trim(),
    stem: String(values.stem || '').trim(),
    options: values.type === 'text' || values.type === 'fill' ? [] : values.type === 'judge' ? ['正确', '错误'] : options,
    answer: values.type === 'single' || values.type === 'judge' || values.type === 'fill' ? String(values.answer || '').trim() : '',
    answers: values.type === 'multiple' ? answers : (values.type === 'single' || values.type === 'judge' || values.type === 'fill') && values.answer ? [String(values.answer).trim()] : [],
    score: Number(values.score || 10),
    status: values.status || '启用'
  };
}

export function labelOf(key: string) {
  const labels: Record<string, string> = {
    name: '名称',
    title: '标题',
    grade: '年级',
    subject: '学科',
    status: '状态',
    accountStatus: '账号状态',
    phone: '手机号',
    openedPackages: '已开通课程',
    packageName: '课程方案',
    course: '课程',
    ownerTeacherName: '负责老师',
    publishStatus: '发布状态',
    previewStatus: '预览状态',
    deadline: '截止时间',
    viewCount: '查看次数',
    action: '操作',
    target: '对象',
    time: '时间',
    academicYear: '学年',
    semester: '学期',
    phaseScope: '适用阶段',
    packageType: '套餐类型',
    openStudentNum: '开通学生数',
    chapterCount: '章节数',
    materialNum: '资料数',
    homeworkNum: '练习数',
    questionNum: '题目数',
    questionIds: '题目',
    type: '题型',
    stem: '题干',
    score: '分值',
    submittedNum: '已提交',
    totalNum: '应提交',
    studentName: '学生',
    homework: '课后练习',
    systemScore: '系统评分',
    operator: '操作人',
    summary: '内容',
    channel: '发送渠道',
    failureReason: '失败原因',
    recipientOpenId: '公众号 openid',
    retryCount: '补发次数',
    relatedType: '关联类型',
    relatedId: '关联对象',
    fileName: '文件',
    fileType: '格式',
    fileSize: '大小',
    key: '设置项',
    value: '当前值'
  };
  return labels[key] ?? key;
}

export function settingLabel(key: string) {
  const labels: Record<string, string> = {
    academicYear: '当前学年',
    grades: '年级范围',
    semesters: '学期设置',
    watermarkRule: '水印规则',
    downloadPolicy: '下载规则',
    miniProgramDomainStatus: '小程序域名备案',
    productionApiDomain: '生产接口域名',
    officialAccountBindingStatus: '微信公众号关联',
    templateMessageStatus: '模板消息审核'
  };
  return labels[key] ?? key;
}

export function richTextPlainText(value?: string) {
  const text = String(value || '')
    .replace(/<[^>]+>/g, ' ')
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/\s+/g, ' ')
    .trim();
  return text || '图片题';
}

export function statusTag(text: string) {
  const color = text.includes('失败') || text.includes('未') || text.includes('待') || text.includes('草稿') ? 'orange' : text.includes('停用') ? 'default' : 'blue';
  return <Tag color={color}>{text}</Tag>;
}

// 学科颜色统一走元数据。这里以前另有一份写死的调色板，
// 同一门课在资源页和排课页会是两种颜色——现在两边共用同一个来源。
// 圆点只需要一个主色，用 accent。
export function subjectColor(subject: string) {
  return subjectAccent(subject);
}

export function SubjectOption({ label }: { label: React.ReactNode }) {
  const text = String(label || '');
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      <span style={{ width: 8, height: 8, borderRadius: 999, background: subjectColor(text), display: 'inline-block' }} />
      {label}
    </span>
  );
}

export function CourseOption({ course }: { course: Course }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
      <span style={{ width: 8, height: 8, borderRadius: 999, background: subjectColor(course.subject), display: 'inline-block', flex: '0 0 auto' }} />
      <span>{course.name}</span>
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        {course.grade} · {course.subject}
      </Typography.Text>
    </span>
  );
}

export function courseSelectOptions(courses: Course[]) {
  return courses.map((course) => ({
    value: course.id,
    label: <CourseOption course={course} />,
    searchLabel: [course.name, course.grade, course.subject].filter(Boolean).join(' ')
  }));
}

export function questionOption(question: QuestionBankItem) {
  return {
    value: question.id,
    label: (
      <Space direction="vertical" size={0}>
        <Typography.Text>{questionTitle(question)}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {question.grade} · {semesterLabel(question.semester)} · {question.subject} · {questionTypeLabel(question.type)} · {question.score} 分
        </Typography.Text>
      </Space>
    ),
    searchLabel: questionSelectLabel(question)
  };
}

export function questionsForCourse(course: Course | undefined, questions: QuestionBankItem[], learningSpaces: LearningSpace[]) {
  if (!course) return [];
  const space = learningSpaces.find((item) => item.id === course.learningSpaceId);
  return questions.filter((question) =>
    question.status === '启用' &&
    question.grade === course.grade &&
    question.subject === course.subject &&
    (!space?.semester || question.semester === space.semester)
  );
}

export function RichTextInput({ value, onChange, placeholder }: { value?: string; onChange?: (value: string) => void; placeholder?: string }) {
  const insert = (before: string, after = '') => {
    const current = value || '';
    onChange?.(`${current}${before}${after}`);
  };
  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      <Space wrap>
        <Button size="small" onClick={() => insert('<strong>', '</strong>')}>B</Button>
        <Button size="small" onClick={() => insert('<ul><li>', '</li></ul>')}>列表</Button>
        <Button size="small" onClick={() => insert('<span style="color:#ef4444">', '</span>')}>重点</Button>
        <Button size="small" onClick={() => insert('<img src="" alt="建议配图" />')}>图片</Button>
      </Space>
      <Input.TextArea rows={4} value={value} onChange={(event) => onChange?.(event.target.value)} placeholder={placeholder} />
    </Space>
  );
}

export function isUploadKind(kind: Kind): kind is UploadKind {
  return kind === 'materials' || kind === 'homework';
}

export function canUpload(kind: UploadKind, user?: CurrentUser) {
  if (!user) return false;
  if (user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role))) return true;
  if (!user.roles.includes('teacher')) return false;
  return kind === 'materials' ? Boolean(user.canUploadHandout) : Boolean(user.canUploadQuestion);
}

export function canManagePackages(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

export function canManageCourses(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

export function hasAdminContentScope(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

export function uniqueValues(values: string[]) {
  return Array.from(new Set(values.map((value) => String(value || '').trim()).filter(Boolean)));
}

export function optionFromValues(values: string[]) {
  return values.map((value) => ({ label: value, value }));
}

export function packageTypeFromCodes(values: string[] = []) {
  const labels = [
    values.includes('course') ? '课程' : '',
    values.includes('question') ? '题' : '',
    values.includes('handout') ? '资料' : ''
  ].filter(Boolean);
  return labels.join('+') || '自定义';
}

export function contentCodesFromLabels(values: string[] = []) {
  return values
    .map((value) => {
      if (value === '课程') return 'course';
      if (value === '题') return 'question';
      if (value === '学习资料' || value === '资料' || value === '讲义') return 'handout';
      return '';
    })
    .filter(Boolean);
}

export async function openFile(url?: unknown, mode: 'preview' | 'download' = 'preview', fileName?: unknown) {
  if (!url) {
    message.warning('这个文件还不能查看');
    return;
  }
  const apiPath = String(url).replace(/^\/api/, '');
  try {
    const response = await http.get<Blob>(apiPath, { responseType: 'blob' });
    const blobUrl = window.URL.createObjectURL(response.data);
    if (mode === 'download') {
      const link = document.createElement('a');
      link.href = blobUrl;
      link.download = String(fileName || '文件');
      link.click();
      window.setTimeout(() => window.URL.revokeObjectURL(blobUrl), 60_000);
      return;
    }
    window.open(blobUrl, '_blank', 'noopener,noreferrer');
    window.setTimeout(() => window.URL.revokeObjectURL(blobUrl), 5 * 60_000);
  } catch {
    message.error(mode === 'download' ? '下载失败，请稍后重试。' : '预览打不开，请下载原文件查看。');
  }
}
