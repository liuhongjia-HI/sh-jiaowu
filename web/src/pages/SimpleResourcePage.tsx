import { Alert, Button, Card, Empty, Form, Input, InputNumber, Modal, Pagination, Select, Skeleton, Space, Table, Tag, Typography, Upload, message } from 'antd';
import type { TableColumnsType, UploadFile } from 'antd';
import { CheckCircleOutlined, DownloadOutlined, EditOutlined, EyeOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type React from 'react';
import { getData, http, postData, postForm, putData } from '../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../components/ListViews';
import { gradeOptions, subjectOptions, subjectsForGrade } from '../utils/curriculum';
import type { Course, CourseUpsertRequest, CurrentUser, Homework, HomeworkUpdateRequest, LearningSpace, Material, MaterialUpdateRequest, NoticeCreateRequest, PackageUpsertRequest, Review, ReviewCompleteRequest, SettingUpdateRequest, StudyPackage } from '../types/starline';

type Kind = 'packages' | 'content' | 'materials' | 'homework' | 'review' | 'notices' | 'logs' | 'settings';
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
};

const config: Record<Kind, { title: string; desc: string; path: string }> = {
  packages: { title: '学习套餐', desc: '维护年级、学科和开放内容。', path: '/packages' },
  content: { title: '课程内容', desc: '维护课程、章节和课节安排。', path: '/courses' },
  materials: { title: '学习资料', desc: '维护讲义、图片和课件。', path: '/materials' },
  homework: { title: '课后练习', desc: '维护练习、截止时间和发布状态。', path: '/homework' },
  review: { title: '批改反馈', desc: '处理分数、评语和学习反馈。', path: '/reviews/pending' },
  notices: { title: '通知提醒', desc: '发送练习、批改、资料和到期提醒。', path: '/notices' },
  logs: { title: '操作记录', desc: '查看开通、访问和后台操作。', path: '/logs' },
  settings: { title: '系统设置', desc: '维护学年、水印、访问和提醒规则。', path: '/settings' }
};

const emptyTextByKind: Record<Kind, string> = {
  packages: '还没有学习套餐，先创建套餐后再给学生开通。',
  content: '还没有课程内容，先维护课程和章节。',
  materials: '还没有学习资料，上传资料后学生开通套餐即可查看。',
  homework: '还没有课后练习，先为课程创建练习。',
  review: '暂时没有待批改练习。',
  notices: '还没有通知提醒。',
  logs: '还没有操作记录。',
  settings: '还没有系统设置。'
};

const settingOrder = ['academicYear', 'grades', 'semesters', 'watermarkRule', 'downloadPolicy'];

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
    homework: ['title', 'course', 'fileType', 'fileName', 'previewStatus', 'ownerTeacherName', 'deadline', 'publishStatus']
  };
  const keys = customKeys[kind] ?? Object.keys(rows[0] ?? {}).slice(0, 6);
  const columns: TableColumnsType<Record<string, unknown>> = keys.map((key) => ({
    title: labelOf(key),
    dataIndex: key,
    render: (value: unknown) => {
      if (Array.isArray(value)) return value.join('、');
      if (key === 'status' || key === 'accountStatus' || key === 'previewStatus') return statusTag(String(value || '-'));
      return String(value ?? '');
    }
  }));
  if (renderActions && (kind === 'packages' || kind === 'content' || kind === 'materials' || kind === 'homework' || kind === 'review')) {
    columns.push({ title: '操作', fixed: 'right', width: 120, render: (_: unknown, record: Record<string, unknown>) => renderActions(record) });
  }
  return columns;
}

function titleFor(row: Record<string, unknown>) {
  if (row.key && row.value !== undefined && Object.keys(row).length === 2) return settingLabel(String(row.key));
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
    materials: ['course', 'fileType', 'fileName', 'ownerTeacherName', 'publishStatus', 'viewCount'],
    homework: ['course', 'fileType', 'fileName', 'ownerTeacherName', 'deadline', 'submittedNum', 'totalNum'],
    review: ['studentName', 'packageName', 'homework', 'systemScore'],
    notices: ['type', 'target', 'summary'],
    logs: ['operator', 'action', 'target', 'time']
  };
  return (keysByKind[kind] ?? Object.keys(row).filter((key) => !['id', 'name', 'title', 'status'].includes(key)).slice(0, 6))
    .filter((key) => row[key] !== undefined && !Array.isArray(row[key]))
    .slice(0, 6)
    .map((key) => ({ label: labelOf(key), value: displayValue(row[key]) }));
}

