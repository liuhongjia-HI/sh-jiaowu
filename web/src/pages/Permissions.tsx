import { Alert, Card, Empty, Input, Skeleton, Space, Table, Tabs, Tag, Typography } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { getData } from '../services/http';
import { CardList, InfoCard, ListViewToggle, TagGroup, useListViewMode } from '../components/ListViews';
import type { ContentPermissionSummary, PackagePermissionSummary, StudentPermissionSummary } from '../types/starline';

export default function Permissions() {
  const [viewMode, setViewMode] = useListViewMode('starline:list-view:permissions');
  const students = useQuery({ queryKey: ['permissions', 'students'], queryFn: () => getData<StudentPermissionSummary[]>('/permissions/students') });
  const packages = useQuery({ queryKey: ['permissions', 'packages'], queryFn: () => getData<PackagePermissionSummary[]>('/permissions/packages') });
  const content = useQuery({ queryKey: ['permissions', 'content'], queryFn: () => getData<ContentPermissionSummary[]>('/permissions/content') });

  if (students.isLoading || packages.isLoading || content.isLoading) return <Skeleton active />;
  if (students.error || packages.error || content.error) return <Alert type="error" message="学习权限数据加载失败，请稍后重试。" />;

  return (
    <div className="page-stack">
      <div>
        <div className="page-heading">
          <div>
            <Typography.Title level={3}>学习权限</Typography.Title>
            <Typography.Text type="secondary">查看学生、学习套餐和课程内容的对应关系，确认学生能看到该看的课程、资料和练习。</Typography.Text>
          </div>
          <div className="page-heading-actions">
            <ListViewToggle storageKey="starline:list-view:permissions" value={viewMode} onChange={setViewMode} />
          </div>
        </div>
      </div>
      <Card>
        <Tabs
          items={[
            { key: 'students', label: '按学生查看', children: <StudentPermissions rows={students.data ?? []} viewMode={viewMode} /> },
            { key: 'packages', label: '按套餐查看', children: <PackagePermissions rows={packages.data ?? []} viewMode={viewMode} /> },
            { key: 'content', label: '按内容查看', children: <ContentPermissions rows={content.data ?? []} viewMode={viewMode} /> }
          ]}
        />
      </Card>
    </div>
  );
}

function StudentPermissions({ rows, viewMode }: { rows: StudentPermissionSummary[]; viewMode: 'card' | 'table' }) {
  const [keyword, setKeyword] = useState('');
  const filtered = useMemo(() => rows.filter((record) => matchKeyword(keyword, [
    record.studentName, record.grade, record.accountStatus, record.permissionState, record.effectiveUntil,
    record.openedPackages, record.learningSpaces, record.contentTypes, record.openCourses, record.openMaterials, record.openHomework
  ])), [rows, keyword]);
  const emptyText = rows.length === 0 ? '还没有开通记录，开通学习套餐后可在这里核查学习权限。' : '没有符合条件的结果';
  const search = (
    <Input.Search placeholder="搜索学生、年级或套餐" allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 280 }} />
  );
  if (viewMode === 'card') {
    return (
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        {search}
        <CardList
          rows={filtered}
          rowKey={(record) => record.studentId}
          emptyText={emptyText}
          renderCard={(record) => (
          <InfoCard
            title={record.studentName}
            subtitle={`${record.grade} · 有效期至 ${record.effectiveUntil || '-'}`}
            status={tagStatus(record.permissionState)}
            fields={[
              { label: '账号状态', value: record.accountStatus },
              { label: '已开通套餐', value: <TagGroup values={record.openedPackages} color="blue" /> },
              { label: '适用课程范围', value: <TagGroup values={record.learningSpaces} color="cyan" /> },
              { label: '包含学习内容', value: <TagGroup values={record.contentTypes} color="geekblue" /> }
            ]}
            tags={(
              <Space direction="vertical" size={6} style={{ width: '100%' }}>
                <TagGroup values={record.openCourses} color="green" emptyText="暂无开放课程" />
                <TagGroup values={record.openMaterials} color="purple" emptyText="暂无开放资料" />
                <TagGroup values={record.openHomework} color="orange" emptyText="暂无开放练习" />
              </Space>
            )}
          />
          )}
        />
      </Space>
    );
  }
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {search}
      {filtered.length === 0 ? <Empty description={emptyText} /> : (
      <Table
        rowKey="studentId"
        dataSource={filtered}
        pagination={{ pageSize: 10 }}
        scroll={{ x: 1100 }}
        columns={[
          { title: '学生', dataIndex: 'studentName', fixed: 'left', width: 120 },
        { title: '年级', dataIndex: 'grade', width: 100 },
        { title: '状态', dataIndex: 'permissionState', width: 100, render: tagStatus },
        { title: '已开通套餐', dataIndex: 'openedPackages', width: 260, render: (values) => tagList(values, 'blue') },
        { title: '适用课程范围', dataIndex: 'learningSpaces', width: 280, render: (values) => tagList(values, 'cyan') },
        { title: '包含学习内容', dataIndex: 'contentTypes', width: 180, render: (values) => tagList(values, 'geekblue') },
        { title: '开放课程', dataIndex: 'openCourses', width: 220, render: (values) => tagList(values, 'green') },
        { title: '开放资料', dataIndex: 'openMaterials', width: 220, render: (values) => tagList(values, 'purple') },
        { title: '开放练习', dataIndex: 'openHomework', width: 220, render: (values) => tagList(values, 'orange') },
        { title: '有效期至', dataIndex: 'effectiveUntil', width: 120, render: (value) => value || '-' }
        ]}
      />
      )}
    </Space>
  );
}

