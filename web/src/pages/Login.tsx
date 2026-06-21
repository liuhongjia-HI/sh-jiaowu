import {
  ArrowRightOutlined,
  CheckCircleOutlined,
  LockOutlined,
  PhoneOutlined,
  SafetyCertificateOutlined
} from '@ant-design/icons';
import { Button, Form, Input, Space, Typography, message } from 'antd';
import { useState } from 'react';
import { loginWithAdminPassword } from '../services/http';

type LoginFormValues = {
  phone: string;
  password: string;
};

export default function Login() {
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<LoginFormValues>();
  const env = (import.meta as { env?: Record<string, string> }).env ?? {};
  const showDemoAccounts = env.VITE_DEMO_ACCOUNTS_ENABLED === 'true' || env.MODE !== 'production';

  const demoAccounts = [
    { label: '校区管理员', phone: '13800000002', password: '123456' },
    { label: '教师', phone: '13800000004', password: '123456' },
    { label: '超级管理员', phone: '13800000001', password: '123456' }
  ];

  async function handleLogin(values: LoginFormValues) {
    setLoading(true);
    try {
      await loginWithAdminPassword(values.phone, values.password);
      message.success('登录成功');
      window.location.href = '/dashboard';
    } catch (error: any) {
      message.error(error.response?.data?.message || '登录失败，请检查手机号和密码。');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-shell">
        <section className="login-panel">
          <div className="login-brand">
            <div className="login-logo">S</div>
            <div>
              <Typography.Title level={3}>Starline 教务后台</Typography.Title>
              <Typography.Text>用微信生态的方式管理校区日常</Typography.Text>
            </div>
          </div>
          <Form form={form} layout="vertical" requiredMark={false} onFinish={handleLogin} className="login-form">
            <Form.Item
              name="phone"
              label="手机号"
              rules={[
                { required: true, message: '请输入手机号' },
                { pattern: /^1\d{10}$/, message: '请输入正确的 11 位手机号' }
              ]}
            >
              <Input size="large" prefix={<PhoneOutlined />} placeholder="请输入手机号" autoComplete="username" />
            </Form.Item>
            <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
              <Input.Password size="large" prefix={<LockOutlined />} placeholder="请输入密码" autoComplete="current-password" />
            </Form.Item>
            <Button type="primary" htmlType="submit" block size="large" loading={loading} icon={<ArrowRightOutlined />}>
              进入工作台
            </Button>
          </Form>
          {showDemoAccounts && (
            <div className="login-demo">
              <Typography.Text type="secondary">快捷填入</Typography.Text>
              <Space size={8} wrap>
                {demoAccounts.map((account) => (
                  <Button
                    key={account.phone}
                    size="small"
                    onClick={() => form.setFieldsValue({ phone: account.phone, password: account.password })}
                  >
                    {account.label}
                  </Button>
                ))}
              </Space>
            </div>
          )}
        </section>

        <section className="login-hero" aria-label="产品能力">
          <div className="login-hero-content">
            <div className="login-kicker"><SafetyCertificateOutlined /> 管理后台</div>
            <Typography.Title>更轻的教务工作台</Typography.Title>
            <Typography.Paragraph>
              开通、排课、资料和批改集中处理，日常操作更清楚。
            </Typography.Paragraph>
            <div className="login-feature-grid">
              {['学生开通', '课程资料', '作业批改'].map((text) => (
                <div className="login-feature" key={text}>
                  <CheckCircleOutlined />
                  <span>{text}</span>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
