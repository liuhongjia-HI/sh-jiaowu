import { CalendarOutlined, CloseCircleOutlined, CopyOutlined, DeleteOutlined, DisconnectOutlined, EditOutlined, LeftOutlined, PlusOutlined, ReloadOutlined, RetweetOutlined, RightOutlined, SaveOutlined, SettingOutlined, TableOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Form, Input, InputNumber, Dropdown, Modal, Popconfirm, Popover, Select, Skeleton, Space, Switch, Table, Tag, Tooltip, Typography, message } from 'antd';
import type { FormInstance, TableColumnsType } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { getData, postData, putData } from '../../services/http';
import { ActionButton } from '../../components/ListViews';
import { gradeOptions, subjectOptions } from '../../utils/curriculum';
import type { AvailabilitySlot, Course, CurrentUser, ScheduleClass, Student, Teacher } from '../../types/starline';
import {
  addDays,
  addMonths,
  classCapacity,
  formatWeekRange,
  localDateText,
  minimumStudentCount,
  scheduleClassOccursOn,
  startOfMonth,
  startOfWeek,
  weekLabel,
  weekOptions,
  weekdayOfDateText
} from './scheduling-utils';
import { subjectPalette, subjectShortLabel, type SubjectPalette } from '../../utils/subject-colors';

type AvailabilityFormValues = {
  ownerKey: string;
  slots: AvailabilitySlot[];
};

export type ScheduleRepeatValues = {
  freq: 'daily' | 'weekly';
  interval: number;
  byDay?: number[];
  until?: string;
  count?: number;
};

type ScheduleClassFormValues = {
  courseId: string;
  teacherId: string;
  campusId: string;
  roomName: string;
  classType: string;
  durationMinutes: number;
  startTime: string;
  endTime: string;
  /** 这节课的日期；重复排课时是第一节的日期。 */
  startDate: string;
  studentIds: string[];
  expectedStudentCount: number;
  reservationNote?: string;
  repeat?: ScheduleRepeatValues;
  /** 改重复课次时必填，取值见 EDIT_SCOPE_*。 */
  editScope?: string;
  /** 用户已确认过越出可上课时间的提醒。 */
  ignoreWarnings?: boolean;
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

export type ScheduleMoveTarget = {
  /** 拖到哪一天。课次模型下这是唯一决定日期的字段，星期由它推导。 */
  lessonDate: string;
  label: string;
  startTime?: string;
  endTime?: string;
};

type CourseLookup = Record<string, Course>;
type TimelineKind = 'class' | 'availability' | 'overflow';
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
  // 只画底色不画字：背景带的文字在顶部，常被压在上面的课程块裁掉半行，
  // 留下一截读不通的残字。详情仍在悬浮提示里。
  quiet?: boolean;
  record: ScheduleClass | AvailabilitySlot;
};
type TimelineLayoutItem = TimelineItem & {
  column: number;
  columns: number;
  leftPct?: number;
  widthPct?: number;
  // 同一时段并排数量超过可读上限时，末列折叠成一个 "+N"，hiddenItems 是被折叠的课程。
  hiddenItems?: TimelineItem[];
  // 末列要给右侧「已取消」幽灵条让出的像素宽度。
  reserveRightPx?: number;
  // 已取消课程的幽灵条序号（从右往左第几条）。有值即表示这个块按幽灵条渲染。
  ghostIndex?: number;
};

export type ResourceLane = {
  key: string;
  title: string;
  subtitle: string;
  // 学生可上课泳道只是参考信息，没有可拖动的课程，不接受拖放。
  droppable: boolean;
  items: TimelineItem[];
};

const studentAvailabilityLaneKey = '__student-availability__';
const unassignedLaneKey = '__unassigned__';
const placeholderLaneKey = '__placeholder__';

// 单个课程块低于这个宽度就只剩一两个字，并排再多也读不出是什么课，
// 超出的部分折叠成 "+N"。数值来自实测：约 64px 时课程名可显示 3 个汉字。
const minReadableBlockWidth = 64;
const maxTimelineColumns = 4;
// 周视图单格最多渲染三块（对标 Outlook）。超过三节时，前两节正常显示、
// 第三格变成 "+N"，点开能看到被折叠的全部课程——这样「最多三个」是字面意义上的
// 三个可视块，不会因为再挤一个加号变成四个。
const weekCellMaxLessons = 3;
// 月视图一格最多列 4 条，与客户 Outlook 月视图的密度一致（图上是 4 条 + "+2"）。
// 超过时列前 3 条 + "+N"，保证「+N」本身也算在 4 个格位里。
const monthCellMaxLessons = 4;
// 已取消课程的幽灵条：只画出「这个时段原本有课、现在空出来了」，不需要能读字。
const canceledGhostWidth = 10;
const canceledGhostGap = 3;
const maxCanceledGhosts = 3;

export function maxColumnsForWidth(columnWidth: number) {
  if (!columnWidth || columnWidth <= 0) return 2;
  return Math.min(maxTimelineColumns, Math.max(1, Math.floor(columnWidth / minReadableBlockWidth)));
}
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
const defaultTimelineStart = 8 * 60;
const defaultTimelineEnd = 22 * 60;

