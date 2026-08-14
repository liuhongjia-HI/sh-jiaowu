import { Alert, Card, Form, Input, Skeleton, Space, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getData, postData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { ReviewBoard, ReviewDialog } from './ResourceDialogs';
import type { Review, ReviewCompleteRequest } from '../../types/starline';

export default function ReviewsPage() {
  const [form] = Form.useForm<ReviewCompleteRequest>();
  const [reviewing, setReviewing] = useState<Review | null>(null);
  const queryClient = useQueryClient();
  const reviews = useQuery({ queryKey: ['review'], queryFn: () => getData<Review[]>('/reviews/pending') });
  const complete = useMutation({ mutationFn: (values: ReviewCompleteRequest) => { if (!reviewing) throw new Error('请选择要批改的记录'); return postData(`/reviews/${reviewing.id}/complete`, { score: Number(values.score), teacherComment: values.teacherComment, reward: values.reward || '', finalStatus: values.finalStatus || '已批改' }); }, onSuccess: () => { message.success('批改反馈已保存并同步给学生。'); setReviewing(null); form.resetFields(); queryClient.invalidateQueries({ queryKey: ['review'] }); queryClient.invalidateQueries({ queryKey: ['dashboard'] }); }, onError: (error: Error) => message.error(error.message || '保存批改失败，请稍后重试。') });
  const openReview = (review: Review) => { setReviewing(review); form.setFieldsValue({ score: Number(review.systemScore ?? 0), teacherComment: review.teacherComment || '', reward: review.reward || '', finalStatus: '已批改' }); };
  if (reviews.isLoading) return <Skeleton active />;
  if (reviews.error) return <Alert type="error" message="批改反馈加载失败，请稍后重试。" />;
  return <div className="page-stack"><div className="page-heading"><div><h2>批改反馈</h2><span>处理分数、评语和学习反馈。</span></div><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => reviews.refetch()} /></div><Card><ReviewBoard rows={reviews.data ?? []} onOpen={(row) => openReview(row as Review)} /></Card><ReviewDialog form={form} review={reviewing} loading={complete.isPending} onCancel={() => setReviewing(null)} onSubmit={(values) => complete.mutate(values)} /></div>;
}
