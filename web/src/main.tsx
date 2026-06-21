import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from './App';
import './styles.css';

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#0f8f72',
          colorLink: '#0f8f72',
          colorSuccess: '#0f8f72',
          colorWarning: '#d97706',
          colorInfo: '#2563eb',
          colorText: '#17202a',
          colorTextSecondary: '#667085',
          colorBorder: '#dfe5ec',
          colorBgLayout: '#eef3f7',
          borderRadius: 8,
          fontSize: 14,
          controlHeight: 38
        },
        components: {
          Button: {
            controlHeight: 38,
            fontWeight: 600
          },
          Card: {
            borderRadiusLG: 8
          },
          Table: {
            headerBg: '#f7fafc',
            headerColor: '#344054',
            rowHoverBg: '#f8fbfb'
          }
        }
      }}
    >
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </ConfigProvider>
  </React.StrictMode>
);
