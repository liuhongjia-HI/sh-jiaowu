import { Alert, Button, Card, Checkbox, Form, Input, Modal, Popconfirm, Select, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import { DeleteOutlined, DownloadOutlined, EditOutlined, EyeOutlined, HolderOutlined, PlusOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { deleteData, getData, http, postData, postForm, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { ContentEditDialog, CourseDialog, type CourseFormValues, HomeworkSubmissionDialog, UploadDialog, homeworkTagOptions, materialTagOptions } from './ResourceDialogs';
import { canUpload, suggestMaterialTagCode } from './resource-shared';
import { MaterialUploadOverview } from './MaterialUploadOverview';
import { curriculumLessonOptions, formatResourceCurriculumLabel, prepareCurriculumForSave, subjectLabel, suggestEquivalentCurriculumLessonId } from '../../utils/curriculum';
import type { Course, CourseUpsertRequest, CurrentUser, Homework, HomeworkSubmissionSummary, LearningSpace, Material, MaterialReorderRequest, MaterialSyncPreview, MaterialSyncRequest, MaterialSyncResult, QuestionBankItem, StudyPackage } from '../../types/starline';
import type { UploadFile } from 'antd';

type ResourceKind = 'materials' | 'homework';
type UploadValues = { title: string; courseId: string; lessonId: string; tagCode?: string; allowDownload?: boolean; deadline?: string; deadlineAt?: string; assessmentType?: 'practice' | 'mock_exam'; questionIds?: string[]; fileList?: UploadFile[] };
type ContentValues = Omit<UploadValues, 'fileList'> & { status: string };
type MaterialUploadFailure = { fileName: string; reason: string; file: File; title: string; tagCode: string; allowDownload: boolean };
type MaterialUploadResult = { sourceCourseId: string; sourceLessonId: string; uploaded: Material[]; added: number; failures: MaterialUploadFailure[] };

function materialTitleFromFile(fileName: string) {
  return fileName.replace(/\.[^.]+$/, '').trim() || fileName;
}

const MATERIAL_TAG_ORDER = ['HD', 'Blank', 'HW', 'Exam', 'Special'];

type MaterialPackRow = {
  key: string;
  courseId: string;
  course: string;
  lessonId: string;
  lessonLabel: string;
  subject?: string;
  ownerTeacherName?: string;
  createdAt?: string;
  materials: Material[];
};

function materialTagRank(tag?: string) {
  const index = MATERIAL_TAG_ORDER.indexOf(tag || '');
  return index < 0 ? MATERIAL_TAG_ORDER.length : index;
}

function packKey(item: Pick<Material, 'id' | 'courseId' | 'lessonId'>) {
  return item.lessonId ? `${item.courseId || ''}::${item.lessonId}` : item.id;
}

function groupMaterialsByLesson(rows: Material[], courses: Course[]): MaterialPackRow[] {
  const packs = new Map<string, MaterialPackRow>();
  const order: string[] = [];
  for (const item of rows) {
    const key = packKey(item);
    const existing = packs.get(key);
    if (existing) {
      existing.materials.push(item);
      if (item.createdAt && (!existing.createdAt || item.createdAt > existing.createdAt)) existing.createdAt = item.createdAt;
      continue;
    }
    order.push(key);
    packs.set(key, {
      key,
      courseId: item.courseId || '',
      course: item.course,
      lessonId: item.lessonId || '',
      lessonLabel: formatResourceCurriculumLabel(item, courses.find((course) => course.id === item.courseId)),
      subject: item.subject,
      ownerTeacherName: item.ownerTeacherName,
      createdAt: item.createdAt,
      materials: [item]
    });
  }
  return order.map((key) => {
    const pack = packs.get(key)!;
    pack.materials.sort((left, right) => materialTagRank(left.tagCode) - materialTagRank(right.tagCode) || (left.sortOrder || 0) - (right.sortOrder || 0));
    return pack;
  });
}

function movePack(courseMaterials: Material[], sourceKey: string, targetKey: string) {
  if (sourceKey === targetKey) return courseMaterials;
  const sourceItems = courseMaterials.filter((item) => packKey(item) === sourceKey);
  if (sourceItems.length === 0) return courseMaterials;
  const without = courseMaterials.filter((item) => packKey(item) !== sourceKey);
  const targetIndex = without.findIndex((item) => packKey(item) === targetKey);
  if (targetIndex < 0) return courseMaterials;
  without.splice(targetIndex, 0, ...sourceItems);
  return without;
}

function packPreviewStatus(materials: Material[]) {
  if (materials.some((item) => item.previewStatus === '转换失败')) return '转换失败';
  if (materials.some((item) => item.previewStatus === '待转换' || item.previewStatus === '转换中')) return '待转换';
  return materials.find((item) => item.previewStatus)?.previewStatus || materials[0]?.publishStatus || '';
}

function deadlineAtValue(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toISOString();
}

function moveMaterial(items: Material[], sourceID: string, targetID: string) {
  const sourceIndex = items.findIndex((item) => item.id === sourceID);
  const targetIndex = items.findIndex((item) => item.id === targetID);
  if (sourceIndex < 0 || targetIndex < 0 || sourceIndex === targetIndex) return items;
  const next = [...items];
  const [source] = next.splice(sourceIndex, 1);
  next.splice(targetIndex, 0, source);
  return next;
}

export function ContentResourcesPage({ kind, user, courseId, packageId, onClearFilter }: { kind: ResourceKind; user?: CurrentUser; courseId?: string; packageId?: string; onClearFilter?: () => void }) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Material | Homework | null>(null);
  const [submissionHomework, setSubmissionHomework] = useState<Homework | null>(null);
  const [contentForm] = Form.useForm<ContentValues>();
  const [courseForm] = Form.useForm<CourseFormValues>();
  const [courseEditor, setCourseEditor] = useState<Course | null>(null);
  const client = useQueryClient();
  const title = kind === 'materials' ? '课程讲义' : '课后练习';
  const path = kind === 'materials' ? '/materials' : '/homework';
  const [keyword, setKeyword] = useState('');
  const [subject, setSubject] = useState<string>();
  const [grade, setGrade] = useState<string>();
  const [tagCode, setTagCode] = useState<string>();
  const [assessmentType, setAssessmentType] = useState<string>();
  const [homeworkCourseId, setHomeworkCourseId] = useState<string>();
  const [uploaderId, setUploaderId] = useState<string>();
  const [uploadedFrom, setUploadedFrom] = useState('');
  const [uploadedTo, setUploadedTo] = useState('');
  const [draggingPackKey, setDraggingPackKey] = useState('');
  const [draggingHomeworkId, setDraggingHomeworkId] = useState('');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [syncBatch, setSyncBatch] = useState<MaterialUploadResult | null>(null);
  const [syncCourseIds, setSyncCourseIds] = useState<string[]>([]);
  const [syncLessonIds, setSyncLessonIds] = useState<Record<string, string>>({});
  const [syncPreview, setSyncPreview] = useState<MaterialSyncPreview | null>(null);
  const [uploadTarget, setUploadTarget] = useState<{ courseId: string; lessonId: string } | null>(null);
  const materialParams = Object.fromEntries(Object.entries({ keyword, subject, tagCode, uploaderId, uploadedFrom, uploadedTo }).filter(([, value]) => Boolean(value))) as Record<string, string>;
  const resources = useQuery({ queryKey: [kind, materialParams], queryFn: () => getData<(Material | Homework)[]>(path, kind === 'materials' ? materialParams : undefined) });
  const allMaterials = useQuery({ queryKey: ['materials', 'all-for-reorder'], enabled: kind === 'materials', queryFn: () => getData<Material[]>('/materials') });
  const courses = useQuery({ queryKey: ['courses-for-content-resources'], queryFn: () => getData<Course[]>('/courses') });
  const packages = useQuery({ queryKey: ['packages-for-content-resources'], enabled: Boolean(packageId), queryFn: () => getData<StudyPackage[]>('/packages') });
  const questions = useQuery({ queryKey: ['question-bank-for-homework'], enabled: kind === 'homework', queryFn: () => getData<QuestionBankItem[]>('/questions') });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces-for-content-resources'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
  const submissionSummary = useQuery({
    queryKey: ['homework-submissions', submissionHomework?.id],
    enabled: Boolean(submissionHomework?.id),
    queryFn: () => getData<HomeworkSubmissionSummary>(`/homework/${submissionHomework?.id}/submissions`)
  });
  const canManage = canUpload(kind, user);
  const canManageCourse = Boolean(user?.roles.some((role) => (['ops_staff', 'campus_admin', 'super_admin'].includes(role) || role === 'teacher' && user?.teacherLibrary?.canManageCourses !== false)));
  const unrestrictedCourseScope = Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
  const create = useMutation({
    mutationFn: async (values: UploadValues) => {
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      if (!course) throw new Error('请选择课程');
	  if (kind === 'homework') return postData<Homework>('/homework', { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId || '', lessonId: values.lessonId, tagCode: values.tagCode || '', deadlineAt: deadlineAtValue(values.deadlineAt), assessmentType: values.assessmentType || 'practice', status: '启用', questionIds: values.questionIds ?? [], allowDownload: Boolean(values.allowDownload) });
      const files = (values.fileList ?? []).map((item) => item.originFileObj).filter(Boolean) as File[];
      if (files.length === 0) throw new Error('请选择文件');
      const lessonLabel = formatResourceCurriculumLabel({ lessonId: values.lessonId }, course);
      const uploaded: Material[] = [];
      const failures: MaterialUploadFailure[] = [];
      let added = 0;
      for (const file of files) {
        const tagCode = values.tagCode || suggestMaterialTagCode(file.name);
        const data = new FormData();
        const uploadTitle = values.title?.trim() || lessonLabel || materialTitleFromFile(file.name);
        data.append('title', uploadTitle);
        data.append('courseId', course.id);
        data.append('learningSpaceId', course.learningSpaceId || '');
		data.append('lessonId', values.lessonId);
        data.append('tagCode', tagCode);
		data.append('allowDownload', values.allowDownload ? 'true' : 'false');
        data.append('file', file);
        try {
          uploaded.push(await postForm<Material>('/materials', data));
          added += 1;
        } catch (error) {
          failures.push({ fileName: file.name, reason: error instanceof Error ? error.message : '上传失败', file, title: uploadTitle, tagCode, allowDownload: Boolean(values.allowDownload) });
        }
      }
      if (!uploaded.length) throw new Error(failures[0]?.reason || '上传失败，请稍后重试。');
      return { sourceCourseId: course.id, sourceLessonId: values.lessonId, uploaded, added, failures };
    },
    onSuccess: (result) => {
      if (kind === 'materials' && result && typeof result === 'object' && 'added' in result) {
        if (result.failures.length) message.warning(`本课资料已新增 ${result.added} 个；${result.failures.length} 个文件失败，可重新上传。`);
        else message.success(`本课资料已新增 ${result.added} 个。`);
        setSyncBatch(result);
        setSyncCourseIds([]);
        setSyncLessonIds({});
        setSyncPreview(null);
      } else {
        message.success('课后练习已发布。');
      }
      setOpen(false);
      setUploadTarget(null);
      client.invalidateQueries({ queryKey: [kind] });
      client.invalidateQueries({ queryKey: ['materials-overview'] });
      if (kind === 'materials') client.invalidateQueries({ queryKey: ['materials', 'all-for-reorder'] });
    },
    onError: (error: Error) => message.error(error.message || '保存失败，请稍后重试。')
  });
  const sourceSyncCourse = syncBatch ? (courses.data ?? []).find((item) => item.id === syncBatch.sourceCourseId) : undefined;
  const sourceSyncSpace = sourceSyncCourse ? (learningSpaces.data ?? []).find((item) => item.id === sourceSyncCourse.learningSpaceId) : undefined;
  const syncCandidates = sourceSyncCourse && sourceSyncSpace ? (courses.data ?? []).filter((course) => {
    if (course.id === sourceSyncCourse.id || course.grade !== sourceSyncCourse.grade || course.subject.trim().toLowerCase() !== sourceSyncCourse.subject.trim().toLowerCase()) return false;
    const space = (learningSpaces.data ?? []).find((item) => item.id === course.learningSpaceId);
    return Boolean(space && space.semester === sourceSyncSpace.semester && space.phase === sourceSyncSpace.phase);
  }) : [];
  const materialSyncRequest = (): MaterialSyncRequest => ({
    sourceCourseId: syncBatch?.sourceCourseId || '',
    sourceLessonId: syncBatch?.sourceLessonId || '',
    materialIds: syncBatch?.uploaded.map((item) => item.id) || [],
    targets: syncCourseIds.map((courseId) => ({ courseId, lessonId: syncLessonIds[courseId] || '' }))
  });
  const previewSync = useMutation({
    mutationFn: () => postData<MaterialSyncPreview>('/materials/sync-preview', materialSyncRequest()),
    onSuccess: setSyncPreview,
    onError: (error: Error) => message.error(error.message || '同步预检查失败，请检查目标课节。')
  });
  const retryFailedUploads = useMutation({
    mutationFn: async () => {
      if (!syncBatch) throw new Error('没有需要重试的文件');
      const succeeded: Material[] = [];
      const failures: MaterialUploadFailure[] = [];
      for (const item of syncBatch.failures) {
        const data = new FormData();
        data.append('title', item.title);
        data.append('courseId', syncBatch.sourceCourseId);
        data.append('learningSpaceId', sourceSyncCourse?.learningSpaceId || '');
        data.append('lessonId', syncBatch.sourceLessonId);
        data.append('tagCode', item.tagCode);
        data.append('allowDownload', item.allowDownload ? 'true' : 'false');
        data.append('file', item.file);
        try {
          succeeded.push(await postForm<Material>('/materials', data));
        } catch (error) {
          failures.push({ ...item, reason: error instanceof Error ? error.message : '上传失败' });
        }
      }
      return { succeeded, failures };
    },
    onSuccess: ({ succeeded, failures }) => {
      setSyncBatch((current) => {
        if (!current) return current;
        return { ...current, uploaded: [...current.uploaded, ...succeeded], added: current.added + succeeded.length, failures };
      });
      setSyncPreview(null);
      if (failures.length) message.warning(`${succeeded.length} 个文件重试成功，仍有 ${failures.length} 个失败。`);
      else message.success('失败文件已全部重新上传，可继续同步。');
      client.invalidateQueries({ queryKey: ['materials'] });
      client.invalidateQueries({ queryKey: ['materials-overview'] });
    },
    onError: (error: Error) => message.error(error.message || '重新上传失败，请稍后重试。')
  });
  const executeSync = useMutation({
    mutationFn: () => postData<MaterialSyncResult>('/materials/sync', { ...materialSyncRequest(), snapshot: syncPreview?.snapshot || '' }),
    onSuccess: (result) => {
      const created = result.targets.reduce((total, item) => total + item.created, 0);
      message.success(result.alreadySynced ? '这些资料已经同步完成。' : `已同步到 ${result.targets.length} 门课程：新增 ${created} 个。`);
      setSyncBatch(null);
      setSyncPreview(null);
      client.invalidateQueries({ queryKey: ['materials'] });
      client.invalidateQueries({ queryKey: ['materials-overview'] });
      client.invalidateQueries({ queryKey: ['courses'] });
      client.invalidateQueries({ queryKey: ['content'] });
    },
    onError: (error: Error) => message.error(`源课程已上传，目标同步未完成：${error.message || '请稍后重试。'}`)
  });
  const closeSync = () => {
    setSyncBatch(null);
    setSyncCourseIds([]);
    setSyncLessonIds({});
    setSyncPreview(null);
  };
  const changeSyncCourses = (values: string[]) => {
    setSyncCourseIds(values);
    setSyncPreview(null);
    setSyncLessonIds((current) => {
      const next: Record<string, string> = {};
      for (const courseId of values) {
        if (current[courseId]) {
          next[courseId] = current[courseId];
          continue;
        }
        const target = syncCandidates.find((course) => course.id === courseId);
        const suggestion = sourceSyncCourse && target ? suggestEquivalentCurriculumLessonId(sourceSyncCourse.curriculum, syncBatch?.sourceLessonId || '', target.curriculum) : undefined;
        if (suggestion) next[courseId] = suggestion;
      }
      return next;
    });
  };
  const save = useMutation({
    mutationFn: async (values: ContentValues) => {
      if (!editing) throw new Error('请选择要维护的内容');
      const course = (courses.data ?? []).find((item) => item.id === values.courseId);
      if (!course) throw new Error('请选择课程范围');
	  if (kind === 'materials') return putData<Material>(`/materials/${editing.id}`, { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId, lessonId: values.lessonId, tagCode: values.tagCode || '', status: values.status || '已发布', allowDownload: Boolean(values.allowDownload) });
	  return putData<Homework>(`/homework/${editing.id}`, { title: values.title, courseId: course.id, learningSpaceId: course.learningSpaceId, lessonId: values.lessonId, tagCode: values.tagCode || '', deadlineAt: deadlineAtValue(values.deadlineAt), assessmentType: values.assessmentType || 'practice', status: values.status || '启用', questionIds: values.questionIds ?? [], allowDownload: Boolean(values.allowDownload) });
    },
    onSuccess: () => {
      message.success(kind === 'materials' ? '课程讲义已保存。' : '课后练习已保存。');
      setEditing(null);
      contentForm.resetFields();
      client.invalidateQueries({ queryKey: [kind] });
      client.invalidateQueries({ queryKey: ['permissions'] });
      if (kind === 'materials') client.invalidateQueries({ queryKey: ['materials-overview'] });
    },
    onError: (error: Error) => message.error(error.message || '保存失败，请检查课程范围和发布状态。')
  });
  const saveCourse = useMutation({
    mutationFn: (values: CourseFormValues) => {
      if (!courseEditor) throw new Error('请选择要维护的课程');
      const { grade: _grade, subject: _subject, curriculum = [], ...courseValues } = values;
      const body: CourseUpsertRequest = {
        ...courseValues,
        curriculum: prepareCurriculumForSave(curriculum),
        status: values.status || '启用'
      };
      return putData<Course>(`/courses/${courseEditor.id}`, body);
    },
    onSuccess: () => {
      message.success('课程目录已同步，当前上传可直接选择新课节。');
      setCourseEditor(null);
      courseForm.resetFields();
      client.invalidateQueries({ queryKey: ['courses-for-content-resources'] });
      client.invalidateQueries({ queryKey: ['courses'] });
      client.invalidateQueries({ queryKey: ['content'] });
      client.invalidateQueries({ queryKey: ['materials-overview'] });
    },
    onError: (error: Error) => message.error(error.message || '保存课程目录失败，请检查层级关系。')
  });
  const removeContent = useMutation({
    mutationFn: (id: string) => deleteData(`${path}/${id}`),
    onSuccess: () => {
      message.success(kind === 'materials' ? '课程讲义已删除。' : '课后练习已删除。');
      client.invalidateQueries({ queryKey: [kind] });
		if (kind === 'materials') client.invalidateQueries({ queryKey: ['materials', 'all-for-reorder'] });
      if (kind === 'materials') client.invalidateQueries({ queryKey: ['materials-overview'] });
    },
    onError: (error: Error) => message.error(error.message || '删除失败，请稍后重试。')
  });
  const retryPreview = useMutation({
    mutationFn: (fileId: string) => postData(`/files/${fileId}/preview/retry`, {}),
    onSuccess: () => {
      message.success('已重新提交课件预览生成。');
      client.invalidateQueries({ queryKey: [kind] });
    },
    onError: (error: Error) => message.error(error.message || '重新生成失败，请稍后重试。')
  });
  const reorderMaterials = useMutation({
    mutationFn: (values: MaterialReorderRequest) => postData('/materials/reorder', values),
    onSuccess: () => {
      message.success('讲义展示顺序已保存，小程序会同步更新。');
      client.invalidateQueries({ queryKey: ['materials'] });
      client.invalidateQueries({ queryKey: ['materials-overview'] });
    },
    onError: (error: Error) => message.error(error.message || '排序保存失败，请刷新后重试。')
  });
  const reorderHomework = useMutation({
    mutationFn: (values: { courseId: string; homeworkIds: string[] }) => postData('/homework/reorder', values),
    onSuccess: () => { message.success('练习展示顺序已保存，小程序会同步更新。'); client.invalidateQueries({ queryKey: ['homework'] }); },
    onError: (error: Error) => message.error(error.message || '排序保存失败，请刷新后重试。')
  });
  const removeSelected = useMutation({
    mutationFn: async (ids: string[]) => { for (const id of ids) await deleteData(`${path}/${id}`); },
    onSuccess: (_data, ids) => { message.success(`已删除 ${ids.length} 项内容。`); setSelectedRowKeys([]); client.invalidateQueries({ queryKey: [kind] }); if (kind === 'materials') client.invalidateQueries({ queryKey: ['materials-overview'] }); },
    onError: (error: Error) => message.error(error.message || '批量删除失败，请稍后重试。')
  });
  const openFile = async (url: unknown, download = false, name?: unknown) => {
    if (!url) return message.warning('这个文件还不能查看');
    try {
      const response = await http.get<Blob>(String(url).replace(/^\/api/, ''), { responseType: 'blob' });
      const blob = URL.createObjectURL(response.data);
      if (download) {
        const link = document.createElement('a');
        link.href = blob;
        link.download = String(name || '文件');
        link.click();
        window.setTimeout(() => URL.revokeObjectURL(blob), 60_000);
      } else {
        window.open(blob, '_blank', 'noopener,noreferrer');
        window.setTimeout(() => URL.revokeObjectURL(blob), 5 * 60_000);
      }
    } catch (error) {
      const reason = error instanceof Error ? error.message : '';
      message.error(reason || (download ? '下载失败，请稍后重试。' : '预览打不开，请下载原文件查看。'));
    }
  };
  const openEdit = (item: Material | Homework) => {
    setEditing(item);
    contentForm.setFieldsValue({
      title: item.title,
      courseId: item.courseId || '',
	  lessonId: item.lessonId || '',
      tagCode: item.tagCode || '',
	  allowDownload: 'allowDownload' in item ? Boolean(item.allowDownload) : false,
      deadline: 'deadline' in item ? item.deadline : '',
		deadlineAt: 'deadlineAt' in item ? item.deadlineAt?.slice(0, 16) : '',
		assessmentType: 'assessmentType' in item ? (item.assessmentType || 'practice') : 'practice',
      status: kind === 'materials' ? (item.publishStatus === '停用' || item.status === '停用' ? '停用' : '已发布') : item.status === '已发布' ? '启用' : item.status || '启用',
      questionIds: 'questionIds' in item ? item.questionIds ?? [] : []
    });
  };
  const selectedCourse = (courses.data ?? []).find((item) => item.id === courseId);
  const uploadCourse = (courses.data ?? []).find((item) => item.id === uploadTarget?.courseId) ?? selectedCourse;
  const selectedPackage = (packages.data ?? []).find((item) => item.id === packageId);
  const packageIncludesMaterials = Boolean(selectedPackage?.contentTypeCodes?.includes('handout'));
  const packageSpaceIds = new Set(selectedPackage?.learningSpaceIds ?? []);
  const packageCourseIds = new Set((courses.data ?? []).filter((item) => packageSpaceIds.has(item.learningSpaceId || '')).map((item) => item.id));
  const tableRows = ((resources.data ?? []) as Array<Material | Homework>).filter((item) => {
    const itemCourse = (courses.data ?? []).find((course) => course.id === item.courseId);
    if (grade && itemCourse?.grade !== grade) return false;
    if (kind === 'homework') {
      if (keyword.trim() && !item.title.toLowerCase().includes(keyword.trim().toLowerCase())) return false;
      if (tagCode && item.tagCode !== tagCode) return false;
      if (assessmentType && (item as Homework).assessmentType !== assessmentType) return false;
      if (homeworkCourseId && item.courseId !== homeworkCourseId) return false;
    }
    if (courseId) return item.courseId === courseId || (selectedCourse && item.course === selectedCourse.name);
    if (packageId) return Boolean(packageIncludesMaterials && (packageSpaceIds.has(item.learningSpaceId || '') || packageCourseIds.has(item.courseId || '')));
    return true;
  });
  const filterDescription = selectedCourse
    ? `正在查看“${selectedCourse.name}”的全部课程讲义。`
    : selectedPackage
      ? `正在查看“${selectedPackage.name}”套餐包含的全部课程讲义。`
      : packageId && !packages.isLoading
        ? '未找到对应套餐，请返回查看全部课程讲义。'
        : '按课节维护 HD / Blank / HW / Exam / Special，一次上传一套资料。';
  const loading = resources.isLoading || courses.isLoading || (Boolean(packageId) && packages.isLoading);
  const loadError = resources.error || courses.error || (Boolean(packageId) && packages.error);

  const materialPacks = kind === 'materials' ? groupMaterialsByLesson(tableRows as Material[], courses.data ?? []) : [];
  const selectedPacks = materialPacks.filter((pack) => selectedRowKeys.includes(pack.key));
  const selectedDeleteIds = kind === 'materials' ? selectedPacks.flatMap((pack) => pack.materials.map((item) => item.id)) : selectedRowKeys.map(String);

  const handlePackDrop = (target: MaterialPackRow) => {
    if (!allMaterials.data) {
      setDraggingPackKey('');
      message.info('资料正在加载，请稍后再试。');
      return;
    }
    const sourceKey = draggingPackKey;
    setDraggingPackKey('');
    if (!sourceKey || sourceKey === target.key) return;
    const sourceItems = (allMaterials.data ?? []).filter((item) => packKey(item) === sourceKey);
    const targetItems = (allMaterials.data ?? []).filter((item) => packKey(item) === target.key);
    if (!sourceItems.length || !targetItems.length) return;
    if (sourceItems[0].courseId !== target.courseId) {
      message.warning('讲义只能在同一课程内调整顺序。');
      return;
    }
    const courseMaterials = (allMaterials.data ?? []).filter((item) => item.courseId === target.courseId);
    const reordered = movePack(courseMaterials, sourceKey, target.key);
    if (reordered === courseMaterials) return;
    reorderMaterials.mutate({ courseId: target.courseId || '', materialIds: reordered.map((item) => item.id) });
  };

  const handleHomeworkDrop = (target: Homework) => {
    const source = (resources.data ?? []).find((item) => item.id === draggingHomeworkId) as Homework | undefined;
    setDraggingHomeworkId('');
    if (!source || source.id === target.id || source.courseId !== target.courseId) return;
    const items = (resources.data ?? []).filter((item): item is Homework => 'assessmentType' in item && item.courseId === target.courseId);
    const reordered = moveMaterial(items as unknown as Material[], source.id, target.id);
    reorderHomework.mutate({ courseId: target.courseId || '', homeworkIds: reordered.map((item) => item.id) });
  };

  const subjectOptions = Array.from(new Set((resources.data ?? []).map((row) => row.subject).filter(Boolean))).map((value) => ({ label: subjectLabel(value), value }));
  const gradeOptions = Array.from(new Set((courses.data ?? []).map((course) => course.grade).filter(Boolean))).map((value) => ({ label: value, value }));
  const tagOptions = kind === 'materials' ? materialTagOptions : homeworkTagOptions;
  const courseOptions = (courses.data ?? []).map((course) => ({ label: course.name, value: course.id }));
  const tagQuickFilters = <div className="content-filter-tags" style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', width: '100%', paddingTop: 4 }}><Typography.Text type="secondary">主标签</Typography.Text><Button size="small" type={!tagCode ? 'primary' : 'default'} onClick={() => setTagCode(undefined)}>全部</Button>{tagOptions.map((option) => <Button key={option.value} size="small" type={tagCode === option.value ? 'primary' : 'default'} onClick={() => setTagCode(option.value)}>{option.label}</Button>)}</div>;
  const uploaderOptions = Array.from(new Map((resources.data ?? []).filter((row): row is Material => 'ownerTeacherId' in row && Boolean(row.ownerTeacherId)).map((row) => [row.ownerTeacherId as string, { label: row.ownerTeacherName || row.ownerTeacherId as string, value: row.ownerTeacherId as string }])).values());
  return <div className="page-stack">
    <div className="page-heading">
      <div>
        <Typography.Title level={3}>{title}</Typography.Title>
        <Typography.Text type="secondary">{kind === 'materials' ? filterDescription : '从题库选题并发布到学习空间。'}</Typography.Text>
      </div>
      {canManage && <Button type="primary" icon={kind === 'materials' ? <UploadOutlined /> : <PlusOutlined />} onClick={() => { setUploadTarget(null); setOpen(true); }}>{kind === 'materials' ? '上传讲义' : '新建课后练习'}</Button>}
    </div>
    {kind === 'materials' && !courseId && !packageId && <MaterialUploadOverview courses={courses.data ?? []} canManage={canManage} onUpload={(targetCourseId, lessonId) => { setUploadTarget({ courseId: targetCourseId, lessonId }); setOpen(true); }} onOpenFile={(item, download) => openFile(download ? item.downloadUrl : item.previewUrl, download, item.fileName)} />}
    {kind === 'materials' && <Card><div><Space wrap><Input.Search allowClear placeholder="搜索课节或文件名" value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 220 }} /><Select allowClear placeholder="年级" value={grade} onChange={setGrade} options={gradeOptions} style={{ width: 130 }} /><Select allowClear placeholder="学科" value={subject} onChange={setSubject} options={subjectOptions} style={{ width: 130 }} /><Select allowClear showSearch placeholder="上传人" value={uploaderId} onChange={setUploaderId} options={uploaderOptions} style={{ width: 150 }} /><Input type="date" value={uploadedFrom} onChange={(event) => setUploadedFrom(event.target.value)} /><Input type="date" value={uploadedTo} onChange={(event) => setUploadedTo(event.target.value)} /><Button onClick={() => { setKeyword(''); setGrade(undefined); setSubject(undefined); setTagCode(undefined); setUploaderId(undefined); setUploadedFrom(''); setUploadedTo(''); }}>重置</Button></Space>{tagQuickFilters}</div></Card>}
    {kind === 'homework' && <Card><div><Space wrap><Input.Search allowClear placeholder="搜索练习标题" value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 220 }} /><Select allowClear placeholder="年级" value={grade} onChange={setGrade} options={gradeOptions} style={{ width: 130 }} /><Select allowClear showSearch optionFilterProp="label" placeholder="课程" value={homeworkCourseId} onChange={setHomeworkCourseId} options={courseOptions} style={{ width: 220 }} /><Select allowClear placeholder="练习类型" value={assessmentType} onChange={setAssessmentType} options={[{ label: '常规练习', value: 'practice' }, { label: '模拟考试', value: 'mock_exam' }]} style={{ width: 150 }} /><Button onClick={() => { setKeyword(''); setGrade(undefined); setTagCode(undefined); setAssessmentType(undefined); setHomeworkCourseId(undefined); }}>重置</Button></Space>{tagQuickFilters}</div></Card>}
    {canManage && selectedDeleteIds.length > 0 && <div style={{ marginBottom: 12 }}><Popconfirm title={`确定删除选中的 ${selectedDeleteIds.length} 项内容吗？`} description="删除后学生将无法再查看这些内容。" okText="删除" cancelText="取消" okButtonProps={{ danger: true, loading: removeSelected.isPending }} onConfirm={() => removeSelected.mutate(selectedDeleteIds)}><Button danger icon={<DeleteOutlined />}>批量删除（{selectedDeleteIds.length}）</Button></Popconfirm></div>}

    {loading ? <Skeleton active /> : loadError ? <Alert type="error" message={`${title}加载失败，请稍后重试。`} /> : <Card extra={<Space><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => resources.refetch()} />{kind === 'materials' && canManage && <Typography.Text type="secondary">同一课节的五类讲义显示为一套；拖动可调整课节顺序</Typography.Text>}{(courseId || packageId) && onClearFilter && <Button type="link" onClick={onClearFilter}>查看全部讲义</Button>}</Space>}>
      {kind === 'materials' ? (
        <Table<MaterialPackRow>
          rowKey="key"
          rowSelection={canManage ? { selectedRowKeys, onChange: (keys) => setSelectedRowKeys(keys) } : undefined}
          dataSource={materialPacks}
          pagination={{ pageSize: 10 }}
          onRow={(row) => {
            if (!canManage) return {};
            return {
              onDragOver: (event) => event.preventDefault(),
              onDrop: (event) => { event.preventDefault(); handlePackDrop(row); },
              onDragEnd: () => setDraggingPackKey('')
            };
          }}
          columns={[
            ...(canManage ? [{ title: '排序', width: 64, render: (_: unknown, row: MaterialPackRow) => <button type="button" className="material-sort-handle" title="拖动调整同一课程内课节顺序" draggable onDragStart={(event) => { setDraggingPackKey(row.key); event.dataTransfer.effectAllowed = 'move'; }}><HolderOutlined /></button> }] : []),
            { title: '课节', render: (_: unknown, row: MaterialPackRow) => <div><strong>{row.lessonLabel}</strong><div className="lesson-pack-versions">{row.materials.map((item) => <Tag key={item.id}>{item.tagCode || '未标签'}</Tag>)}</div></div> },
            { title: '课程', dataIndex: 'course' },
            { title: '学科', dataIndex: 'subject', render: (value: string) => subjectLabel(value) },
            { title: '上传人', dataIndex: 'ownerTeacherName' },
            { title: '最近上传', dataIndex: 'createdAt' },
            { title: '状态', render: (_: unknown, row: MaterialPackRow) => packPreviewStatus(row.materials) },
            { title: '操作', render: (_: unknown, row: MaterialPackRow) => (
              <div className="lesson-pack-actions">
                {row.materials.map((item) => {
                  const deleteAction = canManage ? <Popconfirm
                    title={`确定删除“${item.tagCode || item.title}”吗？`}
                    description="删除后，学生将无法再查看这份资料。"
                    okText="删除"
                    okButtonProps={{ danger: true, loading: removeContent.isPending }}
                    cancelText="取消"
                    onConfirm={() => removeContent.mutate(item.id)}
                  ><ActionButton tooltip={`删除${item.tagCode || '资料'}`} danger icon={<DeleteOutlined />} /></Popconfirm> : null;
                  return (
                    <Space key={item.id} size={4} wrap>
                      <Tag>{item.tagCode || '未标签'}</Tag>
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>{item.fileName || item.title}</Typography.Text>
                      <ActionButton tooltip={`预览${item.tagCode || ''}`} icon={<EyeOutlined />} disabled={item.previewStatus !== '可预览'} onClick={() => openFile(item.previewUrl, false, item.fileName)} />
                      {canManage && <ActionButton tooltip={`编辑${item.tagCode || ''}`} icon={<EditOutlined />} onClick={() => openEdit(item)} />}
                      {deleteAction}
                      <ActionButton tooltip={`下载${item.tagCode || ''}`} icon={<DownloadOutlined />} onClick={() => openFile(item.downloadUrl, true, item.fileName)} />
                    </Space>
                  );
                })}
              </div>
            ) }
          ]}
        />
      ) : (
      <Table<Material | Homework>
        rowKey="id"
        rowSelection={canManage ? { selectedRowKeys, onChange: (keys) => setSelectedRowKeys(keys) } : undefined}
        dataSource={tableRows}
        pagination={{ pageSize: 10 }}
        onRow={(row) => {
          if (!canManage) return {};
          return {
            onDragOver: (event) => event.preventDefault(),
            onDrop: (event) => { event.preventDefault(); handleHomeworkDrop(row as Homework); },
            onDragEnd: () => setDraggingHomeworkId('')
          };
        }}
        columns={[
          ...(canManage ? [{ title: '排序', width: 64, render: (_: unknown, row: Material | Homework) => <button type="button" className="material-sort-handle" title="拖动调整同一课程内顺序" draggable onDragStart={(event) => { setDraggingHomeworkId((row as Homework).id); event.dataTransfer.effectAllowed = 'move'; }}><HolderOutlined /></button> }] : []),
          { title: '标题', dataIndex: 'title' },
		  { title: '目录', render: (_: unknown, row: Material | Homework) => formatResourceCurriculumLabel(row, (courses.data ?? []).find((course) => course.id === row.courseId)) },
		  { title: '类型', render: (_: unknown, row: Material | Homework) => (row as Homework).assessmentType === 'mock_exam' ? '模拟考试' : '常规练习' },
          { title: '截止时间', render: (_: unknown, row: Material | Homework) => (row as Homework).deadlineAt ? new Date((row as Homework).deadlineAt as string).toLocaleString() : '不设截止' },
          { title: '课程', dataIndex: 'course' },
          { title: '状态', render: (_: unknown, row: Material | Homework) => { const status = row.status; return <div><div>{status}</div>{row.previewError && <Typography.Text type={status === '转换失败' ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>{row.previewError}</Typography.Text>}</div>; } },
          { title: '操作', render: (_: unknown, row: Material | Homework) => {
            const deleteAction = canManage ? <Popconfirm
              title={`确定删除“${row.title}”吗？`}
              description="删除后，学生将无法再查看这份练习；已有学生提交记录的练习不能删除，请改为停用。"
              okText="删除"
              okButtonProps={{ danger: true, loading: removeContent.isPending }}
              cancelText="取消"
              onConfirm={() => removeContent.mutate(row.id)}
            ><ActionButton tooltip="删除" danger icon={<DeleteOutlined />} /></Popconfirm> : null;
            return <Space><ActionButton tooltip="查看提交详情" icon={<EyeOutlined />} onClick={() => setSubmissionHomework(row as Homework)} />{canManage && <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => openEdit(row)} />}{deleteAction}</Space>;
          } }
        ]}
      />
      )}
    </Card>}
    <HomeworkSubmissionDialog homework={submissionHomework} summary={submissionSummary.data} loading={submissionSummary.isLoading} error={Boolean(submissionSummary.error)} onCancel={() => setSubmissionHomework(null)} />
    {open && <UploadDialog kind={kind} open loading={create.isPending} courses={courses.data ?? []} questions={questions.data ?? []} learningSpaces={learningSpaces.data ?? []} materials={(kind === 'materials' ? (resources.data ?? []) : []) as Material[]} initialCourse={uploadCourse} initialLessonId={uploadTarget?.lessonId} onManageCurriculum={canManageCourse ? (course) => { setCourseEditor(course); courseForm.setFieldsValue({ ...course, grade: course.grade, subject: course.subject, curriculum: course.curriculum ?? [] }); } : undefined} onCancel={() => { setOpen(false); setUploadTarget(null); }} onSubmit={(values) => create.mutate(values)} />}
    <ContentEditDialog kind={kind} form={contentForm} item={editing} loading={save.isPending} courses={courses.data ?? []} questions={questions.data ?? []} learningSpaces={learningSpaces.data ?? []} onCancel={() => setEditing(null)} onSubmit={(values) => save.mutate(values)} />
    <CourseDialog form={courseForm} open={Boolean(courseEditor)} editing loading={saveCourse.isPending} learningSpaces={learningSpaces.data ?? []} allowedLearningSpaceIds={user?.learningSpaceIds ?? []} unrestricted={unrestrictedCourseScope} onCancel={() => { setCourseEditor(null); courseForm.resetFields(); }} onSubmit={(values) => saveCourse.mutate(values)} />
    <Modal
      title="同步本次课程讲义"
      open={Boolean(syncBatch)}
      width={760}
      onCancel={closeSync}
      footer={[
        <Button key="cancel" onClick={closeSync}>暂不同步</Button>,
        syncPreview
          ? <Button key="sync" type="primary" loading={executeSync.isPending} onClick={() => executeSync.mutate()}>确认同步到 {syncCourseIds.length} 门课程</Button>
          : <Button key="preview" type="primary" loading={previewSync.isPending} disabled={!syncCourseIds.length || syncCourseIds.some((id) => !syncLessonIds[id])} onClick={() => previewSync.mutate()}>检查同步内容</Button>
      ]}
    >
      <Alert type="info" showIcon message={`源课程已上传 ${syncBatch?.uploaded.length || 0} 份资料，可选择同步到同年级、学科、学期和阶段的课程。`} description="同步后各课程资料独立维护；即使目标课节已有同标签资料，本次文件也会独立新增。" style={{ marginBottom: 16 }} />
      {syncBatch?.failures.length ? <Alert type="warning" showIcon message={`${syncBatch.failures.length} 个文件上传失败，不会参与同步`} description={<div><div>{syncBatch.failures.map((item) => `${item.fileName}：${item.reason}`).join('；')}</div><Button size="small" style={{ marginTop: 8 }} loading={retryFailedUploads.isPending} onClick={() => retryFailedUploads.mutate()}>只重新上传失败文件</Button></div>} style={{ marginBottom: 16 }} /> : null}
      <Typography.Text strong>本次资料</Typography.Text>
      <div style={{ margin: '8px 0 16px' }}>{syncBatch?.uploaded.map((item) => <Tag key={item.id}>{item.tagCode || '未标签'} · {item.fileName}</Tag>)}</div>
      <Typography.Text strong>目标课程</Typography.Text>
      {syncCandidates.length ? <Checkbox.Group value={syncCourseIds} onChange={(values) => changeSyncCourses(values.map(String))} style={{ display: 'grid', gap: 12, marginTop: 10 }}>
        {syncCandidates.map((course) => <div key={course.id} className="material-sync-target-row">
          <Checkbox value={course.id}>{course.name} <Tag>{course.status}</Tag></Checkbox>
          <Select showSearch optionFilterProp="label" placeholder="选择对应课节" disabled={!syncCourseIds.includes(course.id)} value={syncLessonIds[course.id]} options={curriculumLessonOptions(course.curriculum)} onChange={(lessonId) => { setSyncLessonIds((current) => ({ ...current, [course.id]: lessonId })); setSyncPreview(null); }} />
        </div>)}
      </Checkbox.Group> : <Alert type="warning" showIcon message="没有符合年级、学科、学期和阶段条件的其他课程。" style={{ marginTop: 12 }} />}
      {syncPreview ? <div style={{ marginTop: 20 }}>
        <Typography.Text strong>确认变更</Typography.Text>
        {syncPreview.targets.map((target) => <Card key={target.courseId} size="small" title={target.courseName} style={{ marginTop: 10 }}>
          <div style={{ marginBottom: 8 }}>{[target.curriculum.unit, target.curriculum.chapter, target.curriculum.lesson].filter(Boolean).join(' · ')}</div>
          {target.items.map((item) => <div key={item.sourceMaterialId}><Tag color="green">新增</Tag>{item.tagCode} · {item.title}</div>)}
        </Card>)}
      </div> : null}
    </Modal>
  </div>;
}
