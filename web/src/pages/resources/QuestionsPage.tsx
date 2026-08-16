import { Alert, Button, Card, Form, Input, Pagination, Select, Skeleton, Space, Table, Typography, message } from 'antd';
import { EditOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { QuestionDialog } from './ResourceDialogs';
import { canUpload, hasAdminContentScope, normalizeQuestionForm, optionFromValues, questionTitle, uniqueValues } from './resource-shared';
import { gradeOptions, semesterOptions, subjectOptions } from '../../utils/curriculum';
import type { CurrentUser, LearningSpace, QuestionBankItem, QuestionBankUpsertRequest } from '../../types/starline';

export default function QuestionsPage({ user }: { user?: CurrentUser }) {
  const [form] = Form.useForm<QuestionBankUpsertRequest>();
  const [editing, setEditing] = useState<QuestionBankItem | null>(null);
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [gradeFilter, setGradeFilter] = useState<string>();
  const [subjectFilter, setSubjectFilter] = useState<string>();
  const [page, setPage] = useState(1);
  const client = useQueryClient();
  const questions = useQuery({ queryKey: ['questions', gradeFilter, subjectFilter, keyword], queryFn: () => getData<QuestionBankItem[]>('/questions', { grade: gradeFilter || '', subject: subjectFilter || '', keyword }) });
  const learningSpaces = useQuery({ queryKey: ['learning-spaces-for-questions'], queryFn: () => getData<LearningSpace[]>('/learning-spaces') });
  const settings = useQuery({ queryKey: ['settings-for-questions'], queryFn: () => getData<Record<string, string>>('/settings') });
  const currentSemesterOptions = semesterOptions(settings.data?.semesters);
  const questionScope = useMemo(() => {
    const unrestricted = hasAdminContentScope(user);
    if (unrestricted) {
      return { unrestricted, gradeOptions: gradeOptions(), semesterOptions: currentSemesterOptions, spaces: [] as LearningSpace[], hasScope: true };
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
  const canManage = canUpload('homework', user);
  const save = useMutation({
    mutationFn: (values: QuestionBankUpsertRequest) => {
      const body = normalizeQuestionForm(values);
      return editing ? putData<QuestionBankItem>(`/questions/${editing.id}`, body) : postData<QuestionBankItem>('/questions', body);
    },
    onSuccess: () => {
      message.success(editing ? '题目已保存。' : '题目已加入题库。');
      setOpen(false);
      setEditing(null);
      form.resetFields();
      client.invalidateQueries({ queryKey: ['questions'] });
      client.invalidateQueries({ queryKey: ['question-bank-for-homework'] });
    },
    onError: (error: Error) => message.error(error.message || '保存题目失败，请检查题干、选项和答案。')
  });
  const rows = questions.data ?? [];
  const paged = rows.slice((page - 1) * 10, page * 10);

  const start = (item?: QuestionBankItem) => {
    setEditing(item ?? null);
    if (item) {
      form.setFieldsValue({ ...item, title: item.title || questionTitle(item), options: item.options ?? [], answer: item.answer || (item.answers ?? [])[0] || '', answers: item.answers ?? [], score: item.score || 10, status: item.status || '启用' });
    } else {
      const defaultGrade = questionScope.gradeOptions.length === 1 ? questionScope.gradeOptions[0].value : undefined;
      const defaultSemester = questionScope.semesterOptions.length === 1 ? questionScope.semesterOptions[0].value : currentSemesterOptions[0]?.value || 'S1';
      const scopedSubjects = questionScope.unrestricted
        ? subjectOptions(defaultGrade)
        : optionFromValues(uniqueValues(questionScope.spaces.filter((space) => (!defaultGrade || space.grade === defaultGrade) && (!defaultSemester || space.semester === defaultSemester)).map((space) => space.subject)));
      form.setFieldsValue({ title: '', grade: defaultGrade as string, semester: defaultSemester, subject: (scopedSubjects.length === 1 ? scopedSubjects[0].value : undefined) as string, type: 'single', stem: '', options: [''], answer: '', answers: [], score: 10, status: '启用' });
    }
    setOpen(true);
  };

  return <div className="page-stack">
    <div className="page-heading"><div><Typography.Title level={3}>题库</Typography.Title><Typography.Text type="secondary">按年级、学期和学科维护可复用题目。</Typography.Text></div>{canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => start()}>新增题目</Button>}</div>
    {questions.isLoading ? <Skeleton active /> : questions.error ? <Alert type="error" message="题库加载失败，请稍后重试。" /> : <Card><Space wrap style={{ marginBottom: 16 }}><Select allowClear placeholder="全部年级" options={gradeOptions()} value={gradeFilter} onChange={(value) => { setGradeFilter(value); setSubjectFilter(undefined); setPage(1); }} style={{ width: 140 }} /><Select allowClear placeholder="全部学科" options={subjectOptions(gradeFilter)} value={subjectFilter} onChange={(value) => { setSubjectFilter(value); setPage(1); }} style={{ width: 140 }} /><Input.Search allowClear placeholder="搜索题目" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} /><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => questions.refetch()} /></Space><Table rowKey="id" dataSource={paged} pagination={false} columns={[{ title: '题目', dataIndex: 'title', render: (title, row) => title || row.stem }, { title: '年级', dataIndex: 'grade' }, { title: '学期', dataIndex: 'semester' }, { title: '学科', dataIndex: 'subject' }, { title: '题型', dataIndex: 'type' }, { title: '操作', render: (_: unknown, row: QuestionBankItem) => <ActionButton tooltip="编辑" icon={<EditOutlined />} onClick={() => start(row)} /> }]} />{rows.length > 10 && <Pagination current={page} pageSize={10} total={rows.length} showSizeChanger={false} onChange={setPage} style={{ marginTop: 16 }} />}</Card>}
    <QuestionDialog form={form} open={open} editing={Boolean(editing)} loading={save.isPending} scopeLoading={!questionScope.unrestricted && learningSpaces.isLoading} scopeError={Boolean(!questionScope.unrestricted && learningSpaces.error)} gradeOptions={questionScope.gradeOptions} semesterOptions={questionScope.semesterOptions} allowedSpaces={questionScope.spaces} unrestricted={questionScope.unrestricted} hasScope={questionScope.hasScope} onCancel={() => setOpen(false)} onSubmit={(values) => save.mutate(values)} />
  </div>;
}