function PackagePermissions({ rows, viewMode }: { rows: PackagePermissionSummary[]; viewMode: 'card' | 'table' }) {
  const [keyword, setKeyword] = useState('');
  const filtered = useMemo(() => rows.filter((record) => matchKeyword(keyword, [
    record.packageName, record.status, record.students, record.learningSpaces,
    record.contentTypes, record.openCourses, record.openMaterials, record.openHomework
  ])), [rows, keyword]);
  const emptyText = rows.length === 0 ? '还没有配置学习套餐，创建套餐后可在这里核查学习权限。' : '没有符合条件的结果';
  const search = (
    <Input.Search placeholder="搜索套餐或学生" allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 280 }} />
  );
  if (viewMode === 'card') {
    return (
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        {search}
        <CardList
          rows={filtered}
          rowKey={(record) => record.packageId}
          emptyText={emptyText}
          renderCard={(record) => (
          <InfoCard
            title={record.packageName}
            subtitle={`已开通学生 ${record.openedStudents} 人`}
            status={tagStatus(record.status)}
            fields={[
              { label: '已开通学生', value: <TagGroup values={record.students} color="blue" /> },
              { label: '适用课程范围', value: <TagGroup values={record.learningSpaces} color="cyan" /> },
              { label: '包含学习内容', value: <TagGroup values={record.contentTypes} color="geekblue" /> },
              { label: '开放课程', value: <TagGroup values={record.openCourses} color="green" /> }
            ]}
            tags={(
              <Space direction="vertical" size={6} style={{ width: '100%' }}>
                <TagGroup values={record.openMaterials} color="purple" emptyText="暂无开放资料" />
                <TagGroup values={record.openHomework} color="orange" emptyText="暂无开放练习" />
              </Space>
            )}
          />
          )}
        />
      </Space>
    );
  }
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {search}
      {filtered.length === 0 ? <Empty description={emptyText} /> : (
      <Table
        rowKey="packageId"
        dataSource={filtered}
        pagination={{ pageSize: 10 }}
        scroll={{ x: 1100 }}
        columns={[
          { title: '套餐', dataIndex: 'packageName', fixed: 'left', width: 260 },
        { title: '状态', dataIndex: 'status', width: 100, render: tagStatus },
        { title: '开通学生数', dataIndex: 'openedStudents', width: 110 },
        { title: '已开通学生', dataIndex: 'students', width: 180, render: (values) => tagList(values, 'blue') },
        { title: '适用课程范围', dataIndex: 'learningSpaces', width: 280, render: (values) => tagList(values, 'cyan') },
        { title: '包含学习内容', dataIndex: 'contentTypes', width: 180, render: (values) => tagList(values, 'geekblue') },
        { title: '开放课程', dataIndex: 'openCourses', width: 220, render: (values) => tagList(values, 'green') },
        { title: '开放资料', dataIndex: 'openMaterials', width: 220, render: (values) => tagList(values, 'purple') },
        { title: '开放练习', dataIndex: 'openHomework', width: 220, render: (values) => tagList(values, 'orange') }
        ]}
      />
      )}
    </Space>
  );
}

