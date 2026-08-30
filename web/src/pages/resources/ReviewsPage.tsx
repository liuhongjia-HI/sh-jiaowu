import { Alert, Card, Form, Input, Modal, Select, Skeleton, Space, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { ReviewBoard, ReviewDialog } from './ResourceDialogs';
import type { CurrentUser, Review, ReviewAssignRequest, ReviewCompleteRequest, Teacher } from '../../types/starline';

function canManageReviewAssignment(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

export default function ReviewsPage({ user }: { user?: CurrentUser }) {
  const [form] = Form.useForm<ReviewCompleteRequest>();
	const [assignForm] = Form.useForm<ReviewAssignRequest>();
  const [reviewing, setReviewing] = useState<Review | null>(null);
	const [assigning, setAssigning] = useState<Review | null>(null);
  const queryClient = useQueryClient();
	const canAssign = canManageReviewAssignment(user);
  const reviews = useQuery({ queryKey: ['review'], queryFn: () => getData<Review[]>('/reviews/pending') });
	const teachers = useQuery({ queryKey: ['teachers', 'review-assignment'], enabled: canAssign, queryFn: () => getData<Teacher[]>('/teachers') });
  const complete = useMutation({ mutationFn: (values: ReviewCompleteRequest) => { if (!reviewing) throw new Error('请选择要批改的记录'); return postData(`/reviews/${reviewing.id}/complete`, { score: Number(values.score), teacherComment: values.teacherComment, reward: values.reward || '', finalStatus: values.finalStatus || '已批改' }); }, onSuccess: () => { message.success('批改反馈已保存并同步给学生。'); setReviewing(null); form.resetFields(); queryClient.invalidateQueries({ queryKey: ['review'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '保存批改失败，请稍后重试。') });
	const assign = useMutation({ mutationFn: (values: ReviewAssignRequest) => { if (!assigning) throw new Error('请选择要分派的任务'); return postData<Review>(`/reviews/${assigning.id}/assign`, values); }, onSuccess: () => { message.success('批改任务已分派。'); setAssigning(null); assignForm.resetFields(); queryClient.invalidateQueries({ queryKey: ['review'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '分派失败，请检查老师权限和校区范围。') });
  const openReview = (review: Review) => { setReviewing(review); form.setFieldsValue({ score: Number(review.systemScore ?? 0), teacherComment: review.teacherComment || '', reward: review.reward || '', finalStatus: '已批改' }); };
	const openAssign = (review: Review) => { setAssigning(review); assignForm.setFieldsValue({ teacherId: review.reviewerTeacherId || undefined, reason: '' }); };
  if (reviews.isLoading) return <Skeleton active />;
  if (reviews.error) return <Alert type="error" message="批改反馈加载失败，请稍后重试。" />;
  return <div className="page-stack"><div className="page-heading"><div><h2>批改反馈</h2><span>{canAssign ? '教务可将待分派任务交给有权限的老师；已分派任务保留责任快照。' : '这里只显示明确分派给我的任务。'}</span></div><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => reviews.refetch()} /></div><Card><ReviewBoard rows={reviews.data ?? []} onOpen={(row) => openReview(row as Review)} onAssign={canAssign ? openAssign : undefined} /></Card><ReviewDialog form={form} review={reviewing} loading={complete.isPending} onCancel={() => setReviewing(null)} onSubmit={(values) => complete.mutate(values)} /><Modal title={assigning?.reviewerTeacherName ? '转派批改任务' : '分派批改任务'} open={Boolean(assigning)} onCancel={() => setAssigning(null)} onOk={() => assignForm.submit()} confirmLoading={assign.isPending} destroyOnHidden><Form form={assignForm} layout="vertical" onFinish={(values) => assign.mutate(values)}><Form.Item label="任务" ><Input value={assigning ? `${assigning.studentName} · ${assigning.homework}` : ''} disabled /></Form.Item><Form.Item name="teacherId" label="负责老师" rules={[{ required: true, message: '请选择负责老师' }]}><Select loading={teachers.isLoading} placeholder="只显示有批改权限的老师" options={(teachers.data ?? []).filter((teacher) => teacher.accountStatus === '正常' && teacher.canReview).map((teacher) => ({ value: teacher.id, label: `${teacher.name} · ${teacher.learningSpaces.join('、') || '未配置范围'}` }))} /></Form.Item><Form.Item name="reason" label="分派原因" rules={[{ required: true, message: '请说明分派或转派原因' }]}><Input.TextArea rows={3} placeholder="例如：原负责老师请假，由同范围老师接手" /></Form.Item></Form></Modal></div>;
}