export function ScheduleWeekTimeline({
  loading,
  weekDays,
  selectedWeekStart,
  availabilityByDay,
  classesByDay,
  courseById,
  teacherById,
  studentById,
  canManage,
  onPreviousWeek,
  onNextWeek,
  onToday,
  onEditClass,
  onCopyClass,
  onResizeClass,
  onMoveClass,
  onCreateClass
}: {
  loading: boolean;
  weekDays: WeekDay[];
  selectedWeekStart: Date;
  availabilityByDay: Record<number, AvailabilitySlot[]>;
  classesByDay: Record<number, ScheduleClass[]>;
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  studentById: Record<string, Student>;
  canManage: boolean;
  onPreviousWeek: () => void;
  onNextWeek: () => void;
  onToday: () => void;
  onEditClass: (record: ScheduleClass) => void;
  onCopyClass: (record: ScheduleClass) => void;
  onResizeClass: (record: ScheduleClass, endTime: string) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
  onCreateClass: (lessonDate: string) => void;
}) {
  const timelineItems = useMemo(() => buildTimelineItems(weekDays, availabilityByDay, classesByDay, courseById, teacherById, studentById), [weekDays, availabilityByDay, classesByDay, courseById, teacherById, studentById]);
  const hasAnyItem = timelineItems.length > 0;
  const timelineRange = useMemo(() => buildTimelineRange(timelineItems), [timelineItems]);
  const rows = useMemo(() => buildTimelineRows(timelineRange.start, timelineRange.end), [timelineRange]);
  const boardHeight = ((timelineRange.end - timelineRange.start) / timelineSlotMinutes) * timelineSlotHeight;
  // 按日列的真实渲染宽度决定能并排几节课：窗口越窄能读的列数越少，
  // 超出的折叠成 "+N"，而不是把每块压到只剩一两个字。
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [dayColumnWidth, setDayColumnWidth] = useState(0);
  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    // 同步测量：effect 与 resize 回调触发时布局已经结算完毕。
    // 不要包在 requestAnimationFrame 里——后台标签页或无头环境下 rAF 可能一直不触发，
    // 会让宽度永远停在初始值，静默退化成固定列数。
    const measure = () => {
      const column = scroller.querySelector('.schedule-day-column');
      if (column) setDayColumnWidth(column.getBoundingClientRect().width);
    };
    measure();
    // 观察滚动容器而不是内部表格：表格有 min-width，窗口收窄到阈值以下时
    // 它的宽度会被钉住不再变化，观察它就收不到后续的尺寸变化。
    // 同时监听 window resize：ResizeObserver 覆盖侧边栏折叠这类窗口尺寸不变的布局变化，
    // window resize 覆盖普通的缩放窗口，两者都留着以免任一环境下失效。
    const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(measure) : null;
    observer?.observe(scroller);
    window.addEventListener('resize', measure);
    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [loading, weekDays.length]);

  // 和日视图同一套：时间轴固定 08:00-22:00，课集中在傍晚，打开后先滚到第一节课，
  // 否则上面永远是几屏空白。两个视图行为保持一致，切换时不会一个自动定位一个不定位。
  const firstItemMinute = useMemo(() => {
    const starts = timelineItems
      .filter((item) => item.kind === 'class')
      .map((item) => timeToMinutes(item.startTime));
    return starts.length > 0 ? Math.min(...starts) : null;
  }, [timelineItems]);
  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller || firstItemMinute === null) return;
    const offset = ((firstItemMinute - timelineRange.start) / timelineSlotMinutes) * timelineSlotHeight;
    scroller.scrollTop = Math.max(0, offset - timelineSlotHeight);
  }, [firstItemMinute, timelineRange.start, selectedWeekStart]);

  const itemsByDay = useMemo(() => {
    return weekDays.reduce<Record<number, TimelineLayoutItem[]>>((result, day) => {
      result[day.dayOfWeek] = layoutOverlappingItems(timelineItems.filter((item) => item.dayOfWeek === day.dayOfWeek), dayColumnWidth, weekCellMaxLessons);
      return result;
    }, {});
  }, [weekDays, timelineItems, dayColumnWidth]);

  if (loading) return <Skeleton active paragraph={{ rows: 6 }} />;

  return (
    <div className="schedule-timeline-wrap">
      {!hasAnyItem && (
        <ScheduleEmptyTips
          description="这一周还没有课程。双击空白格即可直接排一节。"
          compact
        />
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

      <div ref={scrollRef} className="schedule-timeline-scroll">
        <div
          className="schedule-timeline-grid"
          style={{ '--timeline-height': `${boardHeight}px`, '--day-count': weekDays.length } as CSSProperties}
        >
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
                    lessonDate: day.key,
                    startTime,
                    endTime,
                    label: `${day.label} ${startTime}-${endTime}`
                  });
                }}
                onDoubleClick={(event) => {
                  if (event.currentTarget === event.target && canManage) onCreateClass(day.key);
                }}
              >
                {rows.map((row) => <span className="schedule-time-line" key={row.minute} style={{ top: row.top }} />)}
                {dayItems.length === 0 && (
                  <button type="button" className="schedule-day-empty-slot" onClick={() => canManage ? onCreateClass(day.key) : undefined}>
                    {canManage ? '新建课程' : '暂无安排'}
                  </button>
                )}
                {dayItems.map((item) => (
                  <TimelineBlock
                    key={`${item.kind}-${item.id}`}
                    item={item}
                    rangeStart={timelineRange.start}
                    canManage={canManage}
                    onEditClass={onEditClass}
                    onCopyClass={onCopyClass}
                  onResizeClass={onResizeClass}
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

// 资源泳道视图：列＝老师（或教室），只看一天。
//
// 周视图用的是通用日历那套「重叠簇内等宽分列」，它的前提是「同一个人的日程本来就不该重叠」，
// 所以重叠是异常、是少数。教培排课的前提恰好相反：同一时段本来就该有多节课并行
// （不同老师、不同教室），重叠是业务主干。7 天塞进横向空间后每块只剩几十像素，
// 课程名必然被截断，再怎么调分列算法也救不回来——横向空间是被「7 天」吃掉的，不是被重叠吃掉的。
//
// 把资源提升为列维度后，跨老师的重叠在结构上就消失了。每条泳道内部剩下的重叠才是真冲突
// （同一个老师同一时段两节课＝排课错误），数量极少，此时复用 layoutOverlappingItems
// 的等宽分列正好回到了它擅长的场景：不用重写算法，只是换了个正确的容器。
export function ScheduleDayResourceTimeline({
  loading,
  selectedDate,
  availabilityByDay,
  classesByDay,
  courseById,
  teacherById,
  studentById,
  canManage,
  nearestClassDate,
  onPreviousDay,
  onNextDay,
  onToday,
  onJumpToDate,
  onEditClass,
  onCopyClass,
  onResizeClass,
  onMoveClass,
  onCreateClass
}: {
  loading: boolean;
  selectedDate: Date;
  nearestClassDate: { date: Date; count: number } | null;
  availabilityByDay: Record<number, AvailabilitySlot[]>;
  classesByDay: Record<number, ScheduleClass[]>;
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  studentById: Record<string, Student>;
  canManage: boolean;
  onPreviousDay: () => void;
  onNextDay: () => void;
  onToday: () => void;
  onJumpToDate: (date: Date) => void;
  onEditClass: (record: ScheduleClass) => void;
  onCopyClass: (record: ScheduleClass) => void;
  onResizeClass: (record: ScheduleClass, endTime: string) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
  onCreateClass: (lessonDate: string) => void;
}) {
  const dayOfWeek = selectedDate.getDay() === 0 ? 7 : selectedDate.getDay();
  const lanes = useMemo(
    () => buildResourceLanes(
      dayOfWeek,
      availabilityByDay[dayOfWeek] ?? [],
      (classesByDay[dayOfWeek] ?? []).filter((item) => scheduleClassOccursOn(item, selectedDate)),
      courseById,
      teacherById,
      studentById
    ),
    [dayOfWeek, availabilityByDay, classesByDay, selectedDate, courseById, teacherById, studentById]
  );
  const allItems = useMemo(() => lanes.flatMap((lane) => lane.items), [lanes]);
  const hasAnyItem = allItems.length > 0;
  const timelineRange = useMemo(() => buildTimelineRange(allItems), [allItems]);
  const rows = useMemo(() => buildTimelineRows(timelineRange.start, timelineRange.end), [timelineRange]);
  const boardHeight = ((timelineRange.end - timelineRange.start) / timelineSlotMinutes) * timelineSlotHeight;

  // 空白日的成因分两种，给的出路完全不同：一条可上课时间都没收到时，卡点在收集环节，
  // 催建课没用；收到了但没落在这一天，才是「换一天看」或「手动补一节」。
  // 顶部统计条其实已经知道是哪一种，只是没喂给空态。
  // 注意：这里不能按周末/工作日分叉。校外教培周六日恰恰是排课高峰，
  // 把周末的空白解释成「周末一般不排课」是把最该补课的一天说成正常。
  const totalAvailabilityCount = useMemo(
    () => Object.values(availabilityByDay).reduce((sum, slots) => sum + slots.length, 0),
    [availabilityByDay]
  );
  const emptyReason = useMemo(() => {
    const dayText = formatDayTitle(selectedDate);
    if (totalAvailabilityCount === 0) return `${dayText} 没有课程。还没有收到任何老师或学生填报的可上课时间，排课时不会有参考底纹，但仍可直接双击空白格排课。`;
    return `${dayText} 没有课程：已收到的 ${totalAvailabilityCount} 个可上课时段都不在这一天。`;
  }, [selectedDate, totalAvailabilityCount]);

  // 与周视图同一套测量逻辑：泳道内真出现冲突时，按泳道实际宽度决定并排几块。
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [laneWidth, setLaneWidth] = useState(0);
  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller) return;
    const measure = () => {
      const column = scroller.querySelector('.schedule-day-column');
      if (column) setLaneWidth(column.getBoundingClientRect().width);
    };
    measure();
    const observer = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(measure) : null;
    observer?.observe(scroller);
    window.addEventListener('resize', measure);
    return () => {
      observer?.disconnect();
      window.removeEventListener('resize', measure);
    };
  }, [loading, lanes.length]);

  // 这一天什么都没有时也要把时间轴画出来。原来直接用空态整块替掉网格，进页面看到的
  // 是一张插画，既读不出「这天是几点到几点、每个时段是空的」，也丢掉了双击空白建课的入口——
  // 用户得先跳到有课的一天才能看见排班表长什么样。日历产品的通行做法是空日子照画网格，
  // 把「没有内容」降级成一条提示条。泳道由数据推出来，空的时候补一条占位列撑住网格。
  const laidOutLanes = useMemo(() => {
    const laid = lanes.map((lane) => ({ ...lane, layout: layoutOverlappingItems(lane.items, laneWidth) }));
    if (laid.length > 0) return laid;
    return [{
      key: placeholderLaneKey,
      title: '暂无老师排课',
      subtitle: '这一天还没有安排',
      droppable: false,
      items: [],
      layout: []
    }];
  }, [lanes, laneWidth]);

  // 时间轴固定 08:00-22:00，但课通常集中在傍晚，打开后上方是大片空白，
  // 得手动往下翻好几屏才看得到课。日历产品的通行做法是保留完整时间轴（否则拖不到
  // 范围外的时段），改成打开时自动滚到第一节课——这里滚的是时间轴自己的滚动容器，
  // 不动整页滚动位置，不会把上面的筛选区顶走。
  const firstItemMinute = useMemo(() => {
    const starts = lanes
      .flatMap((lane) => lane.items)
      .filter((item) => item.kind === 'class')
      .map((item) => timeToMinutes(item.startTime));
    return starts.length > 0 ? Math.min(...starts) : null;
  }, [lanes]);
  useEffect(() => {
    const scroller = scrollRef.current;
    if (!scroller || firstItemMinute === null) return;
    const offset = ((firstItemMinute - timelineRange.start) / timelineSlotMinutes) * timelineSlotHeight;
    scroller.scrollTop = Math.max(0, offset - timelineSlotHeight);
  }, [firstItemMinute, timelineRange.start, selectedDate]);

  if (loading) return <Skeleton active paragraph={{ rows: 6 }} />;

  return (
    <div className="schedule-timeline-wrap">
      {/* 工具栏必须排在空态前面：先让人看清「你在看哪一天」，再说这天没有内容。
          反过来的话，满屏「还没有课程」而日期在下方，很容易被读成整个功能坏了。 */}
      <div className="schedule-timeline-toolbar">
        <Space size={8}>
          <Button icon={<LeftOutlined />} onClick={onPreviousDay} />
          <Button onClick={onToday}>今天</Button>
          <Button icon={<RightOutlined />} onClick={onNextDay} />
          {/* 这一天一节课都没有时泳道整个不渲染，双击空白的入口也就没了，
              新建必须有一个不依赖泳道的常驻入口。 */}
          {canManage && <Button icon={<PlusOutlined />} onClick={() => onCreateClass(localDateText(selectedDate))}>新建课程</Button>}
        </Space>
        <div>
          <Typography.Title level={4}>{formatDayTitle(selectedDate)}</Typography.Title>
          <Typography.Text type="secondary">双击空白处新建课程；拖动课程可在本泳道内改时间，换老师或教室请点击课程编辑。</Typography.Text>
        </div>
      </div>

      {/* 空态要给出路。「这天没课」本身没有信息量：要么是可上课时间还没收上来，
          要么是收上来了但没落在这一天——两种情况的下一步动作完全不同。 */}
      {!hasAnyItem && (
        <div className="schedule-day-empty-bar">
          <span>{emptyReason}</span>
          {nearestClassDate && (
            <Button type="link" size="small" onClick={() => onJumpToDate(nearestClassDate.date)}>
              最近的排课在 {formatDayTitle(nearestClassDate.date)}（{nearestClassDate.count} 节），去看看
            </Button>
          )}
        </div>
      )}

      <div ref={scrollRef} className="schedule-timeline-scroll">
        <div
          className="schedule-timeline-grid is-resource"
          style={{
            '--timeline-height': `${boardHeight}px`,
            '--lane-count': laidOutLanes.length,
            // 「学生可上课」是跨老师的参考信息，不是一条真资源，给它和老师同等宽度
            // 会把它抬到和老师一样的地位，老师少的时候还会被拉得特别宽。固定窄一档。
            gridTemplateColumns: `64px ${laidOutLanes
              .map((lane) => (lane.key === studentAvailabilityLaneKey ? '200px' : 'minmax(190px, 1fr)'))
              .join(' ')}`
          } as CSSProperties}
        >
          <div className="schedule-time-gutter schedule-day-head-spacer" />
          {laidOutLanes.map((lane) => (
            <div className="schedule-day-head schedule-lane-head" key={lane.key}>
              <strong title={lane.title}>{lane.title}</strong>
              <span title={lane.subtitle}>{lane.subtitle}</span>
              {/* 学生可上课那条泳道里一节课都没有，写「0 节课」会被读成「今天没课」，
                  它该报的是覆盖了几个可排时段。 */}
              <small>
                {lane.key === studentAvailabilityLaneKey
                  ? `${lane.items.length} 个可排时段`
                  : `${lane.items.filter((item) => item.kind === 'class' && item.status !== '已取消').length} 节课`}
              </small>
            </div>
          ))}
          <div className="schedule-time-gutter schedule-time-axis" style={{ height: boardHeight }}>
            {rows.map((row) => (
              <div className="schedule-time-label" key={row.minute} style={{ top: row.top }}>
                {row.label}
              </div>
            ))}
          </div>
          {laidOutLanes.map((lane) => (
            <div
              className="schedule-day-column"
              key={lane.key}
              style={{ height: boardHeight }}
              onDragOver={(event) => canManage && lane.droppable ? event.preventDefault() : undefined}
              onDrop={(event) => {
                const classID = event.dataTransfer.getData('text/schedule-class-id');
                // 只接受本泳道内的课程：调课接口不带老师/教室，跨泳道拖动无法真正改归属，
                // 放行只会让人以为换了老师其实只改了时间。跨泳道请走编辑弹窗。
                const record = lane.items.find((item) => item.kind === 'class' && item.id === classID)?.record as ScheduleClass | undefined;
                if (!record) return;
                const bounds = event.currentTarget.getBoundingClientRect();
                const offsetMinutes = Math.round((event.clientY - bounds.top) / timelineSlotHeight) * timelineSlotMinutes;
                const startMinute = clampTimelineStart(timelineRange.start + offsetMinutes, record.durationMinutes);
                const startTime = formatMinute(startMinute);
                const endTime = formatMinute(startMinute + record.durationMinutes);
                onMoveClass(record, {
                  lessonDate: localDateText(selectedDate),
                  startTime,
                  endTime,
                  label: `${weekLabel(dayOfWeek)} ${startTime}-${endTime}`
                });
              }}
              onDoubleClick={(event) => {
                if (event.currentTarget === event.target && canManage) onCreateClass(localDateText(selectedDate));
              }}
            >
              {rows.map((row) => <span className="schedule-time-line" key={row.minute} style={{ top: row.top }} />)}
              {lane.layout.map((item) => (
                <TimelineBlock
                  key={`${item.kind}-${item.id}`}
                  item={item}
                  rangeStart={timelineRange.start}
                  canManage={canManage}
                  onEditClass={onEditClass}
                  onCopyClass={onCopyClass}
                  onResizeClass={onResizeClass}
                />
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function ScheduleEmptyTips({ description, compact = false }: { description: string; compact?: boolean }) {
  return (
    <div className={compact ? 'schedule-empty-tips compact' : 'schedule-empty-tips'}>
      <Empty description={description} />
    </div>
  );
}

// 迷你日历同时承担「跳转」和「这个月哪几天有课」的导航职责。
// 后者是日视图能用起来的前提：日视图一次只看一天，如果日历上看不出哪天有课，
// 打开就是空白的那天时，用户没有任何线索知道该往哪翻，只能盲点上一天/下一天。
//
// highlight 区分两种视图：周视图选中的是一整周，日视图选中的是具体某一天。
// 日视图下仍然高亮整周的话，点 19 号和点 21 号看起来完全一样，等于没有反馈。
//
// weekDayCount 让高亮只覆盖实际显示的那几天：工作周视图只有周一到周五，
// 把周六日也点亮会让人以为那两天在视图里，点进去却什么都没有。
export function MiniMonthCalendar({
  month,
  selectedWeekStart,
  selectedDate,
  highlight = 'week',
  weekDayCount = 7,
  classCountByDate,
  onPickDate
}: {
  month: Date;
  selectedWeekStart: Date;
  selectedDate?: Date;
  highlight?: 'week' | 'day';
  weekDayCount?: number;
  classCountByDate?: Record<string, number>;
  onPickDate: (date: Date) => void;
}) {
  const days = useMemo(() => buildMiniMonthDays(month), [month]);
  const selectedWeekKey = localDateText(selectedWeekStart);
  const selectedDayKey = selectedDate ? localDateText(selectedDate) : '';
  return (
    <div className="schedule-mini-calendar">
      {['一', '二', '三', '四', '五', '六', '日'].map((item) => <span className="schedule-mini-week" key={item}>{item}</span>)}
      {days.map((day) => {
        const weekSelected = highlight === 'week'
          && localDateText(startOfWeek(day.date)) === selectedWeekKey
          // 周一=1…周日=7；工作周下第 6、7 天不在视图里，不该点亮。
          && (day.date.getDay() === 0 ? 7 : day.date.getDay()) <= weekDayCount;
        const daySelected = highlight === 'day' && day.key === selectedDayKey;
        const count = classCountByDate?.[day.key] ?? 0;
        const className = [
          'schedule-mini-day',
          day.inMonth ? '' : 'is-outside',
          weekSelected ? 'is-week-selected' : '',
          daySelected ? 'is-day-selected' : '',
          day.isToday ? 'is-today' : '',
          count > 0 ? 'has-classes' : ''
        ].filter(Boolean).join(' ');
        return (
          <button
            type="button"
            className={className}
            key={day.key}
            title={count > 0 ? `${day.key} 有 ${count} 节课` : day.key}
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
  canManage,
  onEditClass,
  onCopyClass,
  onResizeClass
}: {
  item: TimelineLayoutItem;
  rangeStart: number;
  canManage: boolean;
  onEditClass: (record: ScheduleClass) => void;
  onCopyClass?: (record: ScheduleClass) => void;
  onResizeClass?: (record: ScheduleClass, endTime: string) => void;
}) {
  const start = timeToMinutes(item.startTime);
  const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
  const color = subjectColor(item.subject);
  const top = ((start - rangeStart) / timelineSlotMinutes) * timelineSlotHeight;
  const height = Math.max(34, ((end - start) / timelineSlotMinutes) * timelineSlotHeight - 4);
  const widthPct = item.widthPct ?? (100 / item.columns);
  const leftPct = item.leftPct ?? ((100 / item.columns) * item.column);
  const reserve = item.reserveRightPx ?? 0;
  // 幽灵条脱离百分比分列，直接从右边缘按序号排开，宽度固定。
  const isGhost = item.ghostIndex !== undefined;
  const width = isGhost ? `${canceledGhostWidth}px` : `calc(${widthPct}% - 4px - ${reserve}px)`;
  const left = isGhost ? 'auto' : `calc(${leftPct}% + 2px)`;
  const right = isGhost ? `${canceledGhostGap + (item.ghostIndex ?? 0) * (canceledGhostWidth + canceledGhostGap)}px` : undefined;
  const style = {
    top,
    height,
    left,
    right,
    width,
    '--subject-bg': color.bg,
    '--subject-border': color.border,
    '--subject-accent': color.accent,
    '--subject-text': color.text
  } as CSSProperties;
  // 块矮的时候会按高度逐档隐藏副标题/学生/标签，悬浮提示必须带上全部内容，
  // 否则被降级掉的信息就真的看不到了（点击块打开详情是另一条路径）。
  const title = [
    `${item.startTime}-${item.endTime}`,
    item.title,
    item.subtitle,
    item.meta,
    [item.classType, item.countText, item.status].filter(Boolean).join(' · ')
  ].filter(Boolean).join('\n');
  const className = [
    'schedule-timeline-block',
    `is-${item.kind}`,
    item.status === '已取消' ? 'is-canceled' : '',
    isGhost ? 'is-ghost' : ''
  ].filter(Boolean).join(' ');
  // 内容包一层 body：容器查询只能作用于容器的后代，不能作用于容器自身，
  // 所以内边距必须挂在这一层才能随块宽收窄（挂在外层块上会被静默忽略）。
  const renderBody = (extra?: ReactNode) => (
    <span className="schedule-timeline-body">
      <span className="schedule-timeline-time">{item.startTime}-{item.endTime}</span>
      <strong>{item.title}</strong>
      <span className="schedule-timeline-subtitle">{item.subtitle}</span>
      <small>{item.meta}</small>
      <span className="schedule-timeline-tags">
        {item.classType && <Tag>{item.classType}</Tag>}
        {item.countText && <Tag>{item.countText}</Tag>}
        {item.status && <Tag color={item.status === '已取消' ? 'default' : item.status === '待确认' ? 'gold' : 'green'}>{item.status}</Tag>}
        {item.kind === 'class' && <RecurrenceMark item={item.record as ScheduleClass} />}
      </span>
      {extra}
    </span>
  );

  // 已取消课程只画一根窄条：说明「这个时段原本有课、现在空出来了」，
  // 细节走悬浮提示，不占正常课程的可读宽度。
  if (isGhost) {
    return <div className={className} style={style} title={`已取消\n${title}`} />;
  }

  // 静音的背景带：只画底色，文字全部留给悬浮提示。
  if (item.quiet) {
    return <div className={className} style={style} title={title} />;
  }

  // 可排时段背景带自带一行贴顶标签，不走课程块那套多行降级。
  // 带子经常从早上一路铺到中午，而时间轴打开时会自动滚到第一节课，
  // 于是带子的顶部（唯一写着「这是什么」的那一行）被滚出视野，
  // 剩下的就是一大块没有任何说明、又因为 pointer-events:none 连悬浮提示都出不来的色块。
  // 标签用 position:sticky 跟着滚动停在可视区顶部：不管从哪一段看进来，都能读到
  // 「这段时间谁有空」。
  if (item.kind === 'availability') {
    return (
      <div className={className} style={style} title={title}>
        <span className="schedule-availability-label">
          <span className="schedule-availability-headline">
            <span className="schedule-timeline-time">{item.startTime}-{item.endTime}</span>
            <strong>{item.title}</strong>
          </span>
          {item.subtitle && <small>{item.subtitle}</small>}
        </span>
      </div>
    );
  }

  // 被折叠的课程不会丢失：点开可以看到完整列表，并直接进入任意一节课。
  if (item.kind === 'overflow') {
    const hidden = item.hiddenItems ?? [];
    return (
      <Popover
        trigger="click"
        placement="right"
        title={`${item.startTime}-${item.endTime} 还有 ${hidden.length} 节课`}
        content={(
          <div className="schedule-overflow-list">
            {hidden.map((hiddenItem) => (
              <button
                type="button"
                key={hiddenItem.id}
                className="schedule-overflow-entry"
                onClick={() => {
                  if (hiddenItem.kind === 'class') onEditClass(hiddenItem.record as ScheduleClass);
                }}
              >
                <strong>{hiddenItem.title}</strong>
                <span>{hiddenItem.startTime}-{hiddenItem.endTime} · {hiddenItem.subtitle}</span>
                {hiddenItem.meta && <small>{hiddenItem.meta}</small>}
              </button>
            ))}
          </div>
        )}
      >
        <button type="button" className={className} style={style} title={`还有 ${hidden.length} 节课，点击查看`}>
          <span className="schedule-timeline-body">
            <strong>{item.title}</strong>
          </span>
        </button>
      </Popover>
    );
  }

  if (item.kind === 'class') {
    const record = item.record as ScheduleClass;
    const editable = canManage && record.status !== '已取消';
    const block = (
      <button
        type="button"
        className={className}
        style={style}
        title={title}
        draggable={editable}
        onDragStart={(event) => event.dataTransfer.setData('text/schedule-class-id', record.id)}
        onClick={() => editable ? onEditClass(record) : undefined}
      >
        {renderBody(
          // 下沿拉伸改时长。整块拖动只能平移时段、时长不变，
          // 而「这节课上到几点」是排课时经常要单独调的一件事。
          editable && onResizeClass ? (
            <span
              className="schedule-timeline-resize"
              title="拖动下沿调整课程时长"
              onClick={(event) => event.stopPropagation()}
              onPointerDown={(event) => {
                event.stopPropagation();
                event.preventDefault();
                startTimelineResize(event, record, onResizeClass);
              }}
            />
          ) : undefined
        )}
      </button>
    );
    if (!editable || !onCopyClass) return block;
    // 右键复制：把这节课的课程、老师、学生、班型原样带进新建表单，
    // 只需要改时间就能再排一节。重复录入一模一样的信息是排课里最费时的部分。
    return (
      <Dropdown
        trigger={['contextMenu']}
        menu={{
          items: [
            { key: 'copy', icon: <CopyOutlined />, label: '复制这节课' },
            { key: 'edit', icon: <EditOutlined />, label: '调整这节课' }
          ],
          onClick: ({ key }) => key === 'copy' ? onCopyClass(record) : onEditClass(record)
        }}
      >
        {block}
      </Dropdown>
    );
  }

  return (
    <div className={className} style={style} title={title}>
      {renderBody()}
    </div>
  );
}

export function MonthScheduleBoard({
  month,
  classes,
  courseById,
  teacherById,
  canManage,
  onEditClass,
  onCopyClass,
  onMoveClass
}: {
  month: Date;
  classes: ScheduleClass[];
  courseById: CourseLookup;
  teacherById: Record<string, Teacher>;
  canManage: boolean;
  onEditClass: (record: ScheduleClass) => void;
  onCopyClass: (record: ScheduleClass) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
}) {
  // 之前这里写死 new Date() 且依赖数组为空，月视图翻页翻不动，永远停在当前月。
  const days = useMemo(() => buildMonthDays(month), [month]);
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
                lessonDate: day.key,
                label: day.label
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
              ) : (() => {
                const sorted = sortByStartTime(dayClasses);
                // 一格里最多列 4 条，其余折叠成 "+N"，与客户 Outlook 月视图一致。
                // 不折叠的话，最忙的那天会把整个月的行高一起撑高。
                const visible = sorted.length > monthCellMaxLessons
                  ? sorted.slice(0, monthCellMaxLessons - 1)
                  : sorted;
                const hidden = sorted.slice(visible.length);
                return (
                  <>
                    {visible.map((item) => (
                      <MonthClassEntry
                        key={item.id}
                        item={item}
                        course={courseById[item.courseId]}
                        teacher={teacherById[item.teacherId]}
                        canManage={canManage}
                        onEditClass={onEditClass}
                        onCopyClass={onCopyClass}
                      />
                    ))}
                    {hidden.length > 0 && (
                      <Popover
                        trigger="click"
                        placement="right"
                        title={`${day.label} 还有 ${hidden.length} 节课`}
                        content={(
                          <div className="month-overflow-list">
                            {hidden.map((item) => (
                              <MonthClassEntry
                                key={item.id}
                                item={item}
                                course={courseById[item.courseId]}
                                teacher={teacherById[item.teacherId]}
                                canManage={canManage}
                                onEditClass={onEditClass}
                                onCopyClass={onCopyClass}
                              />
                            ))}
                          </div>
                        )}
                      >
                        <button type="button" className="month-overflow-toggle">+{hidden.length}</button>
                      </Popover>
                    )}
                  </>
                );
              })()}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// 拖动课程块下沿改结束时间。
//
// 用 pointer 事件而不是 HTML5 drag：drag 那套是给「把东西挪到别处」用的，
// 拉伸要的是连续跟随鼠标的实时反馈，而且和块本身的 draggable 会互相抢事件。
// setPointerCapture 保证鼠标划出块外仍然跟手。
//
// 只在松手时回调一次：拉伸过程中每 30 分钟就发一次请求，等于把一次调课
// 变成十几次写库，还会被后端的冲突校验一路拒绝。
function startTimelineResize(
  event: React.PointerEvent<HTMLElement>,
  record: ScheduleClass,
  onResizeClass: (record: ScheduleClass, endTime: string) => void
) {
  const target = event.currentTarget;
  const startY = event.clientY;
  const originalEnd = timeToMinutes(record.endTime);
  const startMinute = timeToMinutes(record.startTime);
  let latestEnd = originalEnd;

  const compute = (clientY: number) => {
    const deltaSlots = Math.round((clientY - startY) / timelineSlotHeight);
    // 最短一个档位：不允许把课拉成零长度或负长度。
    const next = originalEnd + deltaSlots * timelineSlotMinutes;
    return Math.min(24 * 60, Math.max(startMinute + timelineSlotMinutes, next));
  };

  const move = (moveEvent: PointerEvent) => {
    latestEnd = compute(moveEvent.clientY);
    // 拉伸过程中只动这一块的高度，不重排整个网格——重排会让块在手底下跳。
    const block = target.closest('.schedule-timeline-block') as HTMLElement | null;
    if (block) {
      block.style.height = `${((latestEnd - startMinute) / timelineSlotMinutes) * timelineSlotHeight}px`;
    }
  };

  const finish = () => {
    target.removeEventListener('pointermove', move);
    target.removeEventListener('pointerup', finish);
    target.removeEventListener('pointercancel', finish);
    if (latestEnd !== originalEnd) onResizeClass(record, formatMinute(latestEnd));
  };

  target.setPointerCapture(event.pointerId);
  target.addEventListener('pointermove', move);
  target.addEventListener('pointerup', finish);
  target.addEventListener('pointercancel', finish);
}

// 重复课次的标记。客户的 Outlook 里每个重复课次右下角都有这个回环图标。
//
// 这不只是好看：拖动重复课次时会弹「仅此课次 / 此课次及后续 / 整个系列」，
// 用户事先必须能看出哪节是重复课，否则那个弹窗会显得莫名其妙。
// 已单独调整过的课次（detached）用断开的图标区分——它已经不跟随系列了，
// 拖它不会问范围，标记必须诚实反映这一点。
export function RecurrenceMark({ item }: { item: ScheduleClass }) {
  if (!item.seriesId) return null;
  if (item.detached) {
    return (
      <Tooltip title="重复课程中已单独调整过的一节，不再跟随系列">
        <DisconnectOutlined className="schedule-recurrence-mark is-detached" />
      </Tooltip>
    );
  }
  return (
    <Tooltip title="重复课程的一节，调整时会询问影响范围">
      <RetweetOutlined className="schedule-recurrence-mark" />
    </Tooltip>
  );
}

// 月视图的单个课次。排版对标客户的 Outlook：一行放完
// 「开始时间 · 教师 · 年级 · 科目短标签 · 学生」，超宽省略，完整内容挂 title。
//
// 之前这里是四行 grid（时间/科目年级/教师/学生各一行），一条占 4 行高，
// 一格连两条都放不下——那才是月视图放不下几节课的真正原因，
// 光加折叠不压排版等于把阈值设成 1。
function MonthClassEntry({
  item,
  course,
  teacher,
  canManage,
  onEditClass,
  onCopyClass
}: {
  item: ScheduleClass;
  course?: Course;
  teacher?: Teacher;
  canManage: boolean;
  onEditClass: (record: ScheduleClass) => void;
  onCopyClass: (record: ScheduleClass) => void;
}) {
  const editable = canManage && item.status !== '已取消';
  const teacherName = resourceLaneTeacherName(item.teacherId, item.teacherName, teacher);
  const grade = course?.grade ?? '';
  const subject = scheduleClassSubject(item, course ? { [course.id]: course } : {});
  // 学科用短标签（Eng/Math/Geo/…），这是客户能把一节课塞进一行的关键。
  const subjectText = subject ? subjectShortLabel(subject) : '';
  const students = item.students.map(studentDisplayName).join('、');
  const palette = subjectColor(subject || item.courseName);
  const full = [item.startTime, teacherName, grade, subjectText, students].filter(Boolean).join(' · ');

  const entry = (
    <button
      type="button"
      className={`month-class ${item.status === '已取消' ? 'is-canceled' : ''}`}
      title={full}
      draggable={editable}
      style={{ '--subject-color': palette.accent, '--subject-bg': palette.bg } as CSSProperties}
      onDragStart={(event) => event.dataTransfer.setData('text/schedule-class-id', item.id)}
      onClick={() => editable ? onEditClass(item) : undefined}
    >
      <span className="month-class-time">{item.startTime}</span>
      <span className="month-class-text">
        {[teacherName, grade, subjectText].filter(Boolean).join(' ')}
        {students && <em>{students}</em>}
      </span>
      <RecurrenceMark item={item} />
    </button>
  );
  if (!editable) return entry;
  // 右键复制在月视图同样可用：月视图是「看全貌顺手补一节」的场景，
  // 复制现有课比从空表单重填一遍快得多。
  return (
    <Dropdown
      trigger={['contextMenu']}
      menu={{
        items: [
          { key: 'copy', icon: <CopyOutlined />, label: '复制这节课' },
          { key: 'edit', icon: <EditOutlined />, label: '调整这节课' }
        ],
        onClick: ({ key }) => key === 'copy' ? onCopyClass(item) : onEditClass(item)
      }}
    >
      {entry}
    </Dropdown>
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
    { title: '时间', width: 180, render: (_, record) => `${record.lessonDate} ${record.startTime}-${record.endTime}` },
    { title: '班型', dataIndex: 'classType', width: 90 },
    { title: '学生', render: (_, record) => tagList(record.students.map(studentDisplayName), 'blue') },
    // 成班状态：人数够不够、有没有被取消。
    { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={value === '已取消' ? 'default' : value === '待确认' ? 'gold' : 'green'}>{value}</Tag> },
    // 审核状态：管理员认不认。与上一列是两个维度，不能合并显示。
    { title: '审核', width: 150, render: (_, record) => auditStatusTag(record) }
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
  // 课程和可排时间里存了一份冗余的老师名，老师改名后不回写，于是同一个人在不同位置显示成两个名字。
  // 传进来的 teacher 是老师档案，有就以它为准；查不到（老师已删、历史课程还在）才退回冗余名。
  const name = teacher?.name || teacherName;
  const scope = teacher ? teacherScopeText(teacher) : '';
  if (scope) return `教师：${name} · ${scope}`;
  return course?.grade ? `教师：${name} · ${course.grade}` : `教师：${name}`;
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
    return [...availabilityItems, ...classItems];
  });
}

// 课程和可排时间里都存了一份冗余的老师名（teacherName / ownerName），老师改名后不会回写，
// 于是同一条泳道可能出现两个名字：表头拿冗余名、块内副标题走 teacherById 拿到的是新名。
// 泳道表头以老师档案为准，冗余名只在档案查不到时兜底（老师被删了但历史课程还在）。
export function resourceLaneTeacherName(teacherId: string, fallbackName: string, teacher?: Teacher) {
  if (!teacherId) return '未指定老师';
  return teacher?.name || fallbackName || '未指定老师';
}

export function formatDayTitle(date: Date) {
  return `${date.getFullYear()} 年 ${date.getMonth() + 1} 月 ${date.getDate()} 日 ${weekLabel(date.getDay() === 0 ? 7 : date.getDay())}`;
}

// buildResourceLanes 把一天的课程/推荐方案/可排时间摊到「老师」或「教室」泳道上。
// 课程和推荐方案复用 buildTimelineItems 生成，标题、副标题、meta 的拼法与周视图共用一套，
// 避免两个视图各写一份、以后改文案漏掉一边。
//
// 可排时间不能走 buildTimelineItems：它内部的 aggregateAvailabilitySlots 会把
// 「同一时段的多个老师」合并成一条、只留下第一个人的 record，那样合并后的条目
// 只会落进第一个老师的泳道，其余老师的空闲就凭空消失了。所以这里按原始 slot 逐条分发，
// 落到各自泳道后再由 layoutOverlappingItems 合并成该泳道的背景带。
// 按老师分泳道时，块里那句「教师：张三 · 五年级/英文 · 学生：…」有一半是废话——
// 这一列的表头写的就是张三。而它恰恰是块里最长的一行，把真正要看的学生挤到看不见。
// 只保留学生。
function resourceLaneMeta(item: TimelineItem) {
  if (item.kind === 'class') {
    return studentSummaryText((item.record as ScheduleClass).students);
  }
  return item.meta;
}

function studentSummaryText(students: { name: string; grade?: string }[]) {
  if (!students || students.length === 0) return '暂无学生';
  return studentDisplayNames(students);
}

export function buildResourceLanes(
  dayOfWeek: number,
  availabilitySlots: AvailabilitySlot[],
  classes: ScheduleClass[],
  courseById: CourseLookup,
  teacherById: Record<string, Teacher>,
  studentById: Record<string, Student>
): ResourceLane[] {
  const syntheticDay: WeekDay = {
    key: `resource-${dayOfWeek}`,
    date: new Date(),
    day: 0,
    dayOfWeek,
    weekLabel: weekLabel(dayOfWeek),
    label: weekLabel(dayOfWeek)
  };
  const primaryItems = buildTimelineItems(
    [syntheticDay],
    {},
    { [dayOfWeek]: classes },
    courseById,
    teacherById,
    studentById
  );

  const lanes = new Map<string, ResourceLane>();
  const ensureLane = (key: string, title: string, subtitle: string, droppable = true) => {
    const existing = lanes.get(key);
    if (existing) return existing;
    const lane: ResourceLane = { key, title, subtitle, droppable, items: [] };
    lanes.set(key, lane);
    return lane;
  };

  primaryItems.forEach((item) => {
    const record = item.record as ScheduleClass;
    const teacherId = record.teacherId || '';
    const teacher = teacherById[teacherId];
    const key = teacherId || unassignedLaneKey;
    // 泳道表头只放名字，任教范围放副标题。不要用 teacherDisplay：
    // 它返回的是「教师：张三 · 五年级/英文」，在「这一列就是这个老师」的语境下，
    // 前缀是废话、后半段和副标题重复，还把名字挤出可视区。
    ensureLane(key, resourceLaneTeacherName(teacherId, record.teacherName, teacher), teacher ? teacherScopeText(teacher) : '')
      .items.push({ ...item, meta: resourceLaneMeta(item) });
  });

  const teacherSlots = availabilitySlots.filter((slot) => slot.ownerType === 'teacher');
  const studentSlots = availabilitySlots.filter((slot) => slot.ownerType === 'student');

  teacherSlots.forEach((slot) => {
    const teacher = teacherById[slot.ownerId];
    const lane = ensureLane(slot.ownerId, resourceLaneTeacherName(slot.ownerId, slot.ownerName, teacher), teacher ? teacherScopeText(teacher) : '');
    lane.items.push({
      id: slot.id,
      kind: 'availability',
      dayOfWeek,
      startTime: slot.startTime,
      endTime: slot.endTime,
      subject: '老师可授课',
      title: '老师可授课',
      // 这条带子垫在课程下层，课程之间的缝隙里会透出它的文字，而在老师泳道里
      // 「是哪位老师」表头已经写了、「可排课时间」是废话，全是冗余。
      // 直接静音成一片底色（图例已经说明绿色＝老师可授课），跟 Google Calendar
      // 的工作时间底色一个处理；具体时段仍可悬浮查看。
      subtitle: '',
      meta: '',
      quiet: true,
      record: slot
    });
  });

  const sorted = Array.from(lanes.values()).sort((left, right) => {
    // 未指定老师的兜底泳道排在最后，正常老师按名字排。
    if (left.key === unassignedLaneKey) return 1;
    if (right.key === unassignedLaneKey) return -1;
    return left.title.localeCompare(right.title, 'zh');
  });

  // 学生的可上课时间是跨老师的，挂在任何一条老师泳道上都不对，也不该在每条泳道里重复画一遍。
  // 单独给一条末尾泳道：排课时先在老师泳道里找空档，再横向对一眼学生这一列能不能接上。
  if (studentSlots.length > 0) {
    const studentLane = ensureLane(studentAvailabilityLaneKey, '学生可上课', '跨老师参考', false);
    aggregateAvailabilitySlots(studentSlots, teacherById, studentById).forEach((group) => {
      studentLane.items.push({
        id: group.id,
        kind: 'availability',
        dayOfWeek,
        startTime: group.startTime,
        endTime: group.endTime,
        subject: '学生可上课',
        title: '学生可上课',
        subtitle: group.subtitle,
        meta: '可排课时间',
        countText: group.countText,
        record: group.record
      });
    });
    sorted.push(studentLane);
  }

  return sorted;
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

// 可上课/可授课时段是背景参考信息（没有任何点击行为），不该和课程抢横向空间。
// 它铺满整列垫在课程下层，重叠的合并成一条：原来它和课程分左右车道，
// 一列 139px 时自己只剩 10px 宽，只画得出虚线边框、一个字也放不下，
// 同时还白白占掉课程五分之一的宽度。
// columnLimit 用来硬性限定一格里最多渲染几块。周视图按客户要求固定为 3
// （对标 Outlook：单格最多三个课时，超出折叠成加号）；日视图的泳道不传，
// 仍按列宽推算——那里一个泳道就是一个老师，同时段重叠本身就是排课错误，
// 数量极少，用宽度决定更合适。
export function layoutOverlappingItems(items: TimelineItem[], columnWidth = 0, columnLimit?: number) {
  const availability = items.filter((item) => item.kind === 'availability');
  const primary = items.filter((item) => item.kind !== 'availability');
  // 已取消的课不该和在排的课抢横向空间：一节真课旁边挂着两节已取消的，
  // 三等分之后每块只剩三分之一宽、课程名全被截断，而泳道表头算的又是「1 节课」，
  // 看到的和数到的对不上。它们退成右侧的窄幽灵条，保留时间位置和可查性，
  // 但不再参与在排课程的分列。
  const canceled = primary.filter((item) => item.status === '已取消');
  const live = primary.filter((item) => item.status !== '已取消');

  const availabilityBands: TimelineLayoutItem[] = mergeAvailabilityBands(availability).map((item) => ({
    ...item,
    column: 0,
    columns: 1,
    leftPct: 0,
    widthPct: 100
  }));

  // 幽灵条之间互相错开，同一时段有几节已取消就并排几条，不会互相盖住。
  const ghostItems: TimelineLayoutItem[] = layoutOverlappingGroup(canceled, maxCanceledGhosts).map((item) => ({
    ...item,
    ghostIndex: item.column
  }));
  const reserveRightPx = ghostItems.length > 0
    ? canceledGhostGap + Math.min(maxCanceledGhosts, Math.max(...ghostItems.map((item) => item.column + 1))) * (canceledGhostWidth + canceledGhostGap)
    : 0;

  if (live.length === 0) {
    return [...availabilityBands, ...ghostItems].sort((left, right) => timeToMinutes(left.startTime) - timeToMinutes(right.startTime));
  }

  // 可读列数按扣掉幽灵条之后的净宽算，否则末列会被挤到读不出课程名。
  const widthLimit = maxColumnsForWidth(Math.max(0, columnWidth - reserveRightPx));
  const effectiveLimit = columnLimit ? Math.min(columnLimit, widthLimit) : widthLimit;
  const liveItems = layoutOverlappingGroup(live, effectiveLimit).map((item) => ({
    ...item,
    leftPct: (100 / item.columns) * item.column,
    widthPct: 100 / item.columns,
    // 只有末列需要让位；前面几列本来就够不到右边缘。
    reserveRightPx: item.column === item.columns - 1 ? reserveRightPx : 0
  }));
  return [...availabilityBands, ...ghostItems, ...liveItems].sort((left, right) => timeToMinutes(left.startTime) - timeToMinutes(right.startTime));
}

// mergeAvailabilityBands 把时间上重叠的可排时段合并成一条背景带。
// 合并后标题显示条数，具体是谁放进副标题和 title 悬浮提示，信息不丢。
export function mergeAvailabilityBands(items: TimelineItem[]) {
  if (items.length === 0) return [];
  const sorted = [...items].sort((left, right) => {
    const diff = timeToMinutes(left.startTime) - timeToMinutes(right.startTime);
    return diff || timeToMinutes(left.endTime) - timeToMinutes(right.endTime);
  });
  const bands: TimelineItem[] = [];
  let group: TimelineItem[] = [];
  let groupEnd = -1;

  const flush = () => {
    if (group.length === 0) return;
    if (group.length === 1) {
      bands.push(group[0]);
    } else {
      const start = Math.min(...group.map((item) => timeToMinutes(item.startTime)));
      const end = Math.max(...group.map((item) => timeToMinutes(item.endTime)));
      const first = group[0];
      const detail = group
        .map((item) => [item.title, item.subtitle].filter(Boolean).join(' '))
        .filter(Boolean);
      bands.push({
        ...first,
        id: `availability-band-${first.id}`,
        startTime: formatMinute(start),
        endTime: formatMinute(end),
        // 合并带说的是「师生填报的可上课时间」，标题里把这层意思写全，
        // 免得被当成已经排好的课。
        title: `可上课时间 · ${group.length} 段`,
        subtitle: detail.join('、'),
        meta: '',
        countText: undefined,
        status: undefined
      });
    }
    group = [];
    groupEnd = -1;
  };

  sorted.forEach((item) => {
    const start = timeToMinutes(item.startTime);
    const end = Math.max(timeToMinutes(item.endTime), start + timelineSlotMinutes);
    if (group.length > 0 && start >= groupEnd) flush();
    group.push(item);
    groupEnd = Math.max(groupEnd, end);
  });
  flush();
  return bands;
}

export function layoutOverlappingGroup(items: TimelineItem[], maxColumns = maxTimelineColumns) {
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
    const rawColumns = Math.max(1, columnEnds.length);

    if (rawColumns <= maxColumns) {
      laidOut.forEach((item) => result.push({ ...item, columns: rawColumns }));
      group = [];
      groupEnd = -1;
      return;
    }

    // 超出可读上限：保留前几列正常显示，其余全部折叠进最后一列的 "+N"。
    // 被折叠的课程不会丢失，点击 "+N" 可以看到完整列表。
    // visibleColumns 至少为 1，否则 maxColumns 落到 1 时会把整组课程全部折叠、
    // 一节都不显示，比压窄更糟。
    const visibleColumns = Math.max(1, maxColumns - 1);
    const renderedColumns = visibleColumns + 1;
    const visible = laidOut.filter((item) => item.column < visibleColumns);
    const hidden = laidOut.filter((item) => item.column >= visibleColumns);
    visible.forEach((item) => result.push({ ...item, columns: renderedColumns }));

    const hiddenStart = Math.min(...hidden.map((item) => timeToMinutes(item.startTime)));
    const hiddenEnd = Math.max(...hidden.map((item) => timeToMinutes(item.endTime)));
    const first = hidden[0];
    result.push({
      ...first,
      id: `overflow-${first.id}`,
      kind: 'overflow',
      startTime: formatMinute(hiddenStart),
      endTime: formatMinute(hiddenEnd),
      title: `+${hidden.length}`,
      subtitle: '',
      meta: '',
      status: undefined,
      classType: undefined,
      countText: undefined,
      column: visibleColumns,
      columns: renderedColumns,
      hiddenItems: hidden.map(({ column, columns, ...rest }) => {
        void column;
        void columns;
        return rest;
      })
    });

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

// 时间轴范围 = 默认窗口 ∪ 实际内容。
//
// 不能改成「紧贴内容」：这条时间轴同时是拖拽落点，范围收紧到只包住现有课程后，
// 就再也没法把一节课往更早的时间挪了——这也是当初写死 08:00-22:00 的原因。
// 但写死同样有问题：22:00 之后开始的课算出来的 top 超过 boardHeight，
// 会直接渲染到列外面，用户看不到这节课，还以为那个时段是空的。
//
// 所以按内容单向外扩：早于默认起点的往前扩，晚于默认终点的往后扩，
// 两端都对齐到整小时，保证刻度线还是整点。空日子仍然是默认窗口。
export function buildTimelineRange(items: TimelineItem[]) {
  let start = defaultTimelineStart;
  let end = defaultTimelineEnd;
  for (const item of items) {
    const itemStart = timeToMinutes(item.startTime);
    const itemEnd = timeToMinutes(item.endTime);
    if (Number.isFinite(itemStart)) start = Math.min(start, Math.floor(itemStart / 60) * 60);
    if (Number.isFinite(itemEnd)) end = Math.max(end, Math.ceil(itemEnd / 60) * 60);
  }
  return {
    start: Math.max(0, start),
    // 24:00 是一天的上界；跨零点的课不在业务范围内（课外辅导不跨天），
    // 真出现脏数据也钉在 24:00，不让它把整块画布拉到几千像素高。
    end: Math.min(24 * 60, Math.max(end, start + 60))
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

// 课程块配色统一走学科元数据（见 utils/subject-colors）。
//
// 「老师可授课」「学生可上课」不是学科，是可上课时间底纹用的伪类目，
// 它们要的是「一眼看出这是背景参考、不是已排的课」，所以留在这里单独给色，
// 不进学科元数据——否则运营会在学科配色表里看到两条没法解释的条目。
const availabilityPalettes: Record<string, SubjectPalette> = {
  老师可授课: { bg: '#eef9f1', border: '#b8e2c4', accent: '#32975a', text: '#19542f' },
  学生可上课: { bg: '#eef6ff', border: '#b9d8fb', accent: '#347fc4', text: '#1e4d78' }
};

export function subjectColor(subject: string): SubjectPalette {
  return availabilityPalettes[subject] ?? subjectPalette(subject);
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
    return hasFilters ? `没有符合筛选条件的${gradeText}，可清空筛选查看全部课程。` : '还没有课程，可先创建待确认课程锁定时间段。';
  }
  const confirmedCount = items.filter((item) => item.status === '已确认').length;
  const pendingCount = items.filter((item) => item.status === '待确认').length;
  const canceledCount = items.filter((item) => item.status === '已取消').length;
  return `当前显示 ${items.length} 节${gradeText}：待确认 ${pendingCount} 节、已确认 ${confirmedCount} 节、已取消 ${canceledCount} 节。全部已确认课程 ${totalConfirmedCount} 节。`;
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

export function uniqueScheduleSubjects(classes: ScheduleClass[], courseById: CourseLookup) {
  const values = [
    ...classes.map((item) => scheduleClassSubject(item, courseById)),
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

export function scheduleClassPayload(
  record: ScheduleClass,
  target: ScheduleMoveTarget = { lessonDate: record.lessonDate, label: record.lessonDate }
): ScheduleClassFormValues {
  return {
    courseId: record.courseId,
    teacherId: record.teacherId,
    campusId: record.campusId || 'campus-main',
    roomName: record.roomName,
    classType: record.classType,
    durationMinutes: record.durationMinutes,
    startTime: target.startTime ?? record.startTime,
    endTime: target.endTime ?? record.endTime,
    startDate: target.lessonDate,
    studentIds: record.students.map((student) => student.id),
    expectedStudentCount: record.expectedStudentCount,
    reservationNote: record.reservationNote
  };
}

// dayCount=5 就是「工作周」（周一至周五）。周视图仍然是 7 天：
// 校外教培周末恰恰是排课高峰，默认砍掉周六日会把最忙的两天藏起来，
// 所以工作周是额外一个视图，不是把周视图改窄。
export function buildWeekDays(weekStart: Date, dayCount = 7): WeekDay[] {
  return Array.from({ length: dayCount }, (_, index) => {
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

// findNearestClassDate 从 fromDate 向两边找最近一个有课的日期，用于日视图的空态兜底：
// 「今天没课」本身不是有用的信息，「最近的排课在周三」才是。没有它，用户只能盲点翻页。
// 课程是按周重复的（dayOfWeek + startDate/endDate 区间），所以只能逐天试 scheduleClassOccursOn，
// 没法直接从记录里读出日期。窗口取 ±maxDays 天，找不到就返回 null（真的一节课都没有）。
// 同距离时优先未来：排课是往前看的动作，跳到下周三比跳回上周三更符合预期。
export function findNearestClassDate(classes: ScheduleClass[], fromDate: Date, maxDays = 60) {
  const active = classes.filter((item) => item.status !== '已取消');
  if (active.length === 0) return null;
  const hits = (date: Date) => active.filter((item) => scheduleClassOccursOn(item, date));
  for (let offset = 1; offset <= maxDays; offset += 1) {
    for (const date of [addDays(fromDate, offset), addDays(fromDate, -offset)]) {
      const found = hits(date);
      if (found.length > 0) return { date, count: found.length };
    }
  }
  return null;
}

export function sortByStartTime<T extends { startTime: string }>(items: T[]) {
  return [...items].sort((left, right) => left.startTime.localeCompare(right.startTime));
}


// 审核状态标签。被驳回时把理由挂在 Tooltip 上——老师得知道要改什么。
export function auditStatusTag(record: ScheduleClass) {
  if (record.auditStatus === '已驳回') {
    return (
      <Tooltip title={record.auditReason || '未填写理由'}>
        <Tag color="red">已驳回</Tag>
      </Tooltip>
    );
  }
  if (record.auditStatus === '待审核') return <Tag color="gold">待审核</Tag>;
  return <Tag color="green">已通过</Tag>;
}

// 待审核队列。管理员在这里逐条裁决老师提交的排课。
export function pendingReviewColumns(
  courseById: CourseLookup,
  onApprove: (record: ScheduleClass) => void,
  onReject: (record: ScheduleClass, reason: string) => void,
  reviewing: boolean
): TableColumnsType<ScheduleClass> {
  return [
    { title: '课程', width: 150, render: (_, record) => courseSubjectGradeText(courseById[record.courseId], record.courseName) },
    { title: '老师', dataIndex: 'teacherName', width: 120 },
    { title: '时间', width: 180, render: (_, record) => `${record.lessonDate} ${record.startTime}-${record.endTime}` },
    { title: '学生', render: (_, record) => tagList(record.students.map(studentDisplayName), 'blue') },
    {
      title: '提交人',
      width: 120,
      render: (_, record) => record.createdBy || <Typography.Text type="secondary">-</Typography.Text>
    },
    {
      // 老师排课时可上课时间只是软提醒，越界能提交。这里必须让管理员看见，
      // 否则「越过谁的时间」这件事到审核环节就丢了。
      title: '提示',
      width: 220,
      render: (_, record) => record.overrideNote
        ? <Tooltip title={record.overrideNote}><Tag color="orange">超出可上课时间</Tag></Tooltip>
        : <Typography.Text type="secondary">-</Typography.Text>
    },
    {
      title: '操作',
      width: 150,
      render: (_, record) => (
        <Space size={4}>
          <Button size="small" type="primary" loading={reviewing} onClick={() => onApprove(record)}>通过</Button>
          <RejectButton record={record} onReject={onReject} reviewing={reviewing} />
        </Space>
      )
    }
  ];
}

// 驳回必须填理由，所以单独做成带输入框的气泡，而不是一个直接生效的按钮。
function RejectButton({
  record,
  onReject,
  reviewing
}: {
  record: ScheduleClass;
  onReject: (record: ScheduleClass, reason: string) => void;
  reviewing: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState('');
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setReason('');
      }}
      trigger="click"
      title="驳回理由"
      content={(
        <Space direction="vertical" style={{ width: 240 }}>
          <Input.TextArea
            rows={3}
            maxLength={255}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="例如：该时段教室已排满"
          />
          <Button
            size="small"
            danger
            loading={reviewing}
            disabled={reason.trim().length === 0}
            onClick={() => {
              onReject(record, reason.trim());
              setOpen(false);
              setReason('');
            }}
          >
            确认驳回
          </Button>
        </Space>
      )}
    >
      <Button size="small" danger>驳回</Button>
    </Popover>
  );
}

// 重复排课表单。对应后端 ScheduleRepeat（RRULE 的一个子集）。
//
// 结束方式必须二选一：只按次数、或只按日期。两个都留着的话，
// 「每周一次、共 10 次、到 6 月 30 日止」这种输入没人说得清以谁为准，
// 后端也会直接拒绝。
export function RepeatFields({
  enabled,
  onToggle,
  startDate,
  form
}: {
  enabled: boolean;
  onToggle: (next: boolean) => void;
  startDate?: string;
  form: FormInstance;
}) {
  const freq = Form.useWatch(['repeat', 'freq'], form) ?? 'weekly';
  const endMode = Form.useWatch(['repeat', 'endMode'], form) ?? 'count';
  const interval = Form.useWatch(['repeat', 'interval'], form) ?? 1;
  const byDay = Form.useWatch(['repeat', 'byDay'], form) as number[] | undefined;
  const startWeekday = weekdayOfDateText(startDate);

  return (
    <>
      <Form.Item label="重复">
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
          <Switch
            checked={enabled}
            onChange={onToggle}
            checkedChildren="重复排课"
            unCheckedChildren="只排一节"
          />
          {enabled && (
            <>
              <Space.Compact block>
                <Form.Item name={['repeat', 'freq']} noStyle initialValue="weekly">
                  <Select
                    style={{ width: '34%' }}
                    options={[
                      { label: '按周', value: 'weekly' },
                      { label: '按日', value: 'daily' }
                    ]}
                  />
                </Form.Item>
                <Form.Item name={['repeat', 'interval']} noStyle initialValue={1}>
                  <InputNumber
                    min={1}
                    max={52}
                    style={{ width: '33%' }}
                    addonBefore="每"
                    addonAfter={freq === 'daily' ? '天' : '周'}
                  />
                </Form.Item>
                <Form.Item name={['repeat', 'endMode']} noStyle initialValue="count">
                  <Select
                    style={{ width: '33%' }}
                    options={[
                      { label: '按次数结束', value: 'count' },
                      { label: '按日期结束', value: 'until' }
                    ]}
                  />
                </Form.Item>
              </Space.Compact>

              {freq === 'weekly' && (
                <Form.Item name={['repeat', 'byDay']} noStyle>
                  <Select
                    mode="multiple"
                    allowClear
                    style={{ width: '100%' }}
                    placeholder={startWeekday ? `默认跟随首节：${weekLabel(startWeekday)}` : '默认跟随首节上课日期'}
                    options={weekOptions}
                  />
                </Form.Item>
              )}

              {endMode === 'count' ? (
                <Form.Item name={['repeat', 'count']} noStyle initialValue={4}>
                  <InputNumber min={1} max={200} style={{ width: '100%' }} addonBefore="共" addonAfter="节" />
                </Form.Item>
              ) : (
                <Form.Item name={['repeat', 'until']} noStyle>
                  <Input placeholder="重复到（含）2026-06-30" />
                </Form.Item>
              )}

              <Typography.Text type="secondary">
                {repeatSummaryText(freq, interval, byDay, startWeekday, endMode)}
                。不会自动跳过节假日和寒暑假，需要跳过的课次请排完后单独取消。单次最多生成 200 节。
              </Typography.Text>
            </>
          )}
        </Space>
      </Form.Item>
    </>
  );
}

function repeatSummaryText(
  freq: string,
  interval: number,
  byDay: number[] | undefined,
  startWeekday: number,
  endMode: string
) {
  const every = interval > 1 ? `每 ${interval} ` : '每';
  if (freq === 'daily') {
    return `${every}${interval > 1 ? '天' : '天'}排一节`;
  }
  const days = byDay && byDay.length > 0 ? byDay : (startWeekday ? [startWeekday] : []);
  const dayText = days.length > 0 ? days.map(weekLabel).join('、') : '首节所在星期';
  return `${every}${interval > 1 ? '周' : '周'}的 ${dayText} 各排一节，${endMode === 'count' ? '到指定节数为止' : '到指定日期为止'}`;
}

// endMode 是纯前端的开关，后端只认 until / count 二选一，不认这个字段。
export type RepeatFormValues = {
  freq?: string;
  interval?: number;
  byDay?: number[];
  endMode?: string;
  count?: number;
  until?: string;
};

// 表单里的重复规则转成后端要的形状。
export function buildRepeatPayload(values: RepeatFormValues): ScheduleRepeatValues {
  const freq: ScheduleRepeatValues['freq'] = values.freq === 'daily' ? 'daily' : 'weekly';
  const payload: ScheduleRepeatValues = {
    freq,
    interval: values.interval && values.interval > 0 ? values.interval : 1
  };
  if (freq === 'weekly' && values.byDay && values.byDay.length > 0) payload.byDay = values.byDay;
  if (values.endMode === 'until') {
    payload.until = (values.until ?? '').trim();
  } else {
    payload.count = values.count && values.count > 0 ? values.count : 1;
  }
  return payload;
}
