import { Alert, Button, Card, Form, Input, Modal, Select, Skeleton, Space, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { ReviewBoard, ReviewDialog } from './ResourceDialogs';
import type { CurrentUser, Review, ReviewAssignRequest, ReviewCompleteRequest, Teacher } from '../../types/starline';

function canManageReviewAssignment(user?: CurrentUser) {
  return Boolean(user?.roles.some((role) => ['ops_staff', 'campus_admin', 'super_admin'].includes(role)));
}

export default function ReviewsPage({ user }: { user?: CurrentUser }) {
  const exceptionRequestID = useRef('');
  const [exceptionForm] = Form.useForm<{ reason: string; className: string }>();
  const [exceptionReview, setExceptionReview] = useState<Review | null>(null);
  const [form] = Form.useForm<ReviewCompleteRequest>();
	const [assignForm] = Form.useForm<ReviewAssignRequest>();
  const [reviewing, setReviewing] = useState<Review | null>(null);
	const [assigning, setAssigning] = useState<Review | null>(null);
  const queryClient = useQueryClient();
	const canAssign = canManageReviewAssignment(user);
  const reviews = useQuery({ queryKey: ['review'], queryFn: () => getData<Review[]>('/reviews/pending') });
	const teachers = useQuery({ queryKey: ['teachers', 'review-assignment'], enabled: canAssign, queryFn: () => getData<Teacher[]>('/teachers') });
  const reasons = useQuery({ queryKey: ['review-exception-reasons'], enabled: Boolean(exceptionReview), queryFn: () => getData<string[]>('/reviews/exception-reasons') });
  const markException = useMutation({ mutationFn: (values: { reason: string; className: string }) => { if (!exceptionReview) throw new Error('请选择批改记录'); return postData<Review>(`/reviews/${exceptionReview.id}/exception`, { ...values, requestId: exceptionRequestID.current }); }, onSuccess: () => { message.success('已标记批改异常'); setExceptionReview(null); exceptionForm.resetFields(); queryClient.invalidateQueries({ queryKey: ['review'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '异常保存失败') });
  const openException = (review: Review) => { exceptionRequestID.current = crypto.randomUUID(); setExceptionReview(review); exceptionForm.setFieldsValue({ className: review.exceptionClass || '', reason: review.exceptionReason || undefined }); };
  const complete = useMutation({ mutationFn: (values: ReviewCompleteRequest) => { if (!reviewing) throw new Error('请选择要批改的记录'); return postData(`/reviews/${reviewing.id}/complete`, { score: Number(values.score), teacherComment: values.teacherComment, reward: values.reward || '', finalStatus: values.finalStatus || '已批改' }); }, onSuccess: () => { message.success('批改反馈已保存并同步给学生。'); setReviewing(null); form.resetFields(); queryClient.invalidateQueries({ queryKey: ['review'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '保存批改失败，请稍后重试。') });
	const assign = useMutation({ mutationFn: (values: ReviewAssignRequest) => { if (!assigning) throw new Error('请选择要分派的任务'); return postData<Review>(`/reviews/${assigning.id}/assign`, values); }, onSuccess: () => { message.success('批改任务已分派。'); setAssigning(null); assignForm.resetFields(); queryClient.invalidateQueries({ queryKey: ['review'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '分派失败，请检查老师权限和校区范围。') });
  const openReview = (review: Review) => { setReviewing(review); form.setFieldsValue({ score: Number(review.systemScore ?? 0), teacherComment: review.teacherComment || '', reward: review.reward || '', finalStatus: '已批改' }); };
	const openAssign = (review: Review) => { setAssigning(review); assignForm.setFieldsValue({ teacherId: review.reviewerTeacherId || undefined, reason: '' }); };
  if (reviews.isLoading) return <Skeleton active />;
  if (reviews.error) return <Alert type="error" message="批改反馈加载失败，请稍后重试。" />;
  return <div className="page-stack"><div className="page-heading"><div><h2>批改反馈</h2><span>{canAssign ? '教务可将待分派任务交给有权限的老师；已分派任务保留责任快照。' : '这里只显示明确分派给我的任务。'}</span></div><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => reviews.refetch()} /></div><Card><ReviewBoard rows={reviews.data ?? []} onOpen={(row) => openReview(row as Review)} onAssign={canAssign ? openAssign : undefined} onException={openException} /></Card><ReviewDialog form={form} review={reviewing} loading={complete.isPending} onCancel={() => setReviewing(null)} onSubmit={(values) => complete.mutate(values)} /><Modal title="标记批改异常" open={Boolean(exceptionReview)} onCancel={() => setExceptionReview(null)} onOk={() => exceptionForm.submit()} okText="保存异常" confirmLoading={markException.isPending} okButtonProps={{ disabled: reasons.isLoading || Boolean(reasons.error) || !(reasons.data?.length) }} destroyOnHidden>
    <Form form={exceptionForm} layout="vertical" preserve={false} initialValues={{ className: exceptionReview?.exceptionClass || '', reason: exceptionReview?.exceptionReason || undefined }} onFinish={values => markException.mutate(values)}>
      <Form.Item label="作业"><Input disabled value={exceptionReview ? `${exceptionReview.studentName} · ${exceptionReview.homework}` : ''} /></Form.Item>
      {reasons.error ? <Alert type="error" message="异常原因加载失败" action={<Button onClick={() => reasons.refetch()}>重试</Button>} /> : !reasons.isLoading && !reasons.data?.length && <Alert type="warning" message="后台尚未配置微信审核通过的异常原因，请联系教务" />}
      <Form.Item name="className" label="实际班级" rules={[{ required: true, whitespace: true, message: '请填写实际班级名称' }, { max: 20, message: '班级名称最多20个字符' }]}><Input aria-label="实际班级" maxLength={20} placeholder="填写该作业对应的实际班级" /></Form.Item>
      <Form.Item name="reason" label="异常原因" rules={[{ required: true, message: '请选择异常原因' }]}><Select aria-label="异常原因" loading={reasons.isLoading} placeholder="选择微信审核通过的原因" options={(reasons.data || []).map(value => ({ value, label: value }))} /></Form.Item>
    </Form>
  </Modal><Modal title={assigning?.reviewerTeacherName ? '转派批改任务' : '分派批改任务'} open={Boolean(assigning)} onCancel={() => setAssigning(null)} onOk={() => assignForm.submit()} confirmLoading={assign.isPending} destroyOnHidden><Form form={assignForm} layout="vertical" onFinish={(values) => assign.mutate(values)}><Form.Item label="任务" ><Input value={assigning ? `${assigning.studentName} · ${assigning.homework}` : ''} disabled /></Form.Item><Form.Item name="teacherId" label="负责老师" rules={[{ required: true, message: '请选择负责老师' }]}><Select loading={teachers.isLoading} placeholder="只显示有批改权限的老师" options={(teachers.data ?? []).filter((teacher) => teacher.accountStatus === '正常' && teacher.canReview).map((teacher) => ({ value: teacher.id, label: `${teacher.name} · ${teacher.learningSpaces.join('、') || '未配置范围'}` }))} /></Form.Item><Form.Item name="reason" label="分派原因" rules={[{ required: true, message: '请说明分派或转派原因' }]}><Input.TextArea rows={3} placeholder="例如：原负责老师请假，由同范围老师接手" /></Form.Item></Form></Modal></div>;
}
