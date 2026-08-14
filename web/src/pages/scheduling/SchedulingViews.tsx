import { CalendarOutlined, CloseCircleOutlined, DeleteOutlined, EditOutlined, LeftOutlined, PlusOutlined, ReloadOutlined, RightOutlined, SaveOutlined, SettingOutlined, TableOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Form, Input, InputNumber, Modal, Popconfirm, Segmented, Select, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { gradeOptions, subjectOptions } from '../../utils/curriculum';
import type { AvailabilitySlot, Course, CurrentUser, ScheduleCandidate, ScheduleClass, Student, Teacher } from '../../types/starline';
import {
  addDays,
  addMonths,
  candidateLevel,
  candidateLevelMeta,
  classCapacity,
  formatWeekRange,
  localDateText,
  minimumStudentCount,
  scheduleClassOccursOn,
  startOfMonth,
  startOfWeek,
  weekLabel,
  weekOptions
} from './scheduling-utils';
import { CandidatePanel, CoordinationPanel } from './CandidatePanel';

type CandidateFormValues = {
  subject: string;
  grade: string;
  classType: string;
  durationMinutes: number;
  startDate: string;
  endDate: string;
};

type AvailabilityFormValues = {
  ownerKey: string;
  slots: AvailabilitySlot[];
};

type ScheduleClassFormValues = {
  courseId: string;
  teacherId: string;
  campusId: string;
  roomName: string;
  classType: string;
  durationMinutes: number;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  startDate: string;
  endDate: string;
  studentIds: string[];
};

type ScheduleFilters = {
  grade?: string;
  subject?: string;
  teacherId?: string;
  studentId?: string;
  campusId?: string;
  courseId?: string;
  classType?: string;
  status?: string;
};

type ScheduleMoveTarget = {
  dayOfWeek: number;
  label: string;
  startTime?: string;
  endTime?: string;
  startDate?: string;
  endDate?: string;
};

type CourseLookup = Record<string, Course>;
type TimelineKind = 'class' | 'candidate' | 'availability';
type TimelineItem = {
  id: string;
  kind: TimelineKind;
  dayOfWeek: number;
  startTime: string;
  endTime: string;
  subject: string;
  title: string;
  subtitle: string;
  meta: string;
  status?: string;
  classType?: string;
  countText?: string;
  record: ScheduleClass | ScheduleCandidate | AvailabilitySlot;
};
type TimelineLayoutItem = TimelineItem & {
  column: number;
  columns: number;
  leftPct?: number;
  widthPct?: number;
};
type WeekDay = {
  key: string;
  date: Date;
  day: number;
  dayOfWeek: number;
  weekLabel: string;
  label: string;
};

const timelineSlotMinutes = 30;
const timelineSlotHeight = 44;
const defaultTimelineStart = 14 * 60;
const defaultTimelineEnd = 22 * 60;

export function ScheduleWeekTimeline({
  loading,
  weekDays,
  selectedWeekStart,
  availabilityByDay,
  candidatesByDay,
  classesByDay,
  courseById,
  teacherById,
  studentById,
  candidateRequest,
  selectedCandidateId,
  emptyTips,
  canManage,
  onPreviousWeek,
  onNextWeek,
  onToday,
  onPickCandidate,
  onEditClass,
  onMoveClass,
  onCreateClass
}: {
  loading: boolean;
  weekDays: WeekDay[];
  selectedWeekStart: Date;
  availabilityByDay: Record<number, AvailabilitySlot[]>;
  candidatesByDay: Record<number, ScheduleCandidate[]>;
  classesByDay: Record<number, ScheduleClass[]>;
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  studentById: Record<string, Student>;
  candidateRequest: CandidateFormValues | null;
  selectedCandidateId?: string;
  emptyTips: string[];
  canManage: boolean;
  onPreviousWeek: () => void;
  onNextWeek: () => void;
  onToday: () => void;
  onPickCandidate: (record: ScheduleCandidate) => void;
  onEditClass: (record: ScheduleClass) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
  onCreateClass: (dayOfWeek: number) => void;
}) {
  const timelineItems = useMemo(() => buildTimelineItems(weekDays, availabilityByDay, candidatesByDay, classesByDay, courseById, teacherById, studentById), [weekDays, availabilityByDay, candidatesByDay, classesByDay, courseById, teacherById, studentById]);
  const hasAnyItem = timelineItems.length > 0;
  const timelineRange = useMemo(() => buildTimelineRange(timelineItems), [timelineItems]);
  const rows = useMemo(() => buildTimelineRows(timelineRange.start, timelineRange.end), [timelineRange]);
  const boardHeight = ((timelineRange.end - timelineRange.start) / timelineSlotMinutes) * timelineSlotHeight;
  const itemsByDay = useMemo(() => {
    return weekDays.reduce<Record<number, TimelineLayoutItem[]>>((result, day) => {
      result[day.dayOfWeek] = layoutOverlappingItems(timelineItems.filter((item) => item.dayOfWeek === day.dayOfWeek));
      return result;
    }, {});
  }, [weekDays, timelineItems]);

  if (loading) return <Skeleton active paragraph={{ rows: 6 }} />;

  return (
    <div className="schedule-timeline-wrap">
      {!hasAnyItem && (
        <ScheduleEmptyTips
          description={candidateRequest ? '暂时没有可展示的排课结果。' : '选择课程和老师后，查找可排时间。'}
          tips={emptyTips}
          compact
        />
      )}
      {candidateRequest && !hasGroupedItems(candidatesByDay) && (
        <ScheduleEmptyTips description="没有找到推荐时间。" tips={emptyTips} compact />
      )}
      <div className="schedule-timeline-toolbar">
        <Space size={8}>
          <Button icon={<LeftOutlined />} onClick={onPreviousWeek} />
          <Button onClick={onToday}>今天</Button>
          <Button icon={<RightOutlined />} onClick={onNextWeek} />
        </Space>
        <div>
          <Typography.Title level={4}>{formatWeekRange(selectedWeekStart)}</Typography.Title>
          <Typography.Text type="secondary">拖动已确认课程到目标时间，松开后确认保存；点击课程可快捷调整学生。</Typography.Text>
        </div>
      </div>

      <div className="schedule-timeline-scroll">
        <div className="schedule-timeline-grid" style={{ '--timeline-height': `${boardHeight}px` } as CSSProperties}>
          <div className="schedule-time-gutter schedule-day-head-spacer" />
          {weekDays.map((day) => (
            <div className="schedule-day-head" key={day.key}>
              <strong>{day.day}</strong>
              <span>{day.weekLabel}</span>
              <small>{(classesByDay[day.dayOfWeek] ?? []).filter((item) => item.status !== '已取消').length} 节课</small>
            </div>
          ))}
          <div className="schedule-time-gutter schedule-time-axis" style={{ height: boardHeight }}>
            {rows.map((row) => (
              <div className="schedule-time-label" key={row.minute} style={{ top: row.top }}>
                {row.label}
              </div>
            ))}
          </div>
          {weekDays.map((day) => {
            const dayItems = itemsByDay[day.dayOfWeek] ?? [];
            return (
              <div
                className="schedule-day-column"
                key={day.key}
                style={{ height: boardHeight }}
                onDragOver={(event) => canManage ? event.preventDefault() : undefined}
                onDrop={(event) => {
                  const classID = event.dataTransfer.getData('text/schedule-class-id');
                  const record = Object.values(classesByDay).flat().find((item) => item.id === classID);
                  if (!record) return;
                  const bounds = event.currentTarget.getBoundingClientRect();
                  const offsetMinutes = Math.round((event.clientY - bounds.top) / timelineSlotHeight) * timelineSlotMinutes;
                  const startMinute = clampTimelineStart(timelineRange.start + offsetMinutes, record.durationMinutes);
                  const startTime = formatMinute(startMinute);
                  const endTime = formatMinute(startMinute + record.durationMinutes);
                  onMoveClass(record, {
                    dayOfWeek: day.dayOfWeek,
                    startTime,
                    endTime,
                    label: `${day.label} ${startTime}-${endTime}`
                  });
                }}
                onDoubleClick={(event) => {
                  if (event.currentTarget === event.target && canManage) onCreateClass(day.dayOfWeek);
                }}
              >
                {rows.map((row) => <span className="schedule-time-line" key={row.minute} style={{ top: row.top }} />)}
                {dayItems.length === 0 && (
                  <button type="button" className="schedule-day-empty-slot" onClick={() => canManage ? onCreateClass(day.dayOfWeek) : undefined}>
                    {canManage ? '新建课程' : '暂无安排'}
                  </button>
                )}
                {dayItems.map((item) => (
                  <TimelineBlock
                    key={`${item.kind}-${item.id}`}
                    item={item}
                    rangeStart={timelineRange.start}
                    selectedCandidateId={selectedCandidateId}
                    canManage={canManage}
                    onPickCandidate={onPickCandidate}
                    onEditClass={onEditClass}
                  />
                ))}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

export function ScheduleEmptyTips({ description, tips, compact = false }: { description: string; tips: string[]; compact?: boolean }) {
  return (
    <div className={compact ? 'schedule-empty-tips compact' : 'schedule-empty-tips'}>
      <Empty description={description} />
      {tips.length > 0 && (
        <div className="schedule-tip-list">
          {tips.map((tip) => <div key={tip}>{tip}</div>)}
        </div>
      )}
    </div>
  );
}

export function MiniMonthCalendar({ month, selectedWeekStart, onPickDate }: { month: Date; selectedWeekStart: Date; onPickDate: (date: Date) => void }) {
  const days = useMemo(() => buildMiniMonthDays(month), [month]);
  const selectedKey = localDateText(selectedWeekStart);
  return (
    <div className="schedule-mini-calendar">
      {['一', '二', '三', '四', '五', '六', '日'].map((item) => <span className="schedule-mini-week" key={item}>{item}</span>)}
      {days.map((day) => {
        const weekSelected = localDateText(startOfWeek(day.date)) === selectedKey;
        return (
          <button
            type="button"
            className={`schedule-mini-day ${day.inMonth ? '' : 'is-outside'} ${weekSelected ? 'is-week-selected' : ''} ${day.isToday ? 'is-today' : ''}`}
            key={day.key}
            onClick={() => onPickDate(day.date)}
          >
            {day.date.getDate()}
          </button>
        );
      })}
    </div>
  );
}

export function TimelineBlock({
  item,
  rangeStart,
  selectedCandidateId,
  canManage,
  onPickCandidate,
  onEditClass
}: {
  item: TimelineLayoutItem;
  rangeStart: number;
  selectedCandidateId?: string;
  canManage: boolean;
  onPickCandidate: (record: ScheduleCandidate) => void;
  onEditClass: (record: ScheduleClass) => void;
}) {
  const start = timeToMinutes(item.startTime);
  const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
  const color = subjectColor(item.subject);
  const top = ((start - rangeStart) / timelineSlotMinutes) * timelineSlotHeight;
  const height = Math.max(34, ((end - start) / timelineSlotMinutes) * timelineSlotHeight - 4);
  const widthPct = item.widthPct ?? (100 / item.columns);
  const leftPct = item.leftPct ?? ((100 / item.columns) * item.column);
  const width = `calc(${widthPct}% - 4px)`;
  const left = `calc(${leftPct}% + 2px)`;
  const style = {
    top,
    height,
    left,
    width,
    '--subject-bg': color.bg,
    '--subject-border': color.border,
    '--subject-accent': color.accent,
    '--subject-text': color.text
  } as CSSProperties;
  const title = `${item.startTime}-${item.endTime} ${item.title} ${item.subtitle}`;
  const className = [
    'schedule-timeline-block',
    `is-${item.kind}`,
    item.status === '已取消' ? 'is-canceled' : '',
    item.kind === 'candidate' && item.id === selectedCandidateId ? 'is-selected' : ''
  ].filter(Boolean).join(' ');
  const content = (
    <>
      <div className="schedule-timeline-time">{item.startTime}-{item.endTime}</div>
      <strong>{item.title}</strong>
      <span>{item.subtitle}</span>
      <small>{item.meta}</small>
      <div className="schedule-timeline-tags">
        {item.classType && <Tag>{item.classType}</Tag>}
        {item.countText && <Tag>{item.countText}</Tag>}
        {item.status && <Tag color={item.status === '已取消' ? 'default' : 'green'}>{item.status}</Tag>}
      </div>
    </>
  );

  if (item.kind === 'class') {
    const record = item.record as ScheduleClass;
    return (
      <button
        type="button"
        className={className}
        style={style}
        title={title}
        draggable={canManage && record.status !== '已取消'}
        onDragStart={(event) => event.dataTransfer.setData('text/schedule-class-id', record.id)}
        onClick={() => canManage && record.status !== '已取消' ? onEditClass(record) : undefined}
      >
        {content}
      </button>
    );
  }

  if (item.kind === 'candidate') {
    return (
      <button
        type="button"
        className={className}
        style={style}
        title={title}
        onClick={() => onPickCandidate(item.record as ScheduleCandidate)}
      >
        {content}
        <span className="schedule-timeline-action">确认这个时间</span>
      </button>
    );
  }

  return (
    <div className={className} style={style} title={title}>
      {content}
    </div>
  );
}

export function MonthScheduleBoard({
  classes,
  courseById,
  teacherById,
  canManage,
  onEditClass,
  onMoveClass
}: {
  classes: ScheduleClass[];
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  canManage: boolean;
  onEditClass: (record: ScheduleClass) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
}) {
  const days = useMemo(() => buildMonthDays(new Date()), []);
  if (classes.length === 0) return <Empty description="还没有可展示的课程。" />;
  return (
    <div className="month-board">
      {days.map((day) => {
        const dayClasses = classes.filter((item) => scheduleClassOccursOn(item, day.date));
        return (
          <div
            className="month-day"
            key={day.key}
            onDragOver={(event) => canManage ? event.preventDefault() : undefined}
            onDrop={(event) => {
              const classID = event.dataTransfer.getData('text/schedule-class-id');
              const record = classes.find((item) => item.id === classID);
              if (record) onMoveClass(record, {
                dayOfWeek: day.dayOfWeek,
                label: day.label,
                startDate: day.key,
                endDate: day.key
              });
            }}
          >
            <div className="month-day-head">
              <strong>{day.day}</strong>
              <span>{day.weekLabel}</span>
            </div>
            <div className="month-day-body">
              {dayClasses.length === 0 ? (
                <span className="month-day-empty">暂无课程</span>
              ) : sortByStartTime(dayClasses).map((item) => {
                const course = courseById[item.courseId];
                return (
                  <button
                    type="button"
                    className={`month-class ${item.status === '已取消' ? 'is-canceled' : ''}`}
                    key={item.id}
                    draggable={canManage && item.status !== '已取消'}
                    onDragStart={(event) => event.dataTransfer.setData('text/schedule-class-id', item.id)}
                    onClick={() => canManage && item.status !== '已取消' ? onEditClass(item) : undefined}
                  >
                    <span>{item.startTime}</span>
                    <strong>{courseSubjectGradeText(course, item.courseName)}</strong>
                    <small>{teacherDisplay(item.teacherName, course, teacherById[item.teacherId])}</small>
                  </button>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function parseOwnerKey(value?: string) {
  if (!value) return null;
  const [ownerType, ownerId] = value.split(':');
  if ((ownerType !== 'teacher' && ownerType !== 'student') || !ownerId) return null;
  return { ownerType, ownerId };
}

export function classColumns(courseById: CourseLookup, teacherById: Record<string, Teacher>, canManage: boolean, onEdit: (record: ScheduleClass) => void, onCancel: (id: string) => void, canceling: boolean): TableColumnsType<ScheduleClass> {
  const columns: TableColumnsType<ScheduleClass> = [
    { title: '班级', dataIndex: 'name', width: 160 },
    { title: '科目/年级', width: 130, render: (_, record) => courseSubjectGradeText(courseById[record.courseId], record.courseName) },
    { title: '课程', dataIndex: 'courseName', ellipsis: true },
    { title: '老师', width: 220, render: (_, record) => teacherDisplay(record.teacherName, courseById[record.courseId], teacherById[record.teacherId]) },
    { title: '时间', width: 160, render: (_, record) => `${weekLabel(record.dayOfWeek)} ${record.startTime}-${record.endTime}` },
    { title: '班型', dataIndex: 'classType', width: 90 },
    { title: '学生', render: (_, record) => tagList(record.students.map(studentDisplayName), 'blue') },
    { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={value === '已取消' ? 'default' : 'green'}>{value}</Tag> }
  ];
  if (canManage) {
    columns.push({
      title: '操作',
      width: 92,
      render: (_, record) => record.status === '已取消' ? <Typography.Text type="secondary">-</Typography.Text> : (
        <Space size={4}>
          <ActionButton tooltip="调课" icon={<EditOutlined />} onClick={() => onEdit(record)} />
          <Popconfirm title="取消这节课？" description="取消后该时间不再占用，可重新排课。" okText="取消课程" cancelText="保留" onConfirm={() => onCancel(record.id)}>
            <ActionButton danger tooltip="取消课程" icon={<CloseCircleOutlined />} loading={canceling} />
          </Popconfirm>
        </Space>
      )
    });
  }
  return columns;
}

export function tagList(values: string[], color: string) {
  if (values.length === 0) return <Typography.Text type="secondary">-</Typography.Text>;
  return <Space size={[4, 4]} wrap>{values.map((value) => <Tag color={color} key={value}>{value}</Tag>)}</Space>;
}

export function courseSubjectGradeText(course: Course | undefined, fallbackName: string) {
  if (!course) return fallbackName;
  return [course.subject, course.grade].filter(Boolean).join(' · ') || course.name || fallbackName;
}

export function teacherOptionLabel(teacher: Teacher) {
  const scope = teacherScopeText(teacher);
  return scope ? `${teacher.name} · ${scope}` : teacher.name;
}

export function teacherDisplay(teacherName: string, course: Course | undefined, teacher?: Teacher) {
  const scope = teacher ? teacherScopeText(teacher) : '';
  if (scope) return `教师：${teacherName} · ${scope}`;
  return course?.grade ? `教师：${teacherName} · ${course.grade}` : `教师：${teacherName}`;
}

export function teacherScopeText(teacher: Teacher) {
  const grades = shortList(teacher.grades ?? []);
  const subjects = shortList(teacher.subjects ?? []);
  if (grades && subjects) return `${grades}/${subjects}`;
  return grades || subjects;
}

export function availabilityOwnerDisplayName(slot: AvailabilitySlot, teacherById: Record<string, Teacher>, studentById: Record<string, Student>) {
  if (slot.ownerType === 'teacher') {
    const teacher = teacherById[slot.ownerId];
    return teacher ? teacherOptionLabel(teacher) : slot.ownerName;
  }
  const student = studentById[slot.ownerId];
  return student ? studentDisplayName(student) : slot.ownerName;
}

export function studentOptionLabel(student: { name: string; grade?: string; openedPackages?: string[] }) {
  const base = studentDisplayName(student);
  if (!student.openedPackages || student.openedPackages.length === 0) return base;
  return `${base} · ${shortList(student.openedPackages, 1)}`;
}

export function studentDisplayName(student: { name: string; grade?: string }) {
  return student.grade ? `${student.name}（${student.grade}）` : student.name;
}

export function studentDisplayNames(students: { name: string; grade?: string }[]) {
  if (students.length === 0) return '暂无学生';
  const values = students.map(studentDisplayName);
  if (values.length <= 3) return values.join('、');
  return `${values.slice(0, 3).join('、')} 等 ${values.length} 人`;
}

export function buildTimelineItems(
  weekDays: WeekDay[],
  availabilityByDay: Record<number, AvailabilitySlot[]>,
  candidatesByDay: Record<number, ScheduleCandidate[]>,
  classesByDay: Record<number, ScheduleClass[]>,
  courseById: CourseLookup,
  teacherById: Record<string, Teacher>,
  studentById: Record<string, Student>
) {
  return weekDays.flatMap<TimelineItem>((day) => {
    const availabilityItems = aggregateAvailabilitySlots(availabilityByDay[day.dayOfWeek] ?? [], teacherById, studentById).map<TimelineItem>((group) => ({
      id: group.id,
      kind: 'availability',
      dayOfWeek: day.dayOfWeek,
      startTime: group.startTime,
      endTime: group.endTime,
      subject: group.ownerType === 'teacher' ? '老师可授课' : '学生可上课',
      title: group.ownerType === 'teacher' ? '老师可授课' : '学生可上课',
      subtitle: group.subtitle,
      meta: '可排课时间',
      countText: group.countText,
      record: group.record
    }));
    const candidateItems = sortByStartTime(candidatesByDay[day.dayOfWeek] ?? []).map<TimelineItem>((candidate) => {
      const course = courseById[candidate.courseId];
      const level = candidateLevelMeta(candidateLevel(candidate));
      return {
        id: candidate.id,
        kind: 'candidate',
        dayOfWeek: day.dayOfWeek,
        startTime: candidate.startTime,
        endTime: candidate.endTime,
        subject: candidate.subject || course?.subject || '其他',
        title: courseSubjectGradeText(course, candidate.courseName),
        subtitle: candidate.courseName,
        meta: `${teacherDisplay(candidate.teacherName, course, teacherById[candidate.teacherId])} · 学生：${studentDisplayNames(candidate.availableStudents)}`,
        classType: candidate.classType,
        countText: `${candidate.studentCount}/${candidate.capacity}`,
        status: level.label,
        record: candidate
      };
    });
    const classItems = sortByStartTime(classesByDay[day.dayOfWeek] ?? []).map<TimelineItem>((item) => {
      const course = courseById[item.courseId];
      return {
        id: item.id,
        kind: 'class',
        dayOfWeek: day.dayOfWeek,
        startTime: item.startTime,
        endTime: item.endTime,
        subject: scheduleClassSubject(item, courseById) || '其他',
        title: item.name || courseSubjectGradeText(course, item.courseName),
        subtitle: courseSubjectGradeText(course, item.courseName),
        meta: `${teacherDisplay(item.teacherName, course, teacherById[item.teacherId])} · 学生：${studentDisplayNames(item.students)}`,
        classType: item.classType,
        countText: `${item.students.length}/${item.capacity}`,
        status: item.status,
        record: item
      };
    });
    return [...availabilityItems, ...candidateItems, ...classItems];
  });
}

export function aggregateAvailabilitySlots(slots: AvailabilitySlot[], teacherById: Record<string, Teacher>, studentById: Record<string, Student>) {
  const groups = new Map<string, { slots: AvailabilitySlot[]; names: string[] }>();
  slots.forEach((slot) => {
    const key = `${slot.ownerType}-${slot.dayOfWeek}-${slot.startTime}-${slot.endTime}`;
    const group = groups.get(key) ?? { slots: [], names: [] };
    group.slots.push(slot);
    group.names.push(availabilityOwnerDisplayName(slot, teacherById, studentById));
    groups.set(key, group);
  });
  return Array.from(groups.values()).map((group) => {
    const first = group.slots[0];
    const names = group.names;
    const shortNames = names.length <= 2 ? names.join('、') : `${names.slice(0, 2).join('、')} 等`;
    return {
      id: `${first.ownerType}-${first.dayOfWeek}-${first.startTime}-${first.endTime}`,
      ownerType: first.ownerType,
      startTime: first.startTime,
      endTime: first.endTime,
      subtitle: names.length === 1 ? shortNames : `${names.length} 人：${shortNames}`,
      countText: names.length > 1 ? `${names.length}人` : undefined,
      record: first
    };
  });
}

export function layoutOverlappingItems(items: TimelineItem[]) {
  const primary = items.filter((item) => item.kind !== 'availability');
  const availability = items.filter((item) => item.kind === 'availability');
  if (primary.length === 0) return layoutOverlappingGroup(availability);
  const primaryItems = layoutOverlappingGroup(primary).map((item) => ({
    ...item,
    leftPct: (76 / item.columns) * item.column,
    widthPct: 76 / item.columns
  }));
  const availabilityItems = layoutOverlappingGroup(availability).map((item) => ({
    ...item,
    leftPct: 78 + (20 / item.columns) * item.column,
    widthPct: 20 / item.columns
  }));
  return [...availabilityItems, ...primaryItems].sort((left, right) => timeToMinutes(left.startTime) - timeToMinutes(right.startTime));
}

export function layoutOverlappingGroup(items: TimelineItem[]) {
  const sorted = [...items].sort((left, right) => {
    const diff = timeToMinutes(left.startTime) - timeToMinutes(right.startTime);
    return diff || timeToMinutes(left.endTime) - timeToMinutes(right.endTime);
  });
  const result: TimelineLayoutItem[] = [];
  let group: TimelineItem[] = [];
  let groupEnd = -1;

  function flushGroup() {
    if (group.length === 0) return;
    const columnEnds: number[] = [];
    const laidOut = group.map((item) => {
      const start = timeToMinutes(item.startTime);
      const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
      let column = columnEnds.findIndex((value) => value <= start);
      if (column === -1) {
        column = columnEnds.length;
        columnEnds.push(end);
      } else {
        columnEnds[column] = end;
      }
      return { ...item, column, columns: 1 };
    });
    const columns = Math.max(1, columnEnds.length);
    laidOut.forEach((item) => result.push({ ...item, columns }));
    group = [];
    groupEnd = -1;
  }

  sorted.forEach((item) => {
    const start = timeToMinutes(item.startTime);
    const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
    if (group.length > 0 && start >= groupEnd) flushGroup();
    group.push(item);
    groupEnd = Math.max(groupEnd, end);
  });
  flushGroup();
  return result;
}

export function buildTimelineRange(items: TimelineItem[]) {
  const minutes = items.flatMap((item) => [timeToMinutes(item.startTime), timeToMinutes(item.endTime)]).filter((value) => Number.isFinite(value));
  if (minutes.length === 0) return { start: defaultTimelineStart, end: defaultTimelineEnd };
  const min = Math.min(defaultTimelineStart, ...minutes);
  const max = Math.max(defaultTimelineEnd, ...minutes);
  return {
    start: Math.max(0, Math.floor(min / 60) * 60),
    end: Math.min(24 * 60, Math.ceil(max / 60) * 60)
  };
}

export function buildTimelineRows(start: number, end: number) {
  const rows = [];
  for (let minute = start; minute <= end; minute += timelineSlotMinutes) {
    rows.push({
      minute,
      top: ((minute - start) / timelineSlotMinutes) * timelineSlotHeight,
      label: formatMinute(minute)
    });
  }
  return rows;
}

export function timeToMinutes(value: string) {
  const match = /^(\d{1,2}):(\d{2})$/.exec(value || '');
  if (!match) return defaultTimelineStart;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (!Number.isFinite(hour) || !Number.isFinite(minute)) return defaultTimelineStart;
  return Math.max(0, Math.min(24 * 60, hour * 60 + minute));
}

export function clampTimelineStart(startMinute: number, durationMinutes: number) {
  const maxStart = Math.max(0, 23 * 60 + 30 - durationMinutes);
  return Math.max(0, Math.min(maxStart, startMinute));
}

export function formatMinute(value: number) {
  const hour = Math.floor(value / 60);
  const minute = value % 60;
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

export function subjectColor(subject: string) {
  const normalized = subject.toLowerCase();
  if (normalized.includes('eng') || subject.includes('英文') || subject.includes('英语')) return { bg: '#d8ebfb', border: '#a9cdea', accent: '#2f7dcc', text: '#14395d' };
  if (normalized.includes('math') || subject.includes('数学')) return { bg: '#fff5c9', border: '#f1d673', accent: '#d4a900', text: '#594700' };
  if (normalized.includes('geo') || normalized.includes('his') || subject.includes('地理') || subject.includes('历史')) return { bg: '#d8f0f5', border: '#9ed4df', accent: '#32899b', text: '#164e59' };
  if (normalized.includes('sci') || subject.includes('科学') || subject.includes('物理') || subject.includes('化学') || subject.includes('生物')) return { bg: '#dfe6ff', border: '#adbdf4', accent: '#3556c4', text: '#1e2d68' };
  if (normalized.includes('chn') || subject.includes('语文') || subject.includes('中文')) return { bg: '#efd9f6', border: '#d7a9e6', accent: '#9b43bd', text: '#552066' };
  if (subject.includes('老师')) return { bg: '#eef9f1', border: '#b8e2c4', accent: '#32975a', text: '#19542f' };
  if (subject.includes('学生')) return { bg: '#eef6ff', border: '#b9d8fb', accent: '#347fc4', text: '#1e4d78' };
  const palette = [
    { bg: '#f1e9dc', border: '#d8c5a5', accent: '#9a6b2f', text: '#553712' },
    { bg: '#e5f3e8', border: '#b8d9c1', accent: '#3d8d58', text: '#1f5130' },
    { bg: '#f6e2e2', border: '#e5b5b5', accent: '#b24b4b', text: '#672323' },
    { bg: '#e8edf4', border: '#c3ccd9', accent: '#5d6f89', text: '#2e3b4f' }
  ];
  const index = Math.abs([...subject].reduce((sum, char) => sum + char.charCodeAt(0), 0)) % palette.length;
  return palette[index];
}

export function shortList(values: string[], limit = 2) {
  const cleaned = values.filter(Boolean);
  if (cleaned.length === 0) return '';
  if (cleaned.length <= limit) return cleaned.join('、');
  return `${cleaned.slice(0, limit).join('、')}等`;
}

export function scheduleResultNote(items: ScheduleClass[], totalConfirmedCount: number, hasFilters: boolean, grade?: string) {
  const gradeText = grade ? `${grade}课程` : '课程';
  if (items.length === 0) {
    return hasFilters ? `没有符合筛选条件的${gradeText}，可清空筛选查看全部已确认课程。` : '还没有已确认课程，确认候选时间后会显示在这里。';
  }
  const confirmedCount = items.filter((item) => item.status === '已确认').length;
  const canceledCount = items.filter((item) => item.status === '已取消').length;
  if (canceledCount > 0) {
    return `当前显示 ${items.length} 节${gradeText}，其中已确认 ${confirmedCount} 节、已取消 ${canceledCount} 节。全部已确认课程 ${totalConfirmedCount} 节。`;
  }
  return `当前显示 ${confirmedCount} 节已确认${gradeText}，全部已确认课程 ${totalConfirmedCount} 节。`;
}

export function availabilityStats(items: AvailabilitySlot[]) {
  return {
    total: items.length,
    teacherCount: items.filter((item) => item.ownerType === 'teacher').length,
    studentCount: items.filter((item) => item.ownerType === 'student').length
  };
}

export function uniqueScheduleCampuses(items: ScheduleClass[]) {
  const values = items
    .map((item) => item.campusId || 'campus-main')
    .filter(Boolean);
  return Array.from(new Set(values)).sort();
}

export function uniqueScheduleSubjects(classes: ScheduleClass[], candidates: ScheduleCandidate[], courseById: CourseLookup) {
  const values = [
    ...classes.map((item) => scheduleClassSubject(item, courseById)),
    ...candidates.map((item) => item.subject || courseById[item.courseId]?.subject),
    '老师可授课',
    '学生可上课'
  ].map((value) => value || '其他');
  return Array.from(new Set(values)).sort((left, right) => {
    const priority = ['Eng', '英文', '英语', 'Math', '数学', 'Geo/His', 'Sci', '科学', '综合科学', 'CHN', '语文'];
    const leftIndex = priority.findIndex((item) => left.includes(item));
    const rightIndex = priority.findIndex((item) => right.includes(item));
    if (leftIndex !== -1 || rightIndex !== -1) return (leftIndex === -1 ? 99 : leftIndex) - (rightIndex === -1 ? 99 : rightIndex);
    return left.localeCompare(right, 'zh-Hans-CN');
  });
}

export function filterClasses(items: ScheduleClass[], filters: ScheduleFilters, courseById: CourseLookup) {
  return items.filter((item) =>
    (!filters.grade || scheduleClassGrade(item, courseById) === filters.grade) &&
    (!filters.subject || scheduleClassSubject(item, courseById) === filters.subject) &&
    (!filters.teacherId || item.teacherId === filters.teacherId) &&
    (!filters.studentId || item.students.some((student) => student.id === filters.studentId)) &&
    (!filters.campusId || (item.campusId || 'campus-main') === filters.campusId) &&
    (!filters.courseId || item.courseId === filters.courseId) &&
    (!filters.classType || item.classType === filters.classType) &&
    (!filters.status || filters.status === '全部' || item.status === filters.status)
  );
}

export function scheduleClassGrade(item: ScheduleClass, courseById: CourseLookup) {
  return courseById[item.courseId]?.grade || item.students[0]?.grade || '';
}

export function scheduleClassSubject(item: ScheduleClass, courseById: CourseLookup) {
  return courseById[item.courseId]?.subject || '';
}

export function groupScheduleItems<T extends { dayOfWeek: number; startTime: string }>(items: T[]) {
  return items.reduce<Record<number, T[]>>((result, item) => {
    result[item.dayOfWeek] = [...(result[item.dayOfWeek] ?? []), item].sort((left, right) => left.startTime.localeCompare(right.startTime));
    return result;
  }, {});
}

export function scheduleClassPayload(record: ScheduleClass, target: ScheduleMoveTarget = { dayOfWeek: record.dayOfWeek, label: weekLabel(record.dayOfWeek) }): ScheduleClassFormValues {
  return {
    courseId: record.courseId,
    teacherId: record.teacherId,
    campusId: record.campusId || 'campus-main',
    roomName: '',
    classType: record.classType,
    durationMinutes: record.durationMinutes,
    dayOfWeek: target.dayOfWeek,
    startTime: target.startTime ?? record.startTime,
    endTime: target.endTime ?? record.endTime,
    startDate: target.startDate ?? record.startDate,
    endDate: target.endDate ?? record.endDate,
    studentIds: record.students.map((student) => student.id)
  };
}

export function buildWeekDays(weekStart: Date): WeekDay[] {
  return Array.from({ length: 7 }, (_, index) => {
    const date = addDays(weekStart, index);
    const dayOfWeek = date.getDay() === 0 ? 7 : date.getDay();
    return {
      key: localDateText(date),
      date,
      day: date.getDate(),
      dayOfWeek,
      weekLabel: weekLabel(dayOfWeek),
      label: `${date.getMonth() + 1}月${date.getDate()}日 ${weekLabel(dayOfWeek)}`
    };
  });
}

export function buildMonthDays(base: Date) {
  const first = new Date(base.getFullYear(), base.getMonth(), 1);
  const last = new Date(base.getFullYear(), base.getMonth() + 1, 0);
  const start = new Date(first);
  start.setDate(first.getDate() - ((first.getDay() + 6) % 7));
  const days = [];
  for (let index = 0; index < 42; index++) {
    const date = new Date(start);
    date.setDate(start.getDate() + index);
    const dayOfWeek = date.getDay() === 0 ? 7 : date.getDay();
    days.push({
      key: localDateText(date),
      date,
      day: date.getDate(),
      dayOfWeek,
      weekLabel: weekLabel(dayOfWeek),
      label: `${date.getMonth() + 1}月${date.getDate()}日 ${weekLabel(dayOfWeek)}`,
      inMonth: date.getMonth() === base.getMonth()
    });
  }
  return days.filter((day) => day.inMonth || day.date >= first && day.date <= last);
}

export function buildMiniMonthDays(base: Date) {
  const first = new Date(base.getFullYear(), base.getMonth(), 1);
  const start = startOfWeek(first);
  const todayKey = localDateText(new Date());
  return Array.from({ length: 42 }, (_, index) => {
    const date = addDays(start, index);
    const key = localDateText(date);
    return {
      key,
      date,
      inMonth: date.getMonth() === base.getMonth(),
      isToday: key === todayKey
    };
  });
}

export function hasGroupedItems<T>(groups: Record<number, T[]>) {
  return Object.values(groups).some((items) => items.length > 0);
}

export function sortByStartTime<T extends { startTime: string }>(items: T[]) {
  return [...items].sort((left, right) => left.startTime.localeCompare(right.startTime));
}

export function candidateEmptyTips(request: CandidateFormValues | null, candidates: ScheduleCandidate[]) {
  if (!request) return ['先选择学科和年级，再查找可排时间。'];
  if (candidates.length > 0) return [];
  return [
    '确认该学科 + 年级已有开通学生，且老师授课范围覆盖该学科年级。',
    '让相关老师和学生补充可上课时间后再查找。',
    '可缩短课长或更换班型，更容易凑齐时间。'
  ];
}
