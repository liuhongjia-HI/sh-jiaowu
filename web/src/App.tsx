import {
  BookOutlined,
  CheckCircleOutlined,
  DashboardOutlined,
  DollarOutlined,
  LogoutOutlined,
  FileTextOutlined,
  FormOutlined,
  HistoryOutlined,
  NotificationOutlined,
  ReadOutlined,
  ScheduleOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TeamOutlined,
  UnlockOutlined,
  UsergroupAddOutlined,
  UserOutlined,
  UserSwitchOutlined
} from '@ant-design/icons';
import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import type React from 'react';
import { Button, Layout, Menu, Result, Space, Tooltip, Typography } from 'antd';
import type { MenuProps } from 'antd';
import { BrowserRouter, Link, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { clearToken, getData, getToken } from './services/http';
import type { CurrentUser, Role } from './types/starline';

const { Header, Sider, Content } = Layout;

const Dashboard = lazy(() => import('./pages/Dashboard'));
const SimpleResourcePage = lazy(() => import('./pages/SimpleResourcePage'));
const OpenPackage = lazy(() => import('./pages/OpenPackage'));
const Permissions = lazy(() => import('./pages/Permissions'));
const AdminStaff = lazy(() => import('./pages/AdminStaff'));
const Teachers = lazy(() => import('./pages/Teachers'));
const Students = lazy(() => import('./pages/Students'));
const Scheduling = lazy(() => import('./pages/Scheduling'));
const Commercial = lazy(() => import('./pages/Commercial'));
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
    label: '学生与开通',
    children: [
      { key: '/students', icon: <TeamOutlined />, label: '学生管理', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/packages', icon: <BookOutlined />, label: '学习套餐', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/open', icon: <UnlockOutlined />, label: '开通套餐', roles: ['ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/permissions', icon: <SafetyCertificateOutlined />, label: '学习权限', roles: ['ops_staff', 'campus_admin', 'super_admin'] }
    ]
  },
  {
    key: 'teaching-content',
    icon: <ReadOutlined />,
    label: '教学内容',
    children: [
      { key: '/content', icon: <ReadOutlined />, label: '课程内容', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/scheduling', icon: <ScheduleOutlined />, label: '排课管理', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/materials', icon: <FileTextOutlined />, label: '学习资料', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/homework', icon: <FormOutlined />, label: '课后练习', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/review', icon: <CheckCircleOutlined />, label: '批改反馈', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] }
    ]
  },
  {
    key: 'operation',
    icon: <DollarOutlined />,
    label: '运营管理',
    children: [
      { key: '/commercial', icon: <DollarOutlined />, label: '商业运营', roles: ['ops_staff', 'campus_admin', 'super_admin'] },
      { key: '/notices', icon: <NotificationOutlined />, label: '通知提醒', roles: ['teacher', 'ops_staff', 'campus_admin', 'super_admin'] }
    ]
  },
  {
    key: 'system',
    icon: <SettingOutlined />,
    label: '系统',
    children: [
      { key: '/admin-staff', icon: <UsergroupAddOutlined />, label: '管理人员', roles: ['super_admin'] },
      { key: '/teachers', icon: <UserSwitchOutlined />, label: '教师管理', roles: ['campus_admin', 'super_admin'] },
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

function Shell({ user }: { user: CurrentUser }) {
  const location = useLocation();
  const items = useMemo(() => buildMenuItems(user), [user]);
  const activeOpenKeys = useMemo(() => findOpenKeys(location.pathname), [location.pathname]);
  const currentItem = useMemo(() => findCurrentItem(location.pathname), [location.pathname]);
  const [openKeys, setOpenKeys] = useState(activeOpenKeys);

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
            <Typography.Title level={4}>{currentItem.label}</Typography.Title>
          </div>
          <Space size={12}>
            <div className="user-pill">
              <span className="user-avatar"><UserOutlined /></span>
              <div>
                <strong>{user.name}</strong>
                <span>{roleLabel(user)}</span>
              </div>
            </div>
            <Tooltip title="退出登录">
              <Button
                aria-label="退出登录"
                icon={<LogoutOutlined />}
                onClick={() => {
                  clearToken();
                  window.location.href = '/login';
                }}
              />
            </Tooltip>
          </Space>
        </Header>
        <Content className="app-content">
          <Suspense fallback={<PageLoading />}>
            <Routes>
              <Route path="/dashboard" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><Dashboard /></GuardedRoute>} />
              <Route path="/packages" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><SimpleResourcePage kind="packages" user={user} /></GuardedRoute>} />
              <Route path="/open" element={<GuardedRoute user={user} roles={['ops_staff', 'campus_admin', 'super_admin']}><OpenPackage /></GuardedRoute>} />
              <Route path="/permissions" element={<GuardedRoute user={user} roles={['ops_staff', 'campus_admin', 'super_admin']}><Permissions /></GuardedRoute>} />
              <Route path="/admin-staff" element={<GuardedRoute user={user} roles={['super_admin']}><AdminStaff /></GuardedRoute>} />
              <Route path="/teachers" element={<GuardedRoute user={user} roles={['campus_admin', 'super_admin']}><Teachers /></GuardedRoute>} />
              <Route path="/students" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><Students user={user} /></GuardedRoute>} />
              <Route path="/content" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><SimpleResourcePage kind="content" user={user} /></GuardedRoute>} />
              <Route path="/scheduling" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><Scheduling user={user} /></GuardedRoute>} />
              <Route path="/commercial" element={<GuardedRoute user={user} roles={['ops_staff', 'campus_admin', 'super_admin']}><Commercial /></GuardedRoute>} />
              <Route path="/materials" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><SimpleResourcePage kind="materials" user={user} /></GuardedRoute>} />
              <Route path="/homework" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><SimpleResourcePage kind="homework" user={user} /></GuardedRoute>} />
              <Route path="/review" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><SimpleResourcePage kind="review" /></GuardedRoute>} />
              <Route path="/notices" element={<GuardedRoute user={user} roles={['teacher', 'ops_staff', 'campus_admin', 'super_admin']}><SimpleResourcePage kind="notices" /></GuardedRoute>} />
              <Route path="/logs" element={<GuardedRoute user={user} roles={['campus_admin', 'super_admin']}><SimpleResourcePage kind="logs" /></GuardedRoute>} />
              <Route path="/settings" element={<GuardedRoute user={user} roles={['campus_admin', 'super_admin']}><SimpleResourcePage kind="settings" /></GuardedRoute>} />
              <Route path="*" element={<Navigate to="/dashboard" />} />
            </Routes>
          </Suspense>
        </Content>
      </Layout>
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
  return <BrowserRouter><Routes><Route path="/login" element={<Navigate to="/dashboard" />} /><Route path="*" element={<Shell user={me.data} />} /></Routes></BrowserRouter>;
}
