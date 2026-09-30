import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, PointerEvent as ReactPointerEvent } from 'react';
import { Button, Empty, Space, Tag, Tooltip } from 'antd';
import { subjectLabel } from '../../utils/curriculum';
import type { AvailabilitySlot, Course, ScheduleClass, Student, Teacher } from '../../types/starline';
import { availabilityCovers, localDateText, startOfWeek, weekdayOfDateText } from './scheduling-utils';
import { buildTimelineItems, buildWeekDays, layoutOverlappingItems, TimelineBlock, type ScheduleMoveTarget } from './SchedulingViews';

export type CalendarMode = 'day' | 'workweek' | 'week' | 'month' | 'list';
export type CalendarSelection = { startTime?: string; endTime?: string; ownerKey?: string };
export type CalendarPerson = { key: string; name: string; kind: 'teacher' | 'student'; id: string };
export const minuteText = (minute: number) => `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`;
const minuteOf = (text: string) => Number(text.slice(0, 2)) * 60 + Number(text.slice(3));
export function personHasClass(person: CalendarPerson, lesson: ScheduleClass) {
  return person.kind === 'teacher' ? lesson.teacherId === person.id : lesson.students.some(student => student.id === person.id);
}
export function slotsOnDate(slots: AvailabilitySlot[], date: string, person?: CalendarPerson) {
  return slots.filter(slot => slot.dayOfWeek === weekdayOfDateText(date)
    && (!person || (slot.ownerType === person.kind && slot.ownerId === person.id))
    && (!slot.startDate || slot.startDate <= date) && (!slot.endDate || slot.endDate >= date));
}

export function CalendarTimeline({ mode, date, people, lessons, slots, courseById, teacherById, studentById, canManage, onCreate, onEdit, onCopy, onMove, onResize }: {
  mode: 'day' | 'workweek' | 'week'; date: Date; people: CalendarPerson[];
  lessons: ScheduleClass[]; slots: AvailabilitySlot[]; courseById: Record<string, Course>;
  teacherById: Record<string, Teacher>; studentById: Record<string, Student>; canManage: boolean;
  onCreate: (date: string, selection?: CalendarSelection) => void;
  onEdit: (lesson: ScheduleClass) => void; onCopy: (lesson: ScheduleClass) => void;
  onMove: (lesson: ScheduleClass, target: ScheduleMoveTarget) => void;
  onResize: (lesson: ScheduleClass, endTime: string) => void;
}) {
  const scroller = useRef<HTMLDivElement>(null);
  const columns = useMemo(() => {
    const days = mode === 'day' ? [date] : buildWeekDays(startOfWeek(date)).slice(0, mode === 'workweek' ? 5 : 7).map(day => day.date);
    return days.flatMap(day => {
      const dateText = localDateText(day);
      const owners = mode === 'day' ? (people.length ? people : [undefined]) : [people.length === 1 ? people[0] : undefined];
      return owners.map(person => ({ date: dateText, person, key: `${dateText}:${person?.key ?? 'all'}`,
        lessons: lessons.filter(lesson => lesson.lessonDate === dateText && (!person || personHasClass(person, lesson))),
        slots: slotsOnDate(slots, dateText, person).filter(slot => people.length === 0 || people.some(p => p.kind === slot.ownerType && p.id === slot.ownerId)) }));
    });
  }, [mode, date, people, lessons, slots]);
  // Include early/late lessons and availability rather than cutting them off.
  const times = columns.flatMap(column => [...column.lessons, ...column.slots]);
  const start = Math.min(480, ...times.map(item => Math.floor(minuteOf(item.startTime) / 30) * 30));
  const end = Math.max(1320, ...times.map(item => Math.ceil(minuteOf(item.endTime) / 30) * 30));
  const height = (end - start) / 30 * 44;
  useEffect(() => {
    const first = columns.flatMap(column => column.lessons).filter(lesson => lesson.status !== '已取消').map(lesson => minuteOf(lesson.startTime));
    if (scroller.current) scroller.current.scrollTop = first.length ? Math.max(0, (Math.min(...first) - start) / 30 * 44 - 44) : 0;
  }, [date, mode, start]);
  return <div ref={scroller} className="schedule-timeline-scroll calendar-scroll">
    <div className="schedule-timeline-grid is-resource" style={{ '--lane-count': columns.length, '--timeline-height': `${height}px` } as CSSProperties}>
      <div className="schedule-time-gutter schedule-day-head-spacer" />
      {columns.map(column => <div className="schedule-day-head schedule-lane-head" key={column.key}>
        <strong>{mode === 'day' ? column.person?.name ?? '全部课程' : `${Number(column.date.slice(5, 7))}/${Number(column.date.slice(8))} ${['', '周一', '周二', '周三', '周四', '周五', '周六', '周日'][weekdayOfDateText(column.date)]}`}</strong>
        <span>{column.person ? (column.person.kind === 'teacher' ? '教师' : '学生') : '课程总览'} · {column.lessons.filter(lesson => lesson.status !== '已取消').length} 节课</span>
        {column.person && <small>{column.slots.some(slot => !slot.unavailable) ? '已登记可上课时间' : '当天可上课时间未登记'}</small>}
      </div>)}
      <div className="schedule-time-gutter schedule-time-axis" style={{ height }}>
        {Array.from({ length: (end - start) / 30 }, (_, i) => <div className="schedule-time-label" key={i} style={{ top: i * 44 }}>{minuteText(start + i * 30)}</div>)}
      </div>
      {columns.map(column => <CalendarColumn {...column} key={column.key} sourceLessons={lessons} start={start} end={end} height={height}
        courseById={courseById} teacherById={teacherById} studentById={studentById} canManage={canManage}
        onCreate={onCreate} onEdit={onEdit} onCopy={onCopy} onMove={onMove} onResize={onResize} />)}
    </div>
  </div>;
}

