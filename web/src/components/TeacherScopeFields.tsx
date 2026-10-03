import { Button, Collapse, Form, Select, Space, Switch, Tag, Typography } from 'antd';
import type { FormInstance } from 'antd';
import type { LearningSpace } from '../types/starline';
import { subjectLabel, subjectsForGrade, subjectsMatch, useSubjectCatalog } from '../utils/curriculum';
import { teachingScopeLabels } from '../utils/teacherScopes';

export function TeacherScopeFields({ form, spaces }: { form: FormInstance; spaces: LearningSpace[] }) {
  const catalog = useSubjectCatalog();
  const exactIds: string[] = Form.useWatch('learningSpaceIds', form) ?? [];
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
    {!!exactIds.length && <Space wrap style={{ marginTop: 12 }}>{teachingScopeLabels(exactIds, spaces).map(label => <Tag key={label}>{label}</Tag>)}<Tag>特殊范围</Tag></Space>}
    <Form.Item name="followTeaching" label="资料查阅跟随授课范围" valuePropName="checked" style={{ marginTop: 16 }}><Switch /></Form.Item>
    <Collapse ghost items={[
      { key: 'reading', label: '额外资料查阅范围', forceRender: true, children: <>
        {scopeRows(['teacherLibrary', 'scopes'], '查阅', true)}
        <Form.Item name={['teacherLibrary', 'spaceIds']} label="指定资料范围" style={{ marginTop: 12 }}><Select mode="multiple" allowClear showSearch optionFilterProp="label" options={spaces.map(item => ({ value: item.id, label: item.name }))} /></Form.Item>
      </> },
      { key: 'exact', label: '指定授课班型与阶段', forceRender: true, children: <Form.Item name="learningSpaceIds" label="精确授课范围" extra="保留历史限制；添加年级学科范围会授权该范围内全部有效班型和阶段。"><Select mode="multiple" allowClear showSearch optionFilterProp="label" options={spaces.map(item => ({ value: item.id, label: item.name, disabled: item.status === '停用' && !exactIds.includes(item.id) }))} /></Form.Item> }
    ]} />
  </>;
}
