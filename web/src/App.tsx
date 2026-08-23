import {
  BookOutlined,
  PictureOutlined,
  DashboardOutlined,
  DownOutlined,
  DollarOutlined,
  LogoutOutlined,
  FormOutlined,
  HistoryOutlined,
  KeyOutlined,
  NotificationOutlined,
  ReadOutlined,
  ScheduleOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
  UsergroupAddOutlined,
  UserOutlined,
  UserSwitchOutlined
} from '@ant-design/icons';
import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import type React from 'react';
import { Alert, Button, Dropdown, Form, Input, Layout, Menu, Modal, Result, Space, Typography, message } from 'antd';
import type { MenuProps } from 'antd';
import { BrowserRouter, Link, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { changePassword, clearToken, getData, getToken, logout } from './services/http';
import type { CurrentUser, Role } from './types/starline';

const { Header, Sider, Content } = Layout;

const Dashboard = lazy(() => import('./pages/Dashboard'));
const PackagesPage = lazy(() => import('./pages/resources/PackagesPage'));
const ContentPage = lazy(() => import('./pages/resources/ContentPage'));
const QuestionsPage = lazy(() => import('./pages/resources/QuestionsPage'));
const NoticesPage = lazy(() => import('./pages/resources/NoticesPage'));
const LogsPage = lazy(() => import('./pages/resources/LogsPage'));
const SettingsPage = lazy(() => import('./pages/resources/SettingsPage'));
const AdminStaff = lazy(() => import('./pages/AdminStaff'));
const Teachers = lazy(() => import('./pages/Teachers'));
const Students = lazy(() => import('./pages/Students'));
const Scheduling = lazy(() => import('./pages/scheduling/SchedulingPage'));
const Commercial = lazy(() => import('./pages/Commercial'));
const Banners = lazy(() => import('./pages/Banners'));
const Login = lazy(() => import('./pages/Login'));

type NavItem = {
  key: string;
  icon: React.ReactNode;
  label: string;
  roles: Role[];
};

type NavGroup = {
  key: string;
  icon: React.ReactNode;
  label: string;
  children: NavItem[];
};

type NavNode = NavItem | NavGroup;

const navItems: NavNode[] = [
  {
    key: '/dashboard',
    icon: <DashboardOutlined />,
    label: '工作台',
    roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin']
  },
  {
    key: 'student-access',
    icon: <SafetyCertificateOutlined />,
    label: '学生与老师',
    children: [
      { key: '/students', icon: <TeamOutlined />, label: '学生管理', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/teachers', icon: <UserSwitchOutlined />, label: '老师管理', roles: ['campus_admin', 'super_admin'] },
      { key: '/packages', icon: <BookOutlined />, label: '课程方案', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] }
    ]
  },
  {
    key: 'teaching-content',
    icon: <ReadOutlined />,
    label: '教学内容',
    children: [
      { key: '/content', icon: <ReadOutlined />, label: '课程内容', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/scheduling', icon: <ScheduleOutlined />, label: '排课管理', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/questions', icon: <FormOutlined />, label: '题库', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] }
    ]
  },
  {
    key: 'operation',
    icon: <DollarOutlined />,
    label: '运营管理',
    children: [
      { key: '/commercial', icon: <DollarOutlined />, label: '商业运营', roles: ['ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/notices', icon: <NotificationOutlined />, label: '通知提醒', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/banners', icon: <PictureOutlined />, label: '轮播图管理', roles: ['ops_staff', 'campus_admin', 'super_admin'] }
    ]
  },
  {
    key: 'system',
    icon: <SettingOutlined />,
    label: '系统',
    children: [
      { key: '/admin-staff', icon: <UsergroupAddOutlined />, label: '管理人员', roles: ['super_admin'] },
      { key: '/logs', icon: <HistoryOutlined />, label: '操作记录', roles: ['campus_admin', 'super_admin'] },
      { key: '/settings', icon: <SettingOutlined />, label: '系统设置', roles: ['campus_admin', 'super_admin'] }
    ]
  }
];

function hasAnyRole(user: CurrentUser, roles: Role[]) {
  return user.roles.some((role) => roles.includes(role));
}

function isNavGroup(item: NavNode): item is NavGroup {
  return 'children' in item;
}

function buildMenuItems(user: CurrentUser): MenuProps['items'] {
  const items: MenuProps['items'] = [];

  for (const item of navItems) {
    if (!isNavGroup(item)) {
      if (hasAnyRole(user, item.roles)) {
        items.push({
          key: item.key,
          icon: item.icon,
          label: <Link to={item.key}>{item.label}</Link>
        });
      }
      continue;
    }

    const children = item.children
      .filter((child) => hasAnyRole(user, child.roles))
      .map((child) => ({
        key: child.key,
        icon: child.icon,
        label: <Link to={child.key}>{child.label}</Link>
      }));

    if (children.length > 0) {
      items.push({
        key: item.key,
        icon: item.icon,
        label: item.label,
        children
      });
    }
  }

  return items;
}

function findOpenKeys(pathname: string) {
  for (const item of navItems) {
    if (!isNavGroup(item)) continue;
    if (item.children.some((child) => child.key === pathname)) {
      return [item.key];
    }
  }
  return [];
}

function findCurrentItem(pathname: string) {
  for (const item of navItems) {
    if (!isNavGroup(item) && item.key === pathname) return item;
    if (isNavGroup(item)) {
      const child = item.children.find((nav) => nav.key === pathname);
      if (child) return child;
    }
  }
  return { key: '/dashboard', icon: <DashboardOutlined />, label: '工作台', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] as Role[] };
}

function roleLabel(user: CurrentUser) {
  if (user.roles.includes('super_admin')) return '超级管理员';
  if (user.roles.includes('campus_admin')) return '校区管理员';
  if (user.roles.includes('ops_staff')) return '运营教务';
  if (user.roles.includes('teacher')) return '教师';
  return '学生';
}

function GuardedRoute({ user, roles, children }: { user: CurrentUser; roles: Role[]; children: React.ReactNode }) {
  if (!hasAnyRole(user, roles)) {
    return <Result status="403" title="没有权限" subTitle="当前账号不能访问这个功能" />;
  }
  return <>{children}</>;
}

function PageLoading({ text = '正在加载页面...' }: { text?: string }) {
  return <div className="page-loading">{text}</div>;
}

type PasswordFormValues = {
  oldPassword: string;
  newPassword: string;
  confirmPassword: string;
};

function MustChangePasswordPage({ user }: { user: CurrentUser }) {
  const [form] = Form.useForm<PasswordFormValues>();
  const [saving, setSaving] = useState(false);

  async function submit(values: PasswordFormValues) {
    if (values.newPassword !== values.confirmPassword) {
      message.error('两次输入的新密码不一致。');
      return;
    }
    setSaving(true);
    try {
      await changePassword(values.oldPassword, values.newPassword);
      message.success('密码已修改，请用新密码重新登录。');
      clearToken();
      window.location.href = '/login';
    } catch (error: any) {
      message.error(error.response?.data?.message || error.message || '修改失败，请检查原密码。');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="login-page">
      <section className="login-panel">
        <div className="login-brand">
          <div className="login-logo">S</div>
          <div>
            <Typography.Title level={3}>修改初始密码</Typography.Title>
            <Typography.Text>{user.name}，请先设置自己的后台登录密码。</Typography.Text>
          </div>
        </div>
        <Alert type="info" showIcon message="这只影响手机号密码登录。微信登录不会要求修改初始密码。" style={{ marginBottom: 16 }} />
        <Form form={form} layout="vertical" requiredMark={false} onFinish={submit}>
          <Form.Item name="oldPassword" label="临时密码" rules={[{ required: true, message: '请输入临时密码' }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '新密码至少 8 位' },
              { pattern: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '新密码需同时包含字母和数字' }
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="confirmPassword" label="确认新密码" rules={[{ required: true, message: '请再次输入新密码' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block size="large" loading={saving}>
            保存并重新登录
          </Button>
        </Form>
      </section>
    </div>
  );
}

function ChangePasswordModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [form] = Form.useForm<PasswordFormValues>();
  const [saving, setSaving] = useState(false);

  async function submit(values: PasswordFormValues) {
    setSaving(true);
    try {
      await changePassword(values.oldPassword, values.newPassword);
      message.success('密码已修改，请用新密码重新登录。');
      clearToken();
      window.location.href = '/login';
    } catch (error: any) {
      message.error(error.response?.data?.message || error.message || '修改失败，请检查原密码。');
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      title="修改密码"
      open={open}
      okText="保存并重新登录"
      cancelText="取消"
      confirmLoading={saving}
      onOk={() => form.submit()}
      onCancel={() => {
        if (saving) return;
        form.resetFields();
        onClose();
      }}
      destroyOnClose
    >
      <Alert type="info" showIcon message="修改后当前登录会失效，需要使用新密码重新登录。" style={{ marginBottom: 16 }} />
      <Form form={form} layout="vertical" requiredMark={false} preserve={false} onFinish={submit}>
        <Form.Item name="oldPassword" label="当前密码" rules={[{ required: true, message: '请输入当前密码' }]}>
          <Input.Password autoComplete="current-password" />
        </Form.Item>
        <Form.Item
          name="newPassword"
          label="新密码"
          rules={[
            { required: true, message: '请输入新密码' },
            { min: 8, message: '新密码至少 8 位' },
            { pattern: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '新密码需同时包含字母和数字' }
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item
          name="confirmPassword"
          label="确认新密码"
          dependencies={['newPassword']}
          rules={[
            { required: true, message: '请再次输入新密码' },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('newPassword') === value) {
                  return Promise.resolve();
                }
                return Promise.reject(new Error('两次输入的新密码不一致。'));
              }
            })
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function Shell({ user }: { user: CurrentUser }) {
  const location = useLocation();
  const items = useMemo(() => buildMenuItems(user), [user]);
  const activeOpenKeys = useMemo(() => findOpenKeys(location.pathname), [location.pathname]);
  const currentItem = useMemo(() => findCurrentItem(location.pathname), [location.pathname]);
  const [openKeys, setOpenKeys] = useState(activeOpenKeys);
  const [passwordModalOpen, setPasswordModalOpen] = useState(false);
  const canChangePassword = user.authMethod === 'password';

  async function handleLogout() {
    try {
      await logout();
    } catch {
      message.warning('本机已退出，服务端登录状态稍后自动过期。');
    } finally {
      clearToken();
      window.location.href = '/login';
    }
  }

  const accountMenuItems: MenuProps['items'] = [
    ...(canChangePassword ? [{ key: 'change-password', icon: <KeyOutlined />, label: '修改密码' }] : []),
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', danger: true }
  ];

  useEffect(() => {
    setOpenKeys((current) => Array.from(new Set([...current, ...activeOpenKeys])));
  }, [activeOpenKeys]);

  return (
    <Layout className="app-shell">
      <Sider width={264} theme="dark" className="app-sider">
        <div className="brand">
          <div className="brand-mark">S</div>
          <div>
            <strong>Starline</strong>
            <span>教务运营中心</span>
          </div>
        </div>
        <Menu mode="inline" selectedKeys={[location.pathname]} openKeys={openKeys} onOpenChange={setOpenKeys} items={items} />
      </Sider>
      <Layout>
        <Header className="app-header">
          <div className="header-title-group">
            <Typography.Text className="app-header-title">{currentItem.label}</Typography.Text>
          </div>
          <Space size={12}>
            <Dropdown
              menu={{
                items: accountMenuItems,
                onClick: ({ key }) => {
                  if (key === 'change-password') {
                    setPasswordModalOpen(true);
                    return;
                  }
                  if (key === 'logout') {
                    void handleLogout();
                  }
                }
              }}
              trigger={['click']}
              placement="bottomRight"
            >
              <button type="button" className="user-pill account-trigger" aria-label="账号菜单">
                <span className="user-avatar"><UserOutlined /></span>
                <span>
                  <strong>{user.name}</strong>
                  <span>{roleLabel(user)}</span>
                </span>
                <DownOutlined className="account-chevron" />
              </button>
            </Dropdown>
          </Space>
        </Header>
        <Content className="app-content">
          <Suspense fallback={<PageLoading />}>
            <Routes>
              <Route path="/dashboard" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><Dashboard /></GuardedRoute>} />
              <Route path="/packages" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><PackagesPage user={user} /></GuardedRoute>} />
              <Route path="/open" element={<Navigate to="/students" replace />} />
              <Route path="/permissions" element={<Navigate to="/students" replace />} />
              <Route path="/admin-staff" element={<GuardedRoute user={user} roles={['super_admin']}><AdminStaff /></GuardedRoute>} />
              <Route path="/teachers" element={<GuardedRoute user={user} roles={['campus_admin', 'super_admin']}><Teachers /></GuardedRoute>} />
              <Route path="/students" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><Students user={user} /></GuardedRoute>} />
              <Route path="/content" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><ContentPage user={user} /></GuardedRoute>} />
              <Route path="/scheduling" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><Scheduling user={user} /></GuardedRoute>} />
              <Route path="/questions" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><QuestionsPage user={user} /></GuardedRoute>} />
              <Route path="/commercial" element={<GuardedRoute user={user} roles={['ops_staff', 'campus_admin', 'super_admin']}><Commercial /></GuardedRoute>} />
              <Route path="/materials" element={<Navigate to="/content?tab=materials" replace />} />
              <Route path="/homework" element={<Navigate to="/content?tab=homework" replace />} />
              <Route path="/review" element={<Navigate to="/content?tab=review" replace />} />
              <Route path="/notices" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><NoticesPage /></GuardedRoute>} />
              <Route path="/banners" element={<GuardedRoute user={user} roles={['ops_staff', 'campus_admin', 'super_admin']}><Banners /></GuardedRoute>} />
              <Route path="/logs" element={<GuardedRoute user={user} roles={['campus_admin', 'super_admin']}><LogsPage /></GuardedRoute>} />
              <Route path="/settings" element={<GuardedRoute user={user} roles={['campus_admin', 'super_admin']}><SettingsPage /></GuardedRoute>} />
              <Route path="*" element={<Navigate to="/dashboard" />} />
            </Routes>
          </Suspense>
        </Content>
      </Layout>
      <ChangePasswordModal open={passwordModalOpen} onClose={() => setPasswordModalOpen(false)} />
    </Layout>
  );
}

export default function App() {
  const token = getToken();
  const loginRoutes = (
    <BrowserRouter>
      <Suspense fallback={<PageLoading />}>
        <Routes><Route path="/login" element={<Login />} /><Route path="*" element={<Navigate to="/login" />} /></Routes>
      </Suspense>
    </BrowserRouter>
  );
  const me = useQuery({
    queryKey: ['auth', 'me', token],
    enabled: Boolean(token),
    retry: false,
    queryFn: () => getData<CurrentUser>('/auth/me')
  });

  if (!token) {
    return loginRoutes;
  }
  if (me.isLoading) {
    return <PageLoading text="正在进入后台..." />;
  }
  if (me.error || !me.data) {
    clearToken();
    return loginRoutes;
  }
  if (me.data.mustChangePassword && me.data.authMethod === 'password') {
    return <BrowserRouter><Routes><Route path="*" element={<MustChangePasswordPage user={me.data} />} /></Routes></BrowserRouter>;
  }
  return <BrowserRouter><Routes><Route path="/login" element={<Navigate to="/dashboard" />} /><Route path="*" element={<Shell user={me.data} />} /></Routes></BrowserRouter>;
}