function ContentPermissions({ rows, viewMode }: { rows: ContentPermissionSummary[]; viewMode: 'card' | 'table' }) {
  const [keyword, setKeyword] = useState('');
  const filtered = useMemo(() => rows.filter((record) => matchKeyword(keyword, [
    record.contentTitle, record.contentType, record.course, record.learningSpace,
    record.ownerTeacherName, record.status, record.openedPackages, record.openedStudents
  ])), [rows, keyword]);
  const emptyText = rows.length === 0 ? '还没有可开放的课程内容，发布课程、资料或练习后可在这里查看。' : '没有符合条件的结果';
  const search = (
    <Input.Search placeholder="搜索内容、课程或老师" allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} style={{ width: 280 }} />
  );
  if (viewMode === 'card') {
    return (
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        {search}
        <CardList
          rows={filtered}
          rowKey={(record) => `${record.contentType}-${record.contentId}`}
          emptyText={emptyText}
          renderCard={(record) => (
          <InfoCard
            title={record.contentTitle}
            subtitle={`${record.contentType} · ${record.course}`}
            status={tagStatus(record.status)}
            fields={[
              { label: '适用课程范围', value: record.learningSpace },
              { label: '负责老师', value: record.ownerTeacherName || '-' },
              { label: '开放套餐', value: <TagGroup values={record.openedPackages} color="blue" /> },
              { label: '可见学生', value: <TagGroup values={record.openedStudents} color="green" /> }
            ]}
          />
          )}
        />
      </Space>
    );
  }
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {search}
      {filtered.length === 0 ? <Empty description={emptyText} /> : (
      <Table
        rowKey={(record) => `${record.contentType}-${record.contentId}`}
        dataSource={filtered}
        pagination={{ pageSize: 10 }}
        scroll={{ x: 1000 }}
        columns={[
          { title: '内容', dataIndex: 'contentTitle', fixed: 'left', width: 220 },
        { title: '类型', dataIndex: 'contentType', width: 100, render: (value) => <Tag>{value}</Tag> },
        { title: '所属课程', dataIndex: 'course', width: 180 },
        { title: '适用课程范围', dataIndex: 'learningSpace', width: 260 },
        { title: '负责老师', dataIndex: 'ownerTeacherName', width: 120, render: (value) => value || <Typography.Text type="secondary">-</Typography.Text> },
        { title: '状态', dataIndex: 'status', width: 100, render: tagStatus },
        { title: '开放套餐', dataIndex: 'openedPackages', width: 260, render: (values) => tagList(values, 'blue') },
        { title: '可见学生', dataIndex: 'openedStudents', width: 220, render: (values) => tagList(values, 'green') }
        ]}
      />
      )}
    </Space>
  );
}

// 把若干字段（字符串或字符串数组）拼成一段文本，做关键字包含匹配；关键字为空时全部命中。
function matchKeyword(keyword: string, parts: (string | string[] | undefined)[]) {
  const kw = keyword.trim().toLowerCase();
  if (!kw) return true;
  return parts
    .flat()
    .filter(Boolean)
    .join(' ')
    .toLowerCase()
    .includes(kw);
}

function tagList(values: string[], color: string) {
  if (!values || values.length === 0) return <Typography.Text type="secondary">无</Typography.Text>;
  return (
    <Space size={[4, 4]} wrap>
      {values.map((value) => <Tag key={value} color={color}>{value}</Tag>)}
    </Space>
  );
}

function tagStatus(value: string) {
  const color = value === '生效中' || value === '启用' || value === '已发布' || value === '进行中' ? 'green' : value === '草稿' || value === '待提醒' ? 'orange' : 'default';
  return <Tag color={color}>{value || '-'}</Tag>;
}