function CalendarColumn({ date, person, lessons, sourceLessons, slots, start, end, height, courseById, teacherById, studentById, canManage, onCreate, onEdit, onCopy, onMove, onResize }: {
  date: string; person?: CalendarPerson; lessons: ScheduleClass[]; sourceLessons: ScheduleClass[]; slots: AvailabilitySlot[]; start: number; end: number; height: number;
  courseById: Record<string, Course>; teacherById: Record<string, Teacher>; studentById: Record<string, Student>;
  canManage: boolean; onCreate: (date: string, selection?: CalendarSelection) => void;
  onEdit: (lesson: ScheduleClass) => void; onCopy: (lesson: ScheduleClass) => void;
  onMove: (lesson: ScheduleClass, target: ScheduleMoveTarget) => void; onResize: (lesson: ScheduleClass, endTime: string) => void;
}) {
  const columnRef = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(190);
  const [selection, setSelection] = useState<{ from: number; to: number } | null>(null);
  const drag = useRef<{ from: number; to: number; y: number; moved: boolean } | null>(null);
  useEffect(() => {
    if (!columnRef.current) return;
    const observer = new ResizeObserver(entries => setWidth(entries[0].contentRect.width));
    observer.observe(columnRef.current);
    return () => observer.disconnect();
  }, []);
  const day = { key: date, date: new Date(`${date}T00:00:00`), day: Number(date.slice(8)), dayOfWeek: weekdayOfDateText(date), label: date, weekLabel: '' };
  const items = buildTimelineItems([day], {}, { [day.dayOfWeek]: lessons }, courseById, teacherById, studentById).map(item => {
    const lesson = item.record as ScheduleClass;
    const course = courseById[lesson.courseId];
    return { ...item, title: [course ? subjectLabel(course.subject) : lesson.courseName, course?.grade].filter(Boolean).join(' · '), subtitle: '',
      meta: person?.kind === 'student' ? `教师：${teacherById[lesson.teacherId]?.name ?? lesson.teacherName}` : `${person ? '' : (teacherById[lesson.teacherId]?.name ?? lesson.teacherName) + ' · '}${lesson.students.map(student => studentById[student.id]?.name ?? student.name).join('、') || '待补学生'}` };
  });
  const layout = layoutOverlappingItems(items, width);
  const position = (clientY: number) => Math.max(start, Math.min(end - 30, start + Math.floor((clientY - columnRef.current!.getBoundingClientRect().top) / 44) * 30));
  const createAt = (from: number, to: number) => onCreate(date, { ownerKey: person?.key, startTime: minuteText(from), endTime: minuteText(Math.min(end, to)) });
  const finish = (event: ReactPointerEvent<HTMLDivElement>, cancelled = false) => {
    const active = drag.current;
    drag.current = null; setSelection(null);
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId);
    if (!cancelled && active?.moved) createAt(Math.min(active.from, active.to), Math.max(active.from, active.to) + 30);
  };
  return <div ref={columnRef} className="schedule-day-column calendar-column" data-date={date} data-owner={person?.key ?? 'all'} style={{ height }}
    onPointerDown={event => {
      if (!canManage || event.button !== 0 || event.target !== event.currentTarget) return;
      const from = position(event.clientY); drag.current = { from, to: from, y: event.clientY, moved: false };
      event.currentTarget.setPointerCapture(event.pointerId);
    }}
    onPointerMove={event => {
      if (!drag.current) return;
      drag.current.to = position(event.clientY);
      if (Math.abs(event.clientY - drag.current.y) > 6) drag.current.moved = true;
      if (drag.current.moved) setSelection({ from: Math.min(drag.current.from, drag.current.to), to: Math.max(drag.current.from, drag.current.to) + 30 });
    }}
    onPointerUp={event => finish(event)} onPointerCancel={event => finish(event, true)}
    onDoubleClick={event => { if (canManage && event.target === event.currentTarget) createAt(position(event.clientY), position(event.clientY) + 90); }}
    onDragOver={event => { if (canManage) event.preventDefault(); }}
    onDrop={event => {
      event.preventDefault();
      if (!canManage) return;
      const lesson = sourceLessons.find(item => item.id === event.dataTransfer.getData('text/schedule-class-id'));
      if (lesson && person && !personHasClass(person, lesson)) return;
      if (!lesson || lesson.status === '已取消') return;
      const from = Math.min(1440 - lesson.durationMinutes, position(event.clientY));
      onMove(lesson, { lessonDate: date, startTime: minuteText(from), endTime: minuteText(from + lesson.durationMinutes), label: `${date} ${minuteText(from)}-${minuteText(from + lesson.durationMinutes)}` });
    }}>
    {Array.from({ length: (end - start) / 30 }, (_, i) => <span className="schedule-time-line" key={i} style={{ top: i * 44 }} />)}
    {slots.map(slot => <div key={slot.id} className={`calendar-availability is-${slot.ownerType} ${slot.unavailable ? 'is-unavailable' : ''}`} title={`${slot.ownerName} ${slot.unavailable ? '不可上课' : '可上课'} ${slot.startTime}-${slot.endTime}`}
      style={{ top: (minuteOf(slot.startTime) - start) / 30 * 44, height: (minuteOf(slot.endTime) - minuteOf(slot.startTime)) / 30 * 44 }} />)}
    {selection && <div className="calendar-selection" style={{ top: (selection.from - start) / 30 * 44, height: (selection.to - selection.from) / 30 * 44 }}>{minuteText(selection.from)}–{minuteText(selection.to)}</div>}
    {layout.map(item => <TimelineBlock key={`${item.kind}:${item.id}`} item={item} rangeStart={start} canManage={canManage}
      onEditClass={onEdit} onCopyClass={onCopy} onResizeClass={onResize} />)}
  </div>;
}

