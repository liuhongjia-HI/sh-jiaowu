import { Button, Collapse, Form, Select, Space, Switch, Tag, Typography } from 'antd';
import type { FormInstance } from 'antd';
import type { LearningSpace } from '../types/starline';
import { gradeIndex, phaseLabel, semesterLabel, subjectLabel, subjectsForGrade, subjectsMatch, useSubjectCatalog } from '../utils/curriculum';

const present = (value?: string) => value?.trim() || '未设置';
const spaceLabel = (item: LearningSpace) => `${present(item.grade)} · ${present(subjectLabel(item.subject))} · ${present(semesterLabel(item.semester))} · ${present(phaseLabel(item.phase))} · 班型：${present(item.level)} · ${present(item.name)}`;
const compare = (a: string, b: string) => a.localeCompare(b, 'zh-CN', { numeric: true });

function ScopeSummary({ ids, spaces }: { ids: string[]; spaces: LearningSpace[] }) {
  const byId = new Map(spaces.map(item => [item.id, item]));
  return <details style={{ marginTop: 8 }}>
    <summary style={{ cursor: 'pointer' }}>已选 {ids.length} 项</summary>
    {ids.length > 0 && <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 8 }}>
      {ids.map(id => <Tag key={id} style={{ whiteSpace: 'normal', overflowWrap: 'anywhere', marginInlineEnd: 0 }}>{byId.has(id) ? spaceLabel(byId.get(id)!) : `历史范围(${id})`}</Tag>)}
    </div>}
  </details>;
}

