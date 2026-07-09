import { Alert, Button, Card, Checkbox, Empty, Form, Input, InputNumber, Modal, Pagination, Radio, Select, Skeleton, Space, Table, Tag, Typography, Upload, message } from 'antd';
import type { TableColumnsType, UploadFile } from 'antd';
import { CheckCircleOutlined, DeleteOutlined, DownloadOutlined, EditOutlined, EyeOutlined, PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type React from 'react';
import { getData, http, postData, postForm, putData } from '../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../components/ListViews';
import { DEFAULT_ACADEMIC_YEAR, formatLearningSpace, phaseLabel, semesterLabel, semesterOptions, subjectOptions, gradeOptions, subjectsForGrade } from '../utils/curriculum';
import type { Course, CourseUpsertRequest, CurrentUser, Homework, HomeworkSubmissionSummary, HomeworkUpdateRequest, LearningSpace, Material, MaterialUpdateRequest, NoticeCreateRequest, PackageUpsertRequest, QuestionBankItem, QuestionBankUpsertRequest, Review, ReviewCompleteRequest, SettingUpdateRequest, StudyPackage } from '../types/starline';

type Kind = 'packages' | 'content' | 'questions' | 'materials' | 'homework' | 'review' | 'notices' | 'logs' | 'settings';
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

const config: Record<Kind, { title: string; desc: string; path: string }> = {
  packages: { title: '学习套餐', desc: '维护年级、学科和开放内容。', path: '/packages' },
  content: { title: '课程内容', desc: '维护课程、章节和课节安排。', path: '/courses' },
  questions: { title: '题库', desc: '按年级、学期和学科维护可复用题目。', path: '/questions' },
  materials: { title: '学习资料', desc: '维护学习资料、图片和课件。', path: '/materials' },
  homework: { title: '课后练习', desc: '从题库选题组卷并发布到学习空间。', path: '/homework' },
  review: { title: '批改反馈', desc: '处理分数、评语和学习反馈。', path: '/reviews/pending' },
  notices: { title: '通知提醒', desc: '发送练习、批改、资料和到期提醒。', path: '/notices' },
  logs: { title: '操作记录', desc: '查看开通、访问和后台操作。', path: '/logs' },
  settings: { title: '系统设置', desc: '维护学年、水印、访问和提醒规则。', path: '/settings' }
};

const emptyTextByKind: Record<Kind, string> = {
  packages: '还没有学习套餐，先创建套餐后再给学生开通。',
  content: '还没有课程内容，先维护课程和章节。',
  questions: '还没有题库题目，先按年级、学期和学科新增题目。',
  materials: '还没有学习资料，上传资料后学生开通套餐即可查看。',
  homework: '还没有课后练习，先从题库选题组卷。',
  review: '暂时没有待批改练习。',
  notices: '还没有通知提醒。',
  logs: '还没有操作记录。',
  settings: '还没有系统设置。'
};

const settingOrder = [
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

function columnsFor(kind: Kind, rows: Record<string, unknown>[], renderActions?: (record: Record<string, unknown>) => React.ReactNode): TableColumnsType<Record<string, unknown>> {
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

function titleFor(row: Record<string, unknown>) {
  if (row.key && row.value !== undefined && Object.keys(row).length === 2) return settingLabel(String(row.key));
  if (row.stem && !row.name && !row.title) return questionTitle(row as QuestionBankItem);
  return String(row.name ?? row.title ?? row.packageName ?? row.course ?? row.action ?? row.key ?? '未命名');
}

function subtitleFor(kind: Kind, row: Record<string, unknown>) {
  if (kind === 'settings') return '系统设置';
  const parts = [row.grade, row.subject, row.course, row.ownerTeacherName, row.target].filter(Boolean);
  return parts.map(String).join(' · ') || undefined;
}

function statusFor(row: Record<string, unknown>) {
  const value = row.previewStatus ?? row.status ?? row.publishStatus ?? row.accountStatus;
  if (!value) return undefined;
  return statusTag(String(value));
}

function fieldsFor(kind: Kind, row: Record<string, unknown>) {
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

function tagsFor(row: Record<string, unknown>) {
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

function displayValue(value: unknown) {
  if (Array.isArray(value)) return value.join('、') || '-';
  if (value === null || value === undefined || value === '') return '-';
  return String(value);
}

function displayFieldValue(key: string, value: unknown) {
  if (key === 'semester') return semesterLabel(String(value || '')) || '-';
  if (key === 'phase' || key === 'phaseScope') return phaseLabel(String(value || '')) || displayValue(value);
  return displayValue(value);
}

function questionTypeLabel(type?: string) {
  if (type === 'single') return '单选';
  if (type === 'multiple') return '多选';
  if (type === 'judge') return '判断';
  if (type === 'fill') return '填空';
  if (type === 'text') return '简答';
  return type || '-';
}

function questionAnswerOptions(options?: string[]) {
  return (options ?? [])
    .map((value, index) => ({ value: String(value || '').trim(), index }))
    .filter((item) => item.value)
    .map((item) => ({
      label: `${String.fromCharCode(65 + item.index)}. ${item.value}`,
      value: item.value
    }));
}

function questionTitle(question: Pick<QuestionBankItem, 'title' | 'stem'>) {
  const title = String(question.title || '').trim();
  if (title) return title;
  const stem = richTextPlainText(question.stem);
  return stem.length > 24 ? `${stem.slice(0, 24)}...` : stem || '未命名';
}

function questionSelectLabel(question: QuestionBankItem) {
  const stem = richTextPlainText(question.stem);
  const title = questionTitle(question);
  const stemSuffix = stem && stem !== title ? ` · ${stem}` : '';
  return `${question.grade} ${semesterLabel(question.semester)} ${question.subject} · ${questionTypeLabel(question.type)} · ${title}${stemSuffix}`;
}

function normalizeQuestionForm(values: QuestionFormValues): QuestionBankUpsertRequest {
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

function labelOf(key: string) {
  const labels: Record<string, string> = {
    name: '名称',
    title: '标题',
    grade: '年级',
    subject: '学科',
    status: '状态',
    accountStatus: '账号状态',
    phone: '手机号',
    openedPackages: '已开通套餐',
    packageName: '学习套餐',
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

function settingLabel(key: string) {
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

function richTextPlainText(value?: string) {
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

function statusTag(text: string) {
  const color = text.includes('失败') || text.includes('未') || text.includes('待') || text.includes('草稿') ? 'orange' : text.includes('停用') ? 'default' : 'blue';
  return <Tag color={color}>{text}</Tag>;
}

function subjectColor(subject: string) {
  const colors: Record<string, string> = {
    语文: '#ef4444',
    数学: '#2563eb',
    英语: '#16a34a',
    物理: '#7c3aed',
    化学: '#0891b2',
    生物: '#65a30d',
    历史: '#b45309',
    地理: '#0f766e',
    政治: '#db2777'
  };
  return colors[subject] || '#64748b';
}

function SubjectOption({ label }: { label: React.ReactNode }) {
  const text = String(label || '');
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      <span style={{ width: 8, height: 8, borderRadius: 999, background: subjectColor(text), display: 'inline-block' }} />
      {label}
    </span>
  );
}

function CourseOption({ course }: { course: Course }) {
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

function courseSelectOptions(courses: Course[]) {
  return courses.map((course) => ({
    value: course.id,
    label: <CourseOption course={course} />,
    searchLabel: [course.name, course.grade, course.subject].filter(Boolean).join(' ')
  }));
}

function questionOption(question: QuestionBankItem) {
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

function questionsForCourse(course: Course | undefined, questions: QuestionBankItem[], learningSpaces: LearningSpace[]) {
  if (!course) return [];
  const space = learningSpaces.find((item) => item.id === course.learningSpaceId);
  return questions.filter((question) =>
    question.status === '启用' &&
    question.grade === course.grade &&
    question.subject === course.subject &&
    (!space?.semester || question.semester === space.semester)
  );
}

function RichTextInput({ value, onChange, placeholder }: { value?: string; onChange?: (value: string) => void; placeholder?: string }) {
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

function isUploadKind(kind: Kind): kind is UploadKind {
  return kind === 'materials' || kind === 'homework';
}

function canUpload(kind: UploadKind, user?: CurrentUser) {
  if (!user) return false;
  if (user.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role))) return true;
  if (!user.roles.includes('teacher')) return false;
  return kind === 'materials' ? Boolean(user.canUploadHandout) : Boolean(user.canUploadQuestion);
}

function canManagePackages(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

function canManageCourses(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['teacher', 'ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

function hasAdminContentScope(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

function uniqueValues(values: string[]) {
  return Array.from(new Set(values.map((value) => String(value || '').trim()).filter(Boolean)));
}

function optionFromValues(values: string[]) {
  return values.map((value) => ({ label: value, value }));
}

function packageTypeFromCodes(values: string[] = []) {
  const labels = [
    values.includes('course') ? '课程' : '',
    values.includes('question') ? '题' : '',
    values.includes('handout') ? '资料' : ''
  ].filter(Boolean);
  return labels.join('+') || '自定义';
}

function contentCodesFromLabels(values: string[] = []) {
  return values
    .map((value) => {
      if (value === '课程') return 'course';
      if (value === '题') return 'question';
      if (value === '学习资料' || value === '资料' || value === '讲义') return 'handout';
      return '';
    })
    .filter(Boolean);
}

async function openFile(url?: unknown, mode: 'preview' | 'download' = 'preview', fileName?: unknown) {
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
      window.URL.revokeObjectURL(blobUrl);
      return;
    }
    window.open(blobUrl, '_blank', 'noopener,noreferrer');
  } catch {
    message.error(mode === 'download' ? '下载失败，请稍后重试。' : '预览打不开，请下载原文件查看。');
  }
}

export default function SimpleResourcePage({ kind, user }: { kind: Kind; user?: CurrentUser }) {
  const meta = config[kind];
  const [reviewForm] = Form.useForm<ReviewCompleteRequest>();
  const [packageForm] = Form.useForm<PackageFormValues>();
  const [noticeForm] = Form.useForm<NoticeFormValues>();
  const [courseForm] = Form.useForm<CourseFormValues>();
  const [settingForm] = Form.useForm<SettingFormValues>();
  const [contentForm] = Form.useForm<ContentFormValues>();
  const [questionForm] = Form.useForm<QuestionFormValues>();
  const [viewMode, setViewMode] = useListViewMode(`starline:list-view:${kind}`);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [packageOpen, setPackageOpen] = useState(false);
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [editingPackage, setEditingPackage] = useState<StudyPackage | null>(null);
  const [editingCourse, setEditingCourse] = useState<Course | null>(null);
  const [editingSetting, setEditingSetting] = useState<SettingFormValues | null>(null);
  const [editingContent, setEditingContent] = useState<Material | Homework | null>(null);
  const [editingQuestion, setEditingQuestion] = useState<QuestionBankItem | null>(null);
  const [courseOpen, setCourseOpen] = useState(false);
  const [questionOpen, setQuestionOpen] = useState(false);
  const [reviewing, setReviewing] = useState<Review | null>(null);
  const [submissionHomework, setSubmissionHomework] = useState<Homework | null>(null);
  const [keyword, setKeyword] = useState('');
  const [academicYearFilter, setAcademicYearFilter] = useState<string | undefined>(undefined);
  const [gradeFilter, setGradeFilter] = useState<string | undefined>(undefined);
  const [semesterFilter, setSemesterFilter] = useState<string | undefined>(undefined);
  const [subjectFilter, setSubjectFilter] = useState<string | undefined>(undefined);
  const [statusFilter, setStatusFilter] = useState<string | undefined>(undefined);
  const [page, setPage] = useState(1);
  const showLearningFilter = kind === 'packages' || kind === 'content' || kind === 'questions' || kind === 'materials';
  const pageSize = 10;
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useQuery({
    queryKey: [kind],
    queryFn: async () => {
      const raw = await getData<unknown>(meta.path);
      if (kind === 'settings' && raw && !Array.isArray(raw)) {
        return Object.entries(raw as Record<string, string>)
          .map(([key, value]) => ({ key, value }))
          .sort((a, b) => settingOrder.indexOf(a.key) - settingOrder.indexOf(b.key));
      }
      return raw as Record<string, unknown>[];
    }
  });
  const courses = useQuery({
    queryKey: ['courses-for-upload', kind],
    enabled: isUploadKind(kind),
    queryFn: () => getData<Course[]>('/courses')
  });
  const questions = useQuery({
    queryKey: ['question-bank-for-homework', kind],
    enabled: kind === 'homework',
    queryFn: () => getData<QuestionBankItem[]>('/questions')
  });
  const submissionSummary = useQuery({
    queryKey: ['homework-submissions', submissionHomework?.id],
    enabled: Boolean(submissionHomework?.id),
    queryFn: () => getData<HomeworkSubmissionSummary>(`/homework/${submissionHomework?.id}/submissions`)
  });
  const learningSpaces = useQuery({
    queryKey: ['learning-spaces-for-resource-page', kind],
    enabled: kind === 'packages' || kind === 'content' || kind === 'questions' || kind === 'materials',
    queryFn: () => getData<LearningSpace[]>('/learning-spaces')
  });
  const settings = useQuery({
    queryKey: ['settings-for-resource-page'],
    enabled: kind === 'packages' || kind === 'questions' || kind === 'materials',
    queryFn: () => getData<Record<string, string>>('/settings')
  });
  const currentAcademicYear = settings.data?.academicYear || DEFAULT_ACADEMIC_YEAR;
  const academicYearOptions = useMemo(() => {
    const packageYears = kind === 'packages'
      ? ((data ?? []) as Record<string, unknown>[]).map((item) => item.academicYear)
      : [];
    const years = [
      settings.data?.academicYear,
      ...packageYears,
      ...(learningSpaces.data ?? []).map((space) => space.academicYear),
      DEFAULT_ACADEMIC_YEAR
    ]
      .map((value) => String(value || '').trim())
      .filter(Boolean);

    return Array.from(new Set(years)).map((value) => ({ label: value, value }));
  }, [data, kind, learningSpaces.data, settings.data?.academicYear]);
  const currentSemesterOptions = semesterOptions(settings.data?.semesters);
  const questionScope = useMemo(() => {
    const unrestricted = hasAdminContentScope(user);
    if (unrestricted) {
      return {
        unrestricted,
        gradeOptions: gradeOptions(),
        semesterOptions: currentSemesterOptions,
        spaces: [] as LearningSpace[],
        hasScope: true
      };
    }
    const allowedIDs = new Set(user?.learningSpaceIds ?? []);
    const spaces = (learningSpaces.data ?? []).filter((space) => allowedIDs.has(space.id) && space.status === '启用');
    const allowedGrades = uniqueValues(spaces.map((space) => space.grade));
    const allowedSemesters = uniqueValues(spaces.map((space) => space.semester));
    return {
      unrestricted,
      gradeOptions: gradeOptions().filter((option) => allowedGrades.includes(option.value)),
      semesterOptions: currentSemesterOptions.filter((option) => allowedSemesters.includes(option.value)),
      spaces,
      hasScope: spaces.length > 0
    };
  }, [currentSemesterOptions, learningSpaces.data, user]);
  const upload = useMutation({
    mutationFn: async (values: { title: string; courseId: string; chapter?: string; deadline?: string; questionIds?: string[]; fileList?: UploadFile[] }) => {
      if (!isUploadKind(kind)) throw new Error('当前页面不能上传文件');
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      const file = values.fileList?.[0]?.originFileObj;
      if (!course) throw new Error('请选择课程');
      if (kind === 'homework') {
        return postData<Homework>('/homework', {
          title: values.title,
          courseId: course.id,
          learningSpaceId: course.learningSpaceId || '',
          deadline: values.deadline || '',
          status: '启用',
          questionIds: (values as ContentFormValues).questionIds ?? []
        });
      }
      if (!file) throw new Error('请选择文件');
      const form = new FormData();
      form.append('title', values.title);
      form.append('courseId', course.id);
      form.append('learningSpaceId', course.learningSpaceId || '');
      if (kind === 'materials') form.append('chapter', values.chapter || '');
      form.append('file', file);
      return postForm<Material | Homework>(meta.path, form);
    },
    onSuccess: () => {
      message.success(kind === 'homework' ? '小挑战已组卷发布。' : '上传成功，已保存到课程内容中。');
      setUploadOpen(false);
      queryClient.invalidateQueries({ queryKey: [kind] });
    },
    onError: (err: Error) => {
      message.error(err.message || '上传失败，请稍后重试。');
    }
  });
  const completeReview = useMutation({
    mutationFn: async (values: ReviewCompleteRequest) => {
      if (!reviewing) throw new Error('请选择要批改的记录');
      return postData(`/reviews/${reviewing.id}/complete`, {
        score: Number(values.score),
        teacherComment: values.teacherComment,
        reward: values.reward || '',
        finalStatus: values.finalStatus || '已批改'
      });
    },
    onSuccess: () => {
      message.success('批改反馈已保存并同步给学生。');
      setReviewing(null);
      reviewForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '保存批改失败，请稍后重试。');
    }
  });
  const savePackage = useMutation({
    mutationFn: async (values: PackageFormValues) => {
      const body: PackageUpsertRequest = {
        ...values,
        summary: values.summary || '',
        phaseScope: values.phaseScope || '全学期',
        packageType: values.packageType || packageTypeFromCodes(values.contentTypeCodes),
        status: values.status || '启用'
      };
      if (editingPackage) return putData<StudyPackage>(`/packages/${editingPackage.id}`, body);
      return postData<StudyPackage>('/packages', body);
    },
    onSuccess: () => {
      message.success(editingPackage ? '学习套餐已保存。' : '学习套餐已创建，可给学生开通。');
      setPackageOpen(false);
      setEditingPackage(null);
      packageForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '保存套餐失败，请检查学习空间和内容类型。');
    }
  });
  const sendNotice = useMutation({
    mutationFn: async (values: NoticeFormValues) => postData('/notices', values),
    onSuccess: () => {
      message.success('通知已发送。');
      setNoticeOpen(false);
      noticeForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '发送通知失败，请检查接收对象。');
    }
  });
  const retryNotice = useMutation({
    mutationFn: async (record: Record<string, unknown>) => postData(`/notices/${record.id}/retry`, {}),
    onSuccess: () => {
      message.success('通知已补发。');
      queryClient.invalidateQueries({ queryKey: [kind] });
    },
    onError: (err: Error) => {
      message.error(err.message || '补发失败，请检查通知配置。');
    }
  });
  const saveCourse = useMutation({
    mutationFn: async (values: CourseFormValues) => {
      const body: CourseUpsertRequest = {
        ...values,
        chapterCount: Number(values.chapterCount || 0),
        status: values.status || '启用'
      };
      if (editingCourse) return putData<Course>(`/courses/${editingCourse.id}`, body);
      return postData<Course>('/courses', body);
    },
    onSuccess: () => {
      message.success(editingCourse ? '课程已保存。' : '课程已创建，可继续上传学习资料和题目。');
      setCourseOpen(false);
      setEditingCourse(null);
      courseForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['courses-for-upload'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '保存课程失败，请检查课程范围。');
    }
  });
  const saveQuestion = useMutation({
    mutationFn: async (values: QuestionFormValues) => {
      const body: QuestionBankUpsertRequest = normalizeQuestionForm(values);
      if (editingQuestion) return putData<QuestionBankItem>(`/questions/${editingQuestion.id}`, body);
      return postData<QuestionBankItem>('/questions', body);
    },
    onSuccess: () => {
      message.success(editingQuestion ? '题目已保存。' : '题目已加入题库。');
      setQuestionOpen(false);
      setEditingQuestion(null);
      questionForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['question-bank-for-homework'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '保存题目失败，请检查题干、选项和答案。');
    }
  });
  const saveSetting = useMutation({
    mutationFn: async (values: SettingFormValues) => putData<Record<string, string>>('/settings', values),
    onSuccess: () => {
      message.success('系统设置已保存。');
      setEditingSetting(null);
      settingForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['settings-for-resource-page'] });
      queryClient.invalidateQueries({ queryKey: ['logs'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '保存设置失败，请检查设置值。');
    }
  });
  const saveContent = useMutation({
    mutationFn: async (values: ContentFormValues) => {
      if (!isUploadKind(kind) || !editingContent) throw new Error('请选择要维护的内容');
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      if (!course) throw new Error('请选择课程范围');
      if (kind === 'materials') {
        const body: MaterialUpdateRequest = {
      title: values.title,
      courseId: course.id,
      learningSpaceId: course.learningSpaceId,
      chapter: values.chapter || '',
      status: values.status || '已发布'
        };
        return putData<Material>(`/materials/${editingContent.id}`, body);
      }
      const body: HomeworkUpdateRequest = {
        title: values.title,
        courseId: course.id,
        learningSpaceId: course.learningSpaceId,
        deadline: values.deadline || '',
        status: values.status || '启用',
        questionIds: values.questionIds ?? []
      };
      return putData<Homework>(`/homework/${editingContent.id}`, body);
    },
    onSuccess: () => {
      message.success(kind === 'materials' ? '学习资料已保存。' : '题目已保存。');
      setEditingContent(null);
      contentForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
      queryClient.invalidateQueries({ queryKey: ['permissions'] });
    },
    onError: (err: Error) => {
      message.error(err.message || '保存失败，请检查课程范围和发布状态。');
    }
  });

  const filteredRows = useMemo(() => {
    const source = (data ?? []) as Record<string, unknown>[];
    const kw = keyword.trim().toLowerCase();
    const spaceByID = new Map((learningSpaces.data ?? []).map((space) => [space.id, space]));
    const courseByID = new Map((courses.data ?? []).map((course) => [course.id, course]));
    const learningMeta = (row: Record<string, unknown>) => {
      const course = row.courseId ? courseByID.get(String(row.courseId)) : undefined;
      const space = (row.learningSpaceId ? spaceByID.get(String(row.learningSpaceId)) : undefined) || (course?.learningSpaceId ? spaceByID.get(course.learningSpaceId) : undefined);
      return {
        academicYear: String(row.academicYear ?? space?.academicYear ?? ''),
        grade: String(row.grade ?? course?.grade ?? space?.grade ?? ''),
        semester: String(row.semester ?? space?.semester ?? ''),
        subject: String(row.subject ?? course?.subject ?? space?.subject ?? ''),
        status: String(row.publishStatus ?? row.status ?? '')
      };
    };
    return source.filter((row) => {
      if (showLearningFilter) {
        const meta = learningMeta(row);
        if (academicYearFilter && meta.academicYear !== academicYearFilter) return false;
        if (gradeFilter && meta.grade !== gradeFilter) return false;
        if (semesterFilter && meta.semester !== semesterFilter) return false;
        if (subjectFilter && meta.subject !== subjectFilter) return false;
        if (statusFilter && meta.status !== statusFilter) return false;
      }
      if (!kw) return true;
      const haystack = [titleFor(row), subtitleFor(kind, row), ...fieldsFor(kind, row).map((field) => field.value)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(kw);
    });
  }, [data, keyword, learningSpaces.data, courses.data, academicYearFilter, gradeFilter, semesterFilter, subjectFilter, statusFilter, kind, showLearningFilter]);

  // 关键字或筛选变化时回到第一页，避免停留在不存在的页码。
  useEffect(() => {
    setPage(1);
  }, [keyword, academicYearFilter, gradeFilter, semesterFilter, subjectFilter, statusFilter]);

  if (isLoading) return <Skeleton active />;
  if (error) return <Alert type="error" message={`${meta.title}加载失败，请稍后重试。`} />;

  const rows = data ?? [];
  const hasFilter = Boolean(keyword.trim() || academicYearFilter || gradeFilter || semesterFilter || subjectFilter || statusFilter);
  const emptyText = hasFilter ? '没有符合条件的结果' : emptyTextByKind[kind];
  const pagedRows = filteredRows.slice((page - 1) * pageSize, page * pageSize);
  function openReview(record: Record<string, unknown>) {
    const review = record as Review;
    setReviewing(review);
    reviewForm.setFieldsValue({
      score: Number(review.systemScore ?? 0),
      teacherComment: review.teacherComment || '',
      reward: review.reward || '',
      finalStatus: '已批改'
    });
  }

  function openCreatePackage() {
    setEditingPackage(null);
    packageForm.setFieldsValue({
      name: '',
      academicYear: currentAcademicYear,
      grade: undefined as unknown as string,
      subject: undefined as unknown as string,
      semester: currentSemesterOptions[0]?.value || 'S1',
      phaseScope: '全学期',
      packageType: '',
      summary: '',
      learningSpaceIds: [],
      contentTypeCodes: ['question', 'handout'],
      status: '启用'
    });
    setPackageOpen(true);
  }

  function openCreateNotice() {
    noticeForm.setFieldsValue({
      type: '通知',
      title: '',
      target: '',
      summary: '',
      channel: '站内通知',
      recipientOpenId: '',
      relatedType: '',
      relatedId: ''
    });
    setNoticeOpen(true);
  }

  function openCreateCourse() {
    setEditingCourse(null);
    courseForm.setFieldsValue({
      name: '',
      learningSpaceId: '',
      chapterCount: 8,
      status: '启用'
    });
    setCourseOpen(true);
  }

  function openCreateQuestion() {
    setEditingQuestion(null);
    const defaultGrade = questionScope.gradeOptions.length === 1 ? questionScope.gradeOptions[0].value : undefined;
    const defaultSemester = questionScope.semesterOptions.length === 1 ? questionScope.semesterOptions[0].value : currentSemesterOptions[0]?.value || 'S1';
    const scopedSubjects = questionScope.unrestricted
      ? subjectOptions(defaultGrade)
      : optionFromValues(uniqueValues(questionScope.spaces
        .filter((space) => (!defaultGrade || space.grade === defaultGrade) && (!defaultSemester || space.semester === defaultSemester))
        .map((space) => space.subject)));
    const defaultSubject = scopedSubjects.length === 1 ? scopedSubjects[0].value : undefined;
    questionForm.setFieldsValue({
      title: '',
      grade: defaultGrade as string,
      semester: defaultSemester,
      subject: defaultSubject as string,
      type: 'single',
      stem: '',
      options: [''],
      answer: '',
      answers: [],
      score: 10,
      status: '启用'
    });
    setQuestionOpen(true);
  }

  function openEditQuestion(record: Record<string, unknown>) {
    const item = record as QuestionBankItem;
    setEditingQuestion(item);
    questionForm.setFieldsValue({
      title: item.title || questionTitle(item),
      grade: item.grade,
      semester: item.semester,
      subject: item.subject,
      type: item.type,
      stem: item.stem,
      options: item.options ?? [],
      answer: item.answer || (item.answers ?? [])[0] || '',
      answers: item.answers ?? [],
      score: item.score || 10,
      status: item.status || '启用'
    });
    setQuestionOpen(true);
  }

  function openEditCourse(record: Record<string, unknown>) {
    const course = record as Course;
    setEditingCourse(course);
    courseForm.setFieldsValue({
      name: course.name,
      learningSpaceId: course.learningSpaceId || '',
      chapterCount: course.chapterCount,
      status: course.status
    });
    setCourseOpen(true);
  }

  function openEditPackage(record: Record<string, unknown>) {
    const item = record as StudyPackage;
    setEditingPackage(item);
    packageForm.setFieldsValue({
      name: item.name,
      academicYear: item.academicYear,
      grade: item.grade,
      subject: item.subject,
      semester: item.semester,
      phaseScope: item.phaseScope,
      packageType: item.packageType,
      summary: item.summary,
      learningSpaceIds: item.learningSpaceIds ?? [],
      contentTypeCodes: item.contentTypeCodes ?? contentCodesFromLabels(item.contentTypes ?? []),
      status: item.status
    });
    setPackageOpen(true);
  }

  function openEditSetting(record: Record<string, unknown>) {
    const setting = { key: String(record.key || ''), value: String(record.value || '') };
    setEditingSetting(setting);
    settingForm.setFieldsValue(setting);
  }

  function openEditContent(record: Record<string, unknown>) {
    const item = record as Material | Homework;
    setEditingContent(item);
    contentForm.setFieldsValue({
      title: item.title,
      courseId: item.courseId || '',
      chapter: 'chapter' in item ? item.chapter : '',
      deadline: 'deadline' in item ? item.deadline : '',
      status: kind === 'materials'
        ? (item.publishStatus === '停用' || item.status === '停用' ? '停用' : '已发布')
        : item.status === '已发布' ? '启用' : item.status || '启用',
      questionIds: 'questionIds' in item ? item.questionIds ?? [] : []
    });
  }

  const renderFileActions = (record: Record<string, unknown>) => (
    <Space wrap>
      {isUploadKind(kind) && canUpload(kind, user) && (
        <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditContent(record)} />
      )}
      <ActionButton tooltip="预览" icon={<EyeOutlined />} disabled={record.previewStatus !== '可预览'} onClick={() => openFile(record.previewUrl, 'preview', record.fileName)} />
      <ActionButton tooltip="下载" icon={<DownloadOutlined />} onClick={() => openFile(record.downloadUrl, 'download', record.fileName)} />
    </Space>
  );
  const renderReviewActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="填写反馈" icon={<CheckCircleOutlined />} onClick={() => openReview(record)} />
  );
  const renderPackageActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditPackage(record)} />
  );
  const renderQuestionActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditQuestion(record)} />
  );
  const renderCourseActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditCourse(record)} />
  );
  const renderSettingActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditSetting(record)} />
  );
  const renderNoticeActions = (record: Record<string, unknown>) => {
    if (record.channel !== '公众号模板消息') return <Typography.Text type="secondary">-</Typography.Text>;
    return <ActionButton tooltip="补发" icon={<ReloadOutlined />} loading={retryNotice.isPending} onClick={() => retryNotice.mutate(record)} />;
  };
  const rowActions = kind === 'packages' && canManagePackages(user) ? renderPackageActions : kind === 'content' && canManageCourses(user) ? renderCourseActions : kind === 'questions' ? renderQuestionActions : kind === 'settings' ? renderSettingActions : kind === 'review' ? renderReviewActions : kind === 'notices' ? renderNoticeActions : isUploadKind(kind) ? renderFileActions : undefined;
  const tableColumns = columnsFor(kind, rows, rowActions).map((column) => {
    if (kind !== 'homework' || !('dataIndex' in column)) return column;
    if (column.dataIndex !== 'submittedNum' && column.dataIndex !== 'totalNum') return column;
    return {
      ...column,
      render: (value: unknown, record: Record<string, unknown>) => (
        <Button type="link" size="small" onClick={() => setSubmissionHomework(record as Homework)}>
          {displayValue(value)}
        </Button>
      )
    };
  });
  return (
    <div className="page-stack">
      <div>
        <div className="page-heading">
          <div>
            <Typography.Title level={3}>{meta.title}</Typography.Title>
            <Typography.Text type="secondary">{meta.desc}</Typography.Text>
          </div>
          <div className="page-heading-actions">
            {kind === 'packages' && canManagePackages(user) && (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreatePackage}>
                新增套餐
              </Button>
            )}
            {kind === 'content' && canManageCourses(user) && (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateCourse}>
                新增课程
              </Button>
            )}
            {kind === 'questions' && canUpload('homework', user) && (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateQuestion}>
                新增题目
              </Button>
            )}
            {kind === 'notices' && (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateNotice}>
                发送通知
              </Button>
            )}
            {isUploadKind(kind) && canUpload(kind, user) && (
              <Button type="primary" icon={kind === 'materials' ? <UploadOutlined /> : <PlusOutlined />} onClick={() => setUploadOpen(true)}>
                {kind === 'materials' ? '上传资料' : '手动组卷'}
              </Button>
            )}
            <ListViewToggle storageKey={`starline:list-view:${kind}`} value={viewMode} onChange={setViewMode} />
          </div>
        </div>
      </div>
      {isUploadKind(kind) && !canUpload(kind, user) && (
        <Alert type="info" showIcon message="当前账号没有上传权限，请联系管理员开通。" />
      )}
      <Card>
        <div className="list-toolbar" style={{ marginBottom: 16 }}>
          <Space wrap>
            <Input.Search
              placeholder={`搜索${meta.title}`}
              allowClear
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              style={{ width: 240 }}
            />
            {showLearningFilter && (
              <>
                {(kind === 'packages' || kind === 'materials') && (
                  <Select
                    allowClear
                    placeholder="学年"
                    value={academicYearFilter}
                    options={academicYearOptions}
                    style={{ width: 160 }}
                    onChange={(value) => setAcademicYearFilter(value)}
                  />
                )}
                <Select
                  allowClear
                  placeholder="年级"
                  value={gradeFilter}
                  options={gradeOptions()}
                  style={{ width: 140 }}
                  onChange={(value) => {
                    setGradeFilter(value);
                    if (value && subjectFilter && !subjectsForGrade(value).includes(subjectFilter)) {
                      setSubjectFilter(undefined);
                    }
                  }}
                />
                <Select
                  allowClear
                  placeholder="学期"
                  value={semesterFilter}
                  options={currentSemesterOptions}
                  style={{ width: 140 }}
                  onChange={(value) => setSemesterFilter(value)}
                />
                <Select
                  allowClear
                  placeholder="学科"
                  value={subjectFilter}
                  options={subjectOptions(gradeFilter).map((option) => ({ ...option, label: <SubjectOption label={option.label} /> }))}
                  style={{ width: 140 }}
                  onChange={(value) => setSubjectFilter(value)}
                />
                {(kind === 'materials' || kind === 'questions') && (
                  <Select
                    allowClear
                    placeholder="状态"
                    value={statusFilter}
                    options={kind === 'materials'
                      ? [{ label: '已发布', value: '已发布' }, { label: '停用', value: '停用' }]
                      : [{ label: '启用', value: '启用' }, { label: '草稿', value: '草稿' }, { label: '停用', value: '停用' }]}
                    style={{ width: 120 }}
                    onChange={(value) => setStatusFilter(value)}
                  />
                )}
              </>
            )}
          </Space>
        </div>
        {kind === 'review' ? (
          <ReviewBoard rows={filteredRows as Review[]} onOpen={openReview} />
        ) : viewMode === 'card' ? (
          <>
            <CardList
              rows={pagedRows}
              rowKey={(record) => String(record.id ?? record.key ?? titleFor(record))}
              emptyText={emptyText}
              renderCard={(record) => (
                <InfoCard
                  title={titleFor(record)}
                  subtitle={subtitleFor(kind, record)}
                  status={statusFor(record)}
                  fields={fieldsFor(kind, record)}
                  tags={tagsFor(record)}
                  actions={kind === 'packages' && canManagePackages(user) ? renderPackageActions(record) : kind === 'content' && canManageCourses(user) ? renderCourseActions(record) : kind === 'questions' ? renderQuestionActions(record) : kind === 'settings' ? renderSettingActions(record) : kind === 'notices' ? renderNoticeActions(record) : isUploadKind(kind) ? renderFileActions(record) : undefined}
                />
              )}
            />
            {filteredRows.length > pageSize && (
              <div style={{ marginTop: 16, textAlign: 'right' }}>
                <Pagination current={page} pageSize={pageSize} total={filteredRows.length} onChange={setPage} showSizeChanger={false} />
              </div>
            )}
          </>
        ) : filteredRows.length === 0 ? (
          <Empty description={emptyText} />
        ) : (
          <Table rowKey={(record) => String(record.id ?? record.key)} columns={tableColumns} dataSource={filteredRows} pagination={{ pageSize, showSizeChanger: true }} scroll={{ x: 'max-content' }} />
        )}
      </Card>
      <HomeworkSubmissionDialog
        homework={submissionHomework}
        summary={submissionSummary.data}
        loading={submissionSummary.isLoading}
        error={Boolean(submissionSummary.error)}
        onCancel={() => setSubmissionHomework(null)}
      />
      {isUploadKind(kind) && (
        <UploadDialog
          kind={kind}
          open={uploadOpen}
          loading={upload.isPending}
          courses={courses.data ?? []}
          questions={questions.data ?? []}
          learningSpaces={learningSpaces.data ?? []}
          onCancel={() => setUploadOpen(false)}
          onSubmit={(values) => upload.mutate(values)}
        />
      )}
      {isUploadKind(kind) && (
        <ContentEditDialog
          kind={kind}
          form={contentForm}
          item={editingContent}
          loading={saveContent.isPending}
          courses={courses.data ?? []}
          questions={questions.data ?? []}
          learningSpaces={learningSpaces.data ?? []}
          onCancel={() => setEditingContent(null)}
          onSubmit={(values) => saveContent.mutate(values)}
        />
      )}
      {kind === 'questions' && (
        <QuestionDialog
          form={questionForm}
          open={questionOpen}
          editing={Boolean(editingQuestion)}
          loading={saveQuestion.isPending}
          scopeLoading={!questionScope.unrestricted && learningSpaces.isLoading}
          scopeError={Boolean(!questionScope.unrestricted && learningSpaces.error)}
          gradeOptions={questionScope.gradeOptions}
          semesterOptions={questionScope.semesterOptions}
          allowedSpaces={questionScope.spaces}
          unrestricted={questionScope.unrestricted}
          hasScope={questionScope.hasScope}
          onCancel={() => setQuestionOpen(false)}
          onSubmit={(values) => saveQuestion.mutate(values)}
        />
      )}
      {kind === 'review' && (
        <ReviewDialog
          form={reviewForm}
          review={reviewing}
          loading={completeReview.isPending}
          onCancel={() => setReviewing(null)}
          onSubmit={(values) => completeReview.mutate(values)}
        />
      )}
      {kind === 'packages' && (
        <PackageDialog
          form={packageForm}
          open={packageOpen}
          editing={Boolean(editingPackage)}
          loading={savePackage.isPending}
          learningSpaces={learningSpaces.data ?? []}
          academicYearOptions={academicYearOptions}
          semesterOptions={currentSemesterOptions}
          onCancel={() => setPackageOpen(false)}
          onSubmit={(values) => savePackage.mutate(values)}
        />
      )}
      {kind === 'notices' && (
        <NoticeDialog
          form={noticeForm}
          open={noticeOpen}
          loading={sendNotice.isPending}
          onCancel={() => setNoticeOpen(false)}
          onSubmit={(values) => sendNotice.mutate(values)}
        />
      )}
      {kind === 'content' && (
        <CourseDialog
          form={courseForm}
          open={courseOpen}
          editing={Boolean(editingCourse)}
          loading={saveCourse.isPending}
          learningSpaces={learningSpaces.data ?? []}
          allowedLearningSpaceIds={user?.learningSpaceIds ?? []}
          unrestricted={Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)))}
          onCancel={() => setCourseOpen(false)}
          onSubmit={(values) => saveCourse.mutate(values)}
        />
      )}
      {kind === 'settings' && (
        <SettingDialog
          form={settingForm}
          setting={editingSetting}
          loading={saveSetting.isPending}
          onCancel={() => setEditingSetting(null)}
          onSubmit={(values) => saveSetting.mutate(values)}
        />
      )}
    </div>
  );
}

