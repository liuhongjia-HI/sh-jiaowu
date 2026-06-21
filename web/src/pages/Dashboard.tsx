import {
  Alert,
  Button,
  Card,
  Col,
  Progress,
  Row,
  Skeleton,
  Space,
  Statistic,
  Tag,
  Tooltip,
  Typography
} from 'antd';
import {
  ArrowRightOutlined,
  BookOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  FormOutlined,
  TeamOutlined,
  WarningOutlined
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { getData } from '../services/http';
import type { DashboardOverview } from '../types/starline';

export default function Dashboard() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['dashboard'],
    queryFn: () => getData<DashboardOverview>('/dashboard/overview')
  });

  if (isLoading) return <Skeleton active />;
  if (error || !data) return <Alert type="error" message="工作台加载失败，请稍后重试。" />;

  const reviewProgress = data.pendingReviews > 0 ? Math.max(12, 100 - data.pendingReviews * 8) : 100;

  return (
    <div className="page-stack">
      <div className="dashboard-hero">
        <div>
          <Tag color="green">今日运营</Tag>
          <Typography.Title level={2}>今日待办</Typography.Title>
          <Typography.Text>先处理批改、续费和资料发布。</Typography.Text>
        </div>
        <Space size={10} wrap>
          <Tooltip title="学生">
            <Link to="/students" aria-label="学生">
              <Button type="primary" icon={<TeamOutlined />} />
            </Link>
          </Tooltip>
          <Tooltip title="套餐">
            <Link to="/packages" aria-label="套餐">
              <Button icon={<BookOutlined />} />
            </Link>
          </Tooltip>
        </Space>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-green">
            <Statistic title="学生" value={data.openedStudents} prefix={<TeamOutlined />} />
            <span>已开通</span>
          </Card>
        </Col>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-blue">
            <Statistic title="套餐" value={data.packageCount} prefix={<BookOutlined />} />
            <span>可开通</span>
          </Card>
        </Col>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-amber">
            <Statistic title="待批改" value={data.pendingReviews} prefix={<FormOutlined />} />
            <span>未处理</span>
          </Card>
        </Col>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-purple">
            <Statistic title="资料访问" value={data.materialViews} prefix={<FileTextOutlined />} />
            <span>累计查看</span>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={15}>
          <Card title="今日待办" extra={<Link to="/review">处理批改 <ArrowRightOutlined /></Link>}>
            <div className="todo-list">
              <Link className="todo-item high" to="/review">
                <span className="todo-icon"><FormOutlined /></span>
                <div>
                  <strong>{data.pendingReviews} 份课后练习待批改</strong>
                  <span>先反馈最近提交的学生。</span>
                </div>
                <ArrowRightOutlined />
              </Link>
              <Link className="todo-item warning" to="/students">
                <span className="todo-icon"><WarningOutlined /></span>
                <div>
                  <strong>{data.expiringStudents} 名学生套餐即将到期</strong>
                  <span>提前提醒续费。</span>
                </div>
                <ArrowRightOutlined />
              </Link>
              <Link className="todo-item" to="/materials">
                <span className="todo-icon"><FileTextOutlined /></span>
                <div>
                  <strong>{data.unpublishedFiles} 份学习资料未发布</strong>
                  <span>确认课程范围后发布。</span>
                </div>
                <ArrowRightOutlined />
              </Link>
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={9}>
          <Card title="批改进度">
            <div className="health-panel">
              <Progress type="dashboard" percent={reviewProgress} strokeColor="#07c160" />
              <div>
                <Typography.Title level={5}>处理节奏</Typography.Title>
                <Typography.Text type="secondary">待批改越少，反馈越及时。</Typography.Text>
              </div>
            </div>
            <div className="process-list">
              <div><ClockCircleOutlined /> 创建或编辑学习套餐</div>
              <div><TeamOutlined /> 给学生开通并确认有效期</div>
              <div><FileTextOutlined /> 发布资料和练习到学生端</div>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
