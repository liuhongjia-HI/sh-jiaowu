import { Alert, Card, Empty, Input, Pagination, Skeleton, Space, Table, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getData } from '../../services/http';
import { ActionButton, CardList, InfoCard, ListViewToggle, useListViewMode } from '../../components/ListViews';

type LogRow = Record<string, unknown>;

export default function LogsPage() {
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:logs');
  const [keyword, setKeyword] = useState('');
  const [page, setPage] = useState(1);
  const logs = useQuery({ queryKey: ['logs'], queryFn: () => getData<LogRow[]>('/logs') });
  const filtered = useMemo(() => {
    const term = keyword.trim().toLowerCase();
    return (logs.data ?? []).filter((row) => !term || Object.values(row).join(' ').toLowerCase().includes(term));
  }, [keyword, logs.data]);
  const rows = filtered.slice((page - 1) * 10, page * 10);
  return <div className="page-stack">
    <div className="page-heading"><div><Typography.Title level={3}>操作记录</Typography.Title><Typography.Text type="secondary">查看开通、访问和后台操作。</Typography.Text></div></div>
    {logs.isLoading ? <Skeleton active /> : logs.error ? <Alert type="error" message="操作记录加载失败，请稍后重试。" /> : <Card extra={<Space><Input.Search allowClear placeholder="搜索操作、对象或操作人" value={keyword} onChange={(event) => { setKeyword(event.target.value); setPage(1); }} /><ActionButton tooltip="刷新" icon={<ReloadOutlined />} onClick={() => logs.refetch()} /><ListViewToggle storageKey="starline:list-view:logs" value={viewMode} onChange={setViewMode} /></Space>}>
      {filtered.length === 0 ? <Empty description={keyword ? '没有符合条件的结果' : '还没有操作记录。'} /> : viewMode === 'table' ? <Table rowKey={(row) => String(row.id ?? `${row.time}-${row.action}`)} dataSource={rows} pagination={false} columns={[{ title: '操作人', dataIndex: 'operator' }, { title: '操作', dataIndex: 'action' }, { title: '对象', dataIndex: 'target' }, { title: '时间', dataIndex: 'time' }]} /> : <CardList rows={rows} rowKey={(row) => String(row.id ?? `${row.time}-${row.action}`)} emptyText="还没有操作记录。" renderCard={(row) => <InfoCard title={String(row.action ?? '操作记录')} subtitle={String(row.operator ?? '')} fields={[{ label: '对象', value: String(row.target ?? '-') }, { label: '时间', value: String(row.time ?? '-') }]} />} />}
      {filtered.length > 10 && <Pagination current={page} pageSize={10} total={filtered.length} onChange={setPage} showSizeChanger={false} style={{ marginTop: 16 }} />}
    </Card>}
  </div>;
}