// Every selected participant must have registered coverage and no overlapping
// lesson. Unknown availability remains explicit and is never called free.
export function SchedulingAssistant({ people, date, duration, lessons, slots, excludeId, onChoose }: {
  people: CalendarPerson[]; date: string; duration: number; lessons: ScheduleClass[]; slots: AvailabilitySlot[]; excludeId?: string;
  onChoose: (startTime: string, endTime: string) => void;
}) {
  const starts = Array.from({ length: 28 }, (_, i) => 480 + i * 30).filter(start => start + duration <= 1440);
  const stateAt = (person: CalendarPerson, start: number) => {
    const end = start + duration;
    if (lessons.some(lesson => lesson.id !== excludeId && lesson.status !== '已取消' && lesson.lessonDate === date && personHasClass(person, lesson)
      && minuteOf(lesson.startTime) < end && minuteOf(lesson.endTime) > start)) return 'busy';
    if (slotsOnDate(slots, date, person).some(slot => slot.unavailable && minuteOf(slot.startTime) < end && minuteOf(slot.endTime) > start)) return 'unavailable';
    const registered = slots.filter(slot => !slot.unavailable && slot.ownerType === person.kind && slot.ownerId === person.id);
    if (!registered.length) return 'unknown';
    return registered.some(slot => availabilityCovers(slot, weekdayOfDateText(date), minuteText(start), minuteText(end), date)) ? 'free' : 'outside';
  };
  const common = starts.filter(start => people.length > 0 && people.every(person => stateAt(person, start) === 'free'));
  if (!people.length) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="选择教师和学生后查看共同时间" />;
  return <div className="schedule-assistant">
    <Space wrap size={6}><Tag color="green">可上课</Tag><Tag color="red">已有课程 / 不可上课</Tag><Tag>超出登记范围</Tag><Tag color="gold">未登记</Tag></Space>
    <div className="assistant-scroll"><div className="assistant-grid" style={{ gridTemplateColumns: `110px repeat(${starts.length}, 52px)` }}>
      <strong>人员 / 开始时间</strong>{starts.map(start => <span key={start}>{minuteText(start)}</span>)}
      {people.map(person => <div className="assistant-row" key={person.key}><strong title={person.name}>{person.name}</strong>{starts.map(start => {
        const state = stateAt(person, start);
        const occupied = lessons.filter(lesson => lesson.id !== excludeId && lesson.status !== '已取消' && lesson.lessonDate === date && personHasClass(person, lesson) && minuteOf(lesson.startTime) < start + duration && minuteOf(lesson.endTime) > start);
        return <Tooltip key={start} title={occupied.length ? occupied.map(lesson => `${lesson.startTime}-${lesson.endTime} ${lesson.name}`).join('；') : state === 'unavailable' ? '已登记临时不可上课' : state === 'unknown' ? '未登记可上课时间' : state === 'outside' ? '超出登记范围' : `可上课 ${duration} 分钟`}>
          <button type="button" className={`assistant-cell is-${state}`} aria-label={`${person.name} ${minuteText(start)} ${state}`} onClick={() => { if (state !== 'busy') onChoose(minuteText(start), minuteText(start + duration)); }}>{state === 'busy' ? '占用' : state === 'unavailable' ? '请假' : state === 'unknown' ? '?' : state === 'free' ? '✓' : '—'}</button>
        </Tooltip>;
      })}</div>)}
    </div></div>
    <Space wrap><span>共同可排 {duration} 分钟：</span>{common.slice(0, 12).map(start => <Button key={start} size="small" onClick={() => onChoose(minuteText(start), minuteText(start + duration))}>{minuteText(start)}</Button>)}{!common.length && <span>没有已登记的共同时间，可协调后手动选择。</span>}</Space>
  </div>;
}
