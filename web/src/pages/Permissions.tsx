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
    <div className="page-stack permissions-page">
      <div>
        <div className="page-heading">
          <div>
            <Typography.Title level={3}>学习权限</Typography.Title>
            <Typography.Text type="secondary">核查学生可见的课程、资料和练习。</Typography.Text>
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
  const emptyText = rows.length === 0 ? '还没有开通记录，开通学习套餐后可在这里核查学习权限。' : '没有符合条件的权限记录';
  const search = (
    <Input.Search className="permissions-search" placeholder="搜索学生、年级或套餐" allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} />
  );
  if (viewMode === 'card') {
    return (
      <Space className="permissions-view" direction="vertical" size="middle">
        {search}
        <CardList
          className="permissions-card-grid"
          rows={filtered}
          rowKey={(record) => record.studentId}
          emptyText={emptyText}
          renderCard={(record) => (
          <InfoCard
            className="permission-card"
            title={record.studentName}
            subtitle={`${record.grade} · 有效期至 ${record.effectiveUntil || '-'}`}
            status={tagStatus(record.permissionState)}
            fields={[
              { label: '账号状态', value: record.accountStatus },
              { label: '已开通套餐', value: permissionTags(record.openedPackages, 'blue', '未绑定套餐'), fullWidth: true },
              { label: '适用课程范围', value: permissionTags(record.learningSpaces, 'cyan', '未开放课程范围') },
              { label: '包含学习内容', value: permissionTags(record.contentTypes, 'geekblue', '未开放学习内容') }
            ]}
            tags={(
              <PermissionOpenGroups
                courses={record.openCourses}
                materials={record.openMaterials}
                homework={record.openHomework}
              />
            )}
          />
          )}
        />
      </Space>
    );
  }
  return (
    <Space className="permissions-view" direction="vertical" size="middle">
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
        { title: '已开通套餐', dataIndex: 'openedPackages', width: 260, render: (values) => tagList(values, 'blue', '未绑定套餐') },
        { title: '适用课程范围', dataIndex: 'learningSpaces', width: 280, render: (values) => tagList(values, 'cyan', '未开放课程范围') },
        { title: '包含学习内容', dataIndex: 'contentTypes', width: 180, render: (values) => tagList(values, 'geekblue', '未开放学习内容') },
        { title: '开放课程', dataIndex: 'openCourses', width: 220, render: (values) => tagList(values, 'green', '未开放课程') },
        { title: '开放资料', dataIndex: 'openMaterials', width: 220, render: (values) => tagList(values, 'purple', '未开放资料') },
        { title: '开放练习', dataIndex: 'openHomework', width: 220, render: (values) => tagList(values, 'orange', '未开放练习') },
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
  const emptyText = rows.length === 0 ? '还没有配置学习套餐，创建套餐后可在这里核查学习权限。' : '没有符合条件的权限记录';
  const search = (
    <Input.Search className="permissions-search" placeholder="搜索套餐或学生" allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} />
  );
  if (viewMode === 'card') {
    return (
      <Space className="permissions-view" direction="vertical" size="middle">
        {search}
        <CardList
          className="permissions-card-grid"
          rows={filtered}
          rowKey={(record) => record.packageId}
          emptyText={emptyText}
          renderCard={(record) => (
          <InfoCard
            className="permission-card"
            title={record.packageName}
            subtitle={`已开通学生 ${record.openedStudents} 人`}
            status={tagStatus(record.status)}
            fields={[
              { label: '已开通学生', value: permissionTags(record.students, 'blue', '还没有学生开通'), fullWidth: true },
              { label: '适用课程范围', value: permissionTags(record.learningSpaces, 'cyan', '未开放课程范围') },
              { label: '包含学习内容', value: permissionTags(record.contentTypes, 'geekblue', '未开放学习内容') }
            ]}
            tags={(
              <PermissionOpenGroups
                courses={record.openCourses}
                materials={record.openMaterials}
                homework={record.openHomework}
              />
            )}
          />
          )}
        />
      </Space>
    );
  }
  return (
    <Space className="permissions-view" direction="vertical" size="middle">
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
        { title: '已开通学生', dataIndex: 'students', width: 180, render: (values) => tagList(values, 'blue', '还没有学生开通') },
        { title: '适用课程范围', dataIndex: 'learningSpaces', width: 280, render: (values) => tagList(values, 'cyan', '未开放课程范围') },
        { title: '包含学习内容', dataIndex: 'contentTypes', width: 180, render: (values) => tagList(values, 'geekblue', '未开放学习内容') },
        { title: '开放课程', dataIndex: 'openCourses', width: 220, render: (values) => tagList(values, 'green', '未开放课程') },
        { title: '开放资料', dataIndex: 'openMaterials', width: 220, render: (values) => tagList(values, 'purple', '未开放资料') },
        { title: '开放练习', dataIndex: 'openHomework', width: 220, render: (values) => tagList(values, 'orange', '未开放练习') }
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
  const emptyText = rows.length === 0 ? '还没有可开放的课程内容，发布课程、资料或练习后可在这里查看。' : '没有符合条件的权限记录';
  const search = (
    <Input.Search className="permissions-search" placeholder="搜索内容、课程或老师" allowClear value={keyword} onChange={(event) => setKeyword(event.target.value)} />
  );
  if (viewMode === 'card') {
    return (
      <Space className="permissions-view" direction="vertical" size="middle">
        {search}
        <CardList
          className="permissions-card-grid"
          rows={filtered}
          rowKey={(record) => `${record.contentType}-${record.contentId}`}
          emptyText={emptyText}
          renderCard={(record) => (
          <InfoCard
            className="permission-card"
            title={record.contentTitle}
            subtitle={`${record.contentType} · ${record.course}`}
            status={tagStatus(record.status)}
            fields={[
              { label: '适用课程范围', value: record.learningSpace },
              { label: '负责老师', value: record.ownerTeacherName || '-' },
              { label: '开放套餐', value: permissionTags(record.openedPackages, 'blue', '未绑定开放套餐'), fullWidth: true },
              { label: '可见学生', value: permissionTags(record.openedStudents, 'green', '还没有可见学生'), fullWidth: true }
            ]}
          />
          )}
        />
      </Space>
    );
  }
  return (
    <Space className="permissions-view" direction="vertical" size="middle">
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
        { title: '开放套餐', dataIndex: 'openedPackages', width: 260, render: (values) => tagList(values, 'blue', '未绑定开放套餐') },
        { title: '可见学生', dataIndex: 'openedStudents', width: 220, render: (values) => tagList(values, 'green', '还没有可见学生') }
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

function PermissionOpenGroups({ courses, materials, homework }: { courses: string[]; materials: string[]; homework: string[] }) {
  return (
    <div className="permission-open-groups">
      <PermissionOpenGroup title="开放课程" values={courses} color="green" emptyText="未开放课程" />
      <PermissionOpenGroup title="开放资料" values={materials} color="purple" emptyText="未开放资料" />
      <PermissionOpenGroup title="开放练习" values={homework} color="orange" emptyText="未开放练习" />
    </div>
  );
}

function PermissionOpenGroup({ title, values, color, emptyText }: { title: string; values: string[]; color: string; emptyText: string }) {
  return (
    <div className="permission-open-group">
      <Typography.Text type="secondary">{title}</Typography.Text>
      {permissionTags(values, color, emptyText)}
    </div>
  );
}

function permissionTags(values: string[] | undefined, color: string, emptyText: string) {
  return <TagGroup className="permission-tag-group" values={values} color={color} emptyText={emptyText} />;
}

function tagList(values: string[], color: string, emptyText: string) {
  return permissionTags(values, color, emptyText);
}

function tagStatus(value: string) {
  const color = value === '生效中' || value === '启用' || value === '已发布' || value === '进行中' ? 'green' : value === '未开始' || value === '草稿' || value === '待提醒' ? 'orange' : 'default';
  return <Tag color={color}>{value || '-'}</Tag>;
}