function HomeworkSubmissionDialog({
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

function ReviewBoard({ rows, onOpen }: { rows: Review[]; onOpen: (record: Record<string, unknown>) => void }) {
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

function SettingDialog({
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

function CourseDialog({
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
  const spaceOptions = learningSpaces
    .filter((space) => unrestricted || allowedLearningSpaceIds.includes(space.id))
    .map((space) => ({ label: formatLearningSpace(space), value: space.id }));
  const hasSpaceOptions = spaceOptions.length > 0;

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
      {!hasSpaceOptions && (
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
        <Form.Item name="learningSpaceId" label="课程范围" rules={[{ required: true, message: '请选择课程范围' }]}>
          <Select
            showSearch
            optionFilterProp="label"
            placeholder="选择年级、学科、学期和阶段"
            disabled={!hasSpaceOptions}
            notFoundContent="暂无可维护课程范围"
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

function NoticeDialog({
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
              { label: '套餐到期', value: '权' },
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

function PackageDialog({
  form,
  open,
  editing,
  loading,
  learningSpaces,
  academicYearOptions,
  semesterOptions,
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
  onCancel: () => void;
  onSubmit: (values: PackageFormValues) => void;
}) {
  const grade = Form.useWatch('grade', form);
  const subject = Form.useWatch('subject', form);
  const semester = Form.useWatch('semester', form);
  const spaceOptions = learningSpaces
    .filter((space) => (!grade || space.grade === grade) && (!subject || space.subject === subject) && (!semester || space.semester === semester))
    .map((space) => ({ label: `${phaseLabel(space.phase)} · ${formatLearningSpace(space)}`, value: space.id }));

  return (
    <Modal
      title={editing ? '编辑学习套餐' : '新增学习套餐'}
      open={open}
      okText="保存"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
      width={720}
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="name" label="套餐名称" rules={[{ required: true, message: '请输入套餐名称' }]}>
          <Input placeholder="例如：2025.2026学年 五年级 S1 英语 题+资料" />
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
          <Form.Item name="packageType" label="套餐类型">
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
          <Input.TextArea rows={3} placeholder="说明这个套餐适合什么学生、包含哪些学习内容。" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function ReviewDialog({
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

function QuestionDialog({
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

function UploadDialog({
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
        <Form.Item name="title" label={kind === 'materials' ? '资料标题' : '练习标题'} rules={[{ required: true, message: '请输入标题' }]}>
          <Input placeholder={kind === 'materials' ? '例如：五年级英语 S1 Q1 核心资料' : '例如：五年级英语 S1 Q1 阅读练习'} />
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
              <Select
                mode="multiple"
                showSearch
                optionFilterProp="searchLabel"
                placeholder={selectedCourse ? '从可用题库中选择题目' : '请先选择课程范围'}
                disabled={!selectedCourse}
                options={availableQuestions.map(questionOption)}
                notFoundContent={selectedCourse ? '没有匹配课程范围的启用题目，请先到题库出题。' : '请先选择课程范围'}
              />
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
              getValueFromEvent={(event) => event?.fileList ?? []}
              rules={[{ required: true, message: '请选择文件' }]}
            >
              <Upload beforeUpload={() => false} maxCount={1} accept=".pdf,.ppt,.pptx,.doc,.docx">
                <Button icon={<UploadOutlined />}>选择文件</Button>
              </Upload>
            </Form.Item>
            <Typography.Text type="secondary">支持 PDF、PPT、Word，单个文件不超过 50MB。</Typography.Text>
          </>
        )}
      </Form>
    </Modal>
  );
}

function ContentEditDialog({
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
              <Select
                mode="multiple"
                showSearch
                optionFilterProp="searchLabel"
                placeholder={selectedCourse ? '从可用题库中选择题目' : '请先选择课程范围'}
                disabled={!selectedCourse}
                options={availableQuestions.map(questionOption)}
                notFoundContent={selectedCourse ? '没有匹配课程范围的启用题目，请先到题库出题。' : '请先选择课程范围'}
              />
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
