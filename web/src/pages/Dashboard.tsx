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
          <Typography.Title level={2}>把需要跟进的学生、课程和资料放在一屏里</Typography.Title>
          <Typography.Text>优先处理待批改、到期续费和未发布资料，减少跨页面查找。</Typography.Text>
        </div>
        <Space size={10} wrap>
          <Button type="primary" icon={<TeamOutlined />}>
            <Link to="/students">查看学生</Link>
          </Button>
          <Button icon={<BookOutlined />}>
            <Link to="/packages">维护套餐</Link>
          </Button>
        </Space>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-green">
            <Statistic title="已开通学生" value={data.openedStudents} prefix={<TeamOutlined />} />
            <span>正在学习套餐内容</span>
          </Card>
        </Col>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-blue">
            <Statistic title="学习套餐" value={data.packageCount} prefix={<BookOutlined />} />
            <span>可用于开通和续费</span>
          </Card>
        </Col>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-amber">
            <Statistic title="待批改" value={data.pendingReviews} prefix={<FormOutlined />} />
            <span>需要老师处理</span>
          </Card>
        </Col>
        <Col xs={24} md={6}>
          <Card className="metric-card metric-purple">
            <Statistic title="资料访问" value={data.materialViews} prefix={<FileTextOutlined />} />
            <span>学生累计查看次数</span>
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
                  <span>优先反馈最近提交的学生，避免学习结果滞后。</span>
                </div>
                <ArrowRightOutlined />
              </Link>
              <Link className="todo-item warning" to="/students">
                <span className="todo-icon"><WarningOutlined /></span>
                <div>
                  <strong>{data.expiringStudents} 名学生套餐即将到期</strong>
                  <span>建议提前提醒家长续费，减少学习中断。</span>
                </div>
                <ArrowRightOutlined />
              </Link>
              <Link className="todo-item" to="/materials">
                <span className="todo-icon"><FileTextOutlined /></span>
                <div>
                  <strong>{data.unpublishedFiles} 份学习资料未发布</strong>
                  <span>检查课程范围和资料状态后再开放给学生。</span>
                </div>
                <ArrowRightOutlined />
              </Link>
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={9}>
          <Card title="运营健康度">
            <div className="health-panel">
              <Progress type="dashboard" percent={reviewProgress} strokeColor="#07c160" />
              <div>
                <Typography.Title level={5}>批改处理节奏</Typography.Title>
                <Typography.Text type="secondary">待批改越少，学生端结果页和成长记录越完整。</Typography.Text>
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