export function TeacherScopeFields({ form, spaces }: { form: FormInstance; spaces: LearningSpace[] }) {
  const catalog = useSubjectCatalog();
  const exactIds: string[] = Form.useWatch('learningSpaceIds', form) ?? [];
  const readingIds: string[] = Form.useWatch(['teacherLibrary', 'spaceIds'], form) ?? [];
  const filterSpace = (input: string, option?: { value?: string; label?: React.ReactNode }) => {
    const item = spaces.find(space => space.id === option?.value);
    return [option?.label, item?.grade, item?.subject, item?.semester, item?.phase, item?.level, item?.name].some(value => String(value ?? '').toLowerCase().includes(input.toLowerCase()));
  };
  const renderSpace = (option: { value?: string | number; label?: React.ReactNode }) => {
    const item = spaces.find(space => space.id === option.value);
    if (!item) return <div style={{ whiteSpace: 'normal', overflowWrap: 'anywhere' }}>{option.label}</div>;
    return <div aria-label={spaceLabel(item)} style={{ whiteSpace: 'normal', overflowWrap: 'anywhere' }}>
      <div>{present(item.name)} {item.status === '停用' && <Tag>停用</Tag>}</div>
      <div style={{ marginTop: 4 }}><Tag color={item.semester === 'S1' ? 'blue' : item.semester === 'S2' ? 'purple' : item.semester === 'S3' ? 'cyan' : 'default'}>{present(semesterLabel(item.semester))}</Tag><Typography.Text type="secondary">{present(phaseLabel(item.phase))} · 班型：{present(item.level)}</Typography.Text></div>
    </div>;
  };
  const groupedOptions = (selectedIds: string[], exact: boolean) => {
    const groups = new Map<string, { grade: string; subject: string; options: { value: string; label: string; disabled?: boolean }[] }>();
    for (const item of spaces) {
      const grade = present(item.grade);
      const subject = present(subjectLabel(item.subject));
      const key = JSON.stringify([grade, subject]);
      const group = groups.get(key) ?? { grade, subject, options: [] };
      group.options.push({ value: item.id, label: spaceLabel(item), disabled: exact && item.status === '停用' && !selectedIds.includes(item.id) });
      groups.set(key, group);
    }
    const knownIds = new Set(spaces.map(item => item.id));
    const historical = selectedIds.filter(id => !knownIds.has(id)).map(id => ({ value: id, label: `历史范围(${id})` }));
    const result = [...groups.values()].sort((a, b) => {
      const ai = gradeIndex(a.grade), bi = gradeIndex(b.grade);
      return (ai < 0 ? Infinity : ai) - (bi < 0 ? Infinity : bi) || compare(a.grade, b.grade) || compare(a.subject, b.subject);
    }).map(group => ({ label: `${group.grade} · ${group.subject}`, options: group.options.sort((a, b) => compare(a.label, b.label) || compare(a.value, b.value)) }));
    if (historical.length) result.push({ label: '历史范围', options: historical });
    return result;
  };
  const grades = [...new Set(spaces.filter(item => item.status !== '停用').map(item => item.grade))].map(value => ({ value, label: value }));
  const activeSubjects = (grade?: string) => [...new Set(spaces.filter(item => item.status !== '停用' && (!grade || item.grade === grade) && subjectsForGrade(grade, catalog).some(subject => subjectsMatch(subject, item.subject))).map(item => item.subject))];
  const scopeRows = (path: string | string[], label: string, allGrades: boolean) => <Form.List name={path}>
    {(fields, { add, remove }) => <Space direction="vertical" style={{ width: '100%' }}>
      {fields.map(({ key, name, ...rest }) => <Form.Item key={key} noStyle shouldUpdate>
        {() => {
          const parts = Array.isArray(path) ? path : [path];
          const grade = form.getFieldValue([...parts, name, 'grade']);
          const current = form.getFieldValue([...parts, name, 'subject']);
          const valid = activeSubjects(grade);
          const subjects = [...valid, ...(current && !valid.includes(current) ? [current] : [])];
          return <Space wrap align="baseline">
            <Form.Item {...rest} name={[name, 'grade']} rules={allGrades ? [] : [{ required: true, message: '请选择年级' }]}>
              <Select aria-label={`${label}年级`} placeholder="年级" style={{ width: 140 }} options={[...(allGrades ? [{ value: '', label: '全部年级' }] : []), ...grades]} onChange={(value) => {
                if (current && !activeSubjects(value).some(subject => subjectsMatch(subject, current))) form.setFieldValue([...parts, name, 'subject'], undefined);
              }} />
            </Form.Item>
            <Form.Item {...rest} name={[name, 'subject']} rules={[{ required: true, message: '请选择学科' }]}>
              <Select aria-label={`${label}学科`} placeholder="学科" style={{ width: 170 }} options={subjects.map(value => ({ value, label: `${subjectLabel(value)}${valid.includes(value) ? '' : '（历史配置）'}`, disabled: !valid.includes(value) }))} />
            </Form.Item>
            <Button type="link" danger onClick={() => remove(name)}>移除</Button>
          </Space>;
        }}
      </Form.Item>)}
      <Button onClick={() => add({ grade: '', subject: undefined })}>添加{label}范围</Button>
    </Space>}
  </Form.List>;

  return <>
    <Typography.Title level={5}>授课范围</Typography.Title>
    {scopeRows('teachingScopes', '授课', false)}
    <Form.Item name="followTeaching" label="资料查阅跟随授课范围" valuePropName="checked" style={{ marginTop: 16 }}><Switch /></Form.Item>
    <Collapse ghost items={[
      { key: 'reading', label: `额外资料查阅范围（指定 ${readingIds.length} 项）`, forceRender: true, children: <>
        {scopeRows(['teacherLibrary', 'scopes'], '查阅', true)}
        <Form.Item name={['teacherLibrary', 'spaceIds']} label="指定资料范围" style={{ marginTop: 12 }}><Select aria-label="指定资料范围" mode="multiple" allowClear showSearch filterOption={filterSpace} virtual={false} listHeight={360} optionRender={renderSpace} maxTagCount="responsive" maxTagTextLength={22} options={groupedOptions(readingIds, false)} /></Form.Item>
        <ScopeSummary ids={readingIds} spaces={spaces} />
      </> },
      { key: 'exact', label: `指定授课班型与阶段（已选 ${exactIds.length} 项）`, forceRender: true, children: <><Form.Item name="learningSpaceIds" label="精确授课范围" extra="保留历史限制；添加年级学科范围会授权该范围内全部有效班型和阶段。"><Select aria-label="精确授课范围" mode="multiple" allowClear showSearch filterOption={filterSpace} virtual={false} listHeight={360} optionRender={renderSpace} maxTagCount="responsive" maxTagTextLength={22} options={groupedOptions(exactIds, true)} /></Form.Item><ScopeSummary ids={exactIds} spaces={spaces} /></> }
    ]} />
  </>;
}