function tagsFor(row: Record<string, unknown>) {
  const hiddenArrayKeys = new Set(['learningSpaceIds', 'contentTypeCodes']);
  const arrays = Object.entries(row).filter(([key, value]) => Array.isArray(value) && !hiddenArrayKeys.has(key)) as [string, string[]][];
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
    submittedNum: '已提交',
    totalNum: '应提交',
    studentName: '学生',
    homework: '课后练习',
    systemScore: '系统评分',
    operator: '操作人',
    summary: '内容',
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
    downloadPolicy: '下载规则'
  };
  return labels[key] ?? key;
}

function statusTag(text: string) {
  const color = text.includes('失败') || text.includes('未') || text.includes('草稿') ? 'orange' : text.includes('停用') ? 'default' : 'blue';
  return <Tag color={color}>{text}</Tag>;
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

function packageTypeFromCodes(values: string[] = []) {
  const labels = [
    values.includes('course') ? '课程' : '',
    values.includes('question') ? '题' : '',
    values.includes('handout') ? '讲义' : ''
  ].filter(Boolean);
  return labels.join('+') || '自定义';
}

function contentCodesFromLabels(values: string[] = []) {
  return values
    .map((value) => {
      if (value === '课程') return 'course';
      if (value === '题') return 'question';
      if (value === '讲义') return 'handout';
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
  const [viewMode, setViewMode] = useListViewMode(`starline:list-view:${kind}`);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [packageOpen, setPackageOpen] = useState(false);
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [editingPackage, setEditingPackage] = useState<StudyPackage | null>(null);
  const [editingCourse, setEditingCourse] = useState<Course | null>(null);
  const [editingSetting, setEditingSetting] = useState<SettingFormValues | null>(null);
  const [editingContent, setEditingContent] = useState<Material | Homework | null>(null);
  const [courseOpen, setCourseOpen] = useState(false);
  const [reviewing, setReviewing] = useState<Review | null>(null);
  const [keyword, setKeyword] = useState('');
  const [gradeFilter, setGradeFilter] = useState<string | undefined>(undefined);
  const [subjectFilter, setSubjectFilter] = useState<string | undefined>(undefined);
  const [page, setPage] = useState(1);
  const showGradeSubjectFilter = kind === 'packages' || kind === 'content';
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
  const learningSpaces = useQuery({
    queryKey: ['learning-spaces-for-resource-page', kind],
    enabled: kind === 'packages' || kind === 'content',
    queryFn: () => getData<LearningSpace[]>('/learning-spaces')
  });
  const upload = useMutation({
    mutationFn: async (values: { title: string; courseId: string; chapter?: string; deadline?: string; fileList: UploadFile[] }) => {
      if (!isUploadKind(kind)) throw new Error('当前页面不能上传文件');
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      const file = values.fileList?.[0]?.originFileObj;
      if (!course || !file) throw new Error('请选择课程和文件');
      const form = new FormData();
      form.append('title', values.title);
      form.append('courseId', course.id);
      form.append('learningSpaceId', course.learningSpaceId || '');
      if (kind === 'materials') form.append('chapter', values.chapter || '');
      if (kind === 'homework') form.append('deadline', values.deadline || '');
      form.append('file', file);
      return postForm<Material | Homework>(meta.path, form);
    },
    onSuccess: () => {
      message.success('上传成功，已保存到课程内容中。');
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
        reward: values.reward || ''
      });
    },
    onSuccess: () => {
      message.success('批改反馈已发送给学生。');
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
      message.success(editingCourse ? '课程已保存。' : '课程已创建，可继续上传讲义和题目。');
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
  const saveSetting = useMutation({
    mutationFn: async (values: SettingFormValues) => putData<Record<string, string>>('/settings', values),
    onSuccess: () => {
      message.success('系统设置已保存。');
      setEditingSetting(null);
      settingForm.resetFields();
      queryClient.invalidateQueries({ queryKey: [kind] });
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
          status: values.status || '启用'
        };
        return putData<Material>(`/materials/${editingContent.id}`, body);
      }
      const body: HomeworkUpdateRequest = {
        title: values.title,
        courseId: course.id,
        learningSpaceId: course.learningSpaceId,
        deadline: values.deadline || '',
        status: values.status || '启用'
      };
      return putData<Homework>(`/homework/${editingContent.id}`, body);
    },
    onSuccess: () => {
      message.success(kind === 'materials' ? '讲义已保存。' : '题目已保存。');
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
    return source.filter((row) => {
      if (showGradeSubjectFilter) {
        if (gradeFilter && String(row.grade ?? '') !== gradeFilter) return false;
        if (subjectFilter && String(row.subject ?? '') !== subjectFilter) return false;
      }
      if (!kw) return true;
      const haystack = [titleFor(row), subtitleFor(kind, row), ...fieldsFor(kind, row).map((field) => field.value)]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(kw);
    });
  }, [data, keyword, gradeFilter, subjectFilter, kind, showGradeSubjectFilter]);

  // 关键字或筛选变化时回到第一页，避免停留在不存在的页码。
  useEffect(() => {
    setPage(1);
  }, [keyword, gradeFilter, subjectFilter]);

  if (isLoading) return <Skeleton active />;
  if (error) return <Alert type="error" message={`${meta.title}加载失败，请稍后重试。`} />;

  const rows = data ?? [];
  const hasFilter = Boolean(keyword.trim() || gradeFilter || subjectFilter);
  const emptyText = hasFilter ? '没有符合条件的结果' : emptyTextByKind[kind];
  const pagedRows = filteredRows.slice((page - 1) * pageSize, page * pageSize);
  function openReview(record: Record<string, unknown>) {
    const review = record as Review;
    setReviewing(review);
    reviewForm.setFieldsValue({
      score: Number(review.systemScore ?? 0),
      teacherComment: review.teacherComment || '',
      reward: review.reward || ''
    });
  }

  function openCreatePackage() {
    setEditingPackage(null);
    packageForm.setFieldsValue({
      name: '',
      academicYear: '2026 学年',
      grade: undefined as unknown as string,
      subject: undefined as unknown as string,
      semester: '第一学期',
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
      summary: ''
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
      status: item.status === '已发布' ? '启用' : item.status || '启用'
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
  const renderCourseActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditCourse(record)} />
  );
  const renderSettingActions = (record: Record<string, unknown>) => (
    <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEditSetting(record)} />
  );
  const rowActions = kind === 'packages' && canManagePackages(user) ? renderPackageActions : kind === 'content' && canManageCourses(user) ? renderCourseActions : kind === 'settings' ? renderSettingActions : kind === 'review' ? renderReviewActions : isUploadKind(kind) ? renderFileActions : undefined;
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
            {kind === 'notices' && (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreateNotice}>
                发送通知
              </Button>
            )}
            {isUploadKind(kind) && canUpload(kind, user) && (
              <Button type="primary" icon={<UploadOutlined />} onClick={() => setUploadOpen(true)}>
                {kind === 'materials' ? '上传讲义' : '上传题目'}
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
            {showGradeSubjectFilter && (
              <>
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
                  placeholder="学科"
                  value={subjectFilter}
                  options={subjectOptions(gradeFilter)}
                  style={{ width: 140 }}
                  onChange={(value) => setSubjectFilter(value)}
                />
              </>
            )}
          </Space>
        </div>
        {viewMode === 'card' ? (
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
                  actions={kind === 'packages' && canManagePackages(user) ? renderPackageActions(record) : kind === 'content' && canManageCourses(user) ? renderCourseActions(record) : kind === 'settings' ? renderSettingActions(record) : kind === 'review' ? renderReviewActions(record) : isUploadKind(kind) ? renderFileActions(record) : undefined}
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
          <Table rowKey={(record) => String(record.id ?? record.key)} columns={columnsFor(kind, rows, rowActions)} dataSource={filteredRows} pagination={{ pageSize, showSizeChanger: true }} scroll={{ x: 'max-content' }} />
        )}
      </Card>
      {isUploadKind(kind) && (
        <UploadDialog
          kind={kind}
          open={uploadOpen}
          loading={upload.isPending}
          courses={courses.data ?? []}
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
          onCancel={() => setEditingContent(null)}
          onSubmit={(values) => saveContent.mutate(values)}
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
    .map((space) => ({ label: `${space.grade} · ${space.subject} · ${space.semester} · ${space.phase}`, value: space.id }));
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
          <Input placeholder="例如：五年级英语第一学期期中前阅读课程" />
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
        <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入通知标题' }]}>
          <Input placeholder="例如：英语阅读挑战已发布" />
        </Form.Item>
        <Form.Item name="target" label="接收对象" rules={[{ required: true, message: '请输入接收对象' }]}>
          <Input placeholder="例如：五年级英语班 / 小明 / 全部学生" />
        </Form.Item>
        <Form.Item name="summary" label="通知内容" rules={[{ required: true, message: '请输入通知内容' }]}>
          <Input.TextArea rows={4} placeholder="用学生和家长能看懂的话说明要做什么。" />
        </Form.Item>
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
  onCancel,
  onSubmit
}: {
  form: ReturnType<typeof Form.useForm<PackageFormValues>>[0];
  open: boolean;
  editing: boolean;
  loading: boolean;
  learningSpaces: LearningSpace[];
  onCancel: () => void;
  onSubmit: (values: PackageFormValues) => void;
}) {
  const grade = Form.useWatch('grade', form);
  const subject = Form.useWatch('subject', form);
  const semester = Form.useWatch('semester', form);
  const spaceOptions = learningSpaces
    .filter((space) => (!grade || space.grade === grade) && (!subject || space.subject === subject) && (!semester || space.semester === semester))
    .map((space) => ({ label: `${space.phase} · ${space.name}`, value: space.id }));

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
          <Input placeholder="例如：2026 学年 五年级 第一学期 英语 题+讲义" />
        </Form.Item>
        <Space size={12} align="start" wrap style={{ width: '100%' }}>
          <Form.Item name="academicYear" label="学年" rules={[{ required: true, message: '请输入学年' }]}>
            <Input style={{ width: 160 }} />
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
              options={[
                { label: '第一学期', value: '第一学期' },
                { label: '第二学期', value: '第二学期' }
              ]}
              onChange={() => form.setFieldValue('learningSpaceIds', [])}
            />
          </Form.Item>
        </Space>
        <Form.Item name="learningSpaceIds" label="开放学习空间" rules={[{ required: true, message: '请选择开放学习空间' }]}>
          <Select
            mode="multiple"
            showSearch
            optionFilterProp="label"
            placeholder="选择期中前、期末前等学习空间"
            options={spaceOptions}
          />
        </Form.Item>
        <Form.Item name="contentTypeCodes" label="开放内容" rules={[{ required: true, message: '请选择开放内容' }]}>
          <Select
            mode="multiple"
            options={[
              { label: '课程', value: 'course' },
              { label: '题', value: 'question' },
              { label: '讲义', value: 'handout' }
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
        <Form.Item name="teacherComment" label="老师评语" rules={[{ required: true, message: '请输入给学生看的评语' }]}>
          <Input.TextArea rows={4} placeholder="例如：阅读理解整体不错，注意把答案依据写完整。" />
        </Form.Item>
        <Form.Item name="reward" label="奖励">
          <Input placeholder="例如：阅读小星星" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function UploadDialog({
  kind,
  open,
  loading,
  courses,
  onCancel,
  onSubmit
}: {
  kind: UploadKind;
  open: boolean;
  loading: boolean;
  courses: Course[];
  onCancel: () => void;
  onSubmit: (values: { title: string; courseId: string; chapter?: string; deadline?: string; fileList: UploadFile[] }) => void;
}) {
  const [form] = Form.useForm();
  return (
    <Modal
      title={kind === 'materials' ? '上传讲义' : '上传题目'}
      open={open}
      okText="上传"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="title" label={kind === 'materials' ? '讲义标题' : '题目标题'} rules={[{ required: true, message: '请输入标题' }]}>
          <Input placeholder={kind === 'materials' ? '例如：五年级英语期中核心讲义' : '例如：五年级英语阅读练习题'} />
        </Form.Item>
        <Form.Item name="courseId" label="课程范围" rules={[{ required: true, message: '请选择课程范围' }]}>
          <Select
            showSearch
            placeholder="选择学生可学习的课程"
            optionFilterProp="label"
            options={courses.map((course) => ({ value: course.id, label: course.name }))}
          />
        </Form.Item>
        {kind === 'materials' ? (
          <Form.Item name="chapter" label="章节">
            <Input placeholder="不填则归为未分章节" />
          </Form.Item>
        ) : (
          <Form.Item name="deadline" label="截止时间">
            <Input placeholder="例如：2026-06-30" />
          </Form.Item>
        )}
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
  onCancel,
  onSubmit
}: {
  kind: UploadKind;
  form: ReturnType<typeof Form.useForm<ContentFormValues>>[0];
  item: Material | Homework | null;
  loading: boolean;
  courses: Course[];
  onCancel: () => void;
  onSubmit: (values: ContentFormValues) => void;
}) {
  return (
    <Modal
      title={kind === 'materials' ? '编辑讲义' : '编辑题目'}
      open={Boolean(item)}
      okText="保存"
      cancelText="取消"
      confirmLoading={loading}
      onCancel={onCancel}
      onOk={() => form.submit()}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} onFinish={onSubmit}>
        <Form.Item name="title" label={kind === 'materials' ? '讲义标题' : '题目标题'} rules={[{ required: true, message: '请输入标题' }]}>
          <Input placeholder={kind === 'materials' ? '例如：五年级英语期中核心讲义' : '例如：五年级英语阅读练习题'} />
        </Form.Item>
        <Form.Item name="courseId" label="课程范围" rules={[{ required: true, message: '请选择课程范围' }]}>
          <Select
            showSearch
            placeholder="选择学生可学习的课程"
            optionFilterProp="label"
            options={courses.map((course) => ({ value: course.id, label: course.name }))}
          />
        </Form.Item>
        {kind === 'materials' ? (
          <Form.Item name="chapter" label="章节">
            <Input placeholder="不填则归为未分章节" />
          </Form.Item>
        ) : (
          <Form.Item name="deadline" label="截止时间">
            <Input placeholder="例如：2026-06-30" />
          </Form.Item>
        )}
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
