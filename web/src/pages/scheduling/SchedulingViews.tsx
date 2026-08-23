import { CalendarOutlined, CloseCircleOutlined, DeleteOutlined, EditOutlined, LeftOutlined, PlusOutlined, ReloadOutlined, RightOutlined, SaveOutlined, SettingOutlined, TableOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Drawer, Empty, Form, Input, InputNumber, Modal, Popconfirm, Popover, Select, Skeleton, Space, Table, Tag, Typography, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
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
  expectedStudentCount: number;
  reservationNote?: string;
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
type TimelineKind = 'class' | 'candidate' | 'availability' | 'overflow';
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
  record: ScheduleClass | ScheduleCandidate | AvailabilitySlot;
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
      .filter((item) => item.kind === 'class' || item.kind === 'candidate')
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
      result[day.dayOfWeek] = layoutOverlappingItems(timelineItems.filter((item) => item.dayOfWeek === day.dayOfWeek), dayColumnWidth);
      return result;
    }, {});
  }, [weekDays, timelineItems, dayColumnWidth]);

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

      <div ref={scrollRef} className="schedule-timeline-scroll">
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
  candidatesByDay,
  classesByDay,
  courseById,
  teacherById,
  studentById,
  candidateRequest,
  selectedCandidateId,
  emptyTips,
  canManage,
  nearestClassDate,
  onPreviousDay,
  onNextDay,
  onToday,
  onJumpToDate,
  onPickCandidate,
  onEditClass,
  onMoveClass,
  onCreateClass
}: {
  loading: boolean;
  selectedDate: Date;
  nearestClassDate: { date: Date; count: number } | null;
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
  onPreviousDay: () => void;
  onNextDay: () => void;
  onToday: () => void;
  onJumpToDate: (date: Date) => void;
  onPickCandidate: (record: ScheduleCandidate) => void;
  onEditClass: (record: ScheduleClass) => void;
  onMoveClass: (record: ScheduleClass, target: ScheduleMoveTarget) => void;
  onCreateClass: (dayOfWeek: number) => void;
}) {
  const dayOfWeek = selectedDate.getDay() === 0 ? 7 : selectedDate.getDay();
  const lanes = useMemo(
    () => buildResourceLanes(
      dayOfWeek,
      availabilityByDay[dayOfWeek] ?? [],
      candidatesByDay[dayOfWeek] ?? [],
      (classesByDay[dayOfWeek] ?? []).filter((item) => scheduleClassOccursOn(item, selectedDate)),
      courseById,
      teacherById,
      studentById
    ),
    [dayOfWeek, availabilityByDay, candidatesByDay, classesByDay, selectedDate, courseById, teacherById, studentById]
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
    if (candidateRequest) return `${dayText} 没有符合条件的排课结果。`;
    if (totalAvailabilityCount === 0) return `${dayText} 没有课程——目前还没有收到任何老师或学生填报的可上课时间，先把时间收上来才能匹配出可排方案。`;
    return `${dayText} 没有课程：已收到的 ${totalAvailabilityCount} 个可上课时段都不在这一天。`;
  }, [candidateRequest, selectedDate, totalAvailabilityCount]);

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
      .filter((item) => item.kind === 'class' || item.kind === 'candidate')
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
          {canManage && <Button icon={<PlusOutlined />} onClick={() => onCreateClass(dayOfWeek)}>新建课程</Button>}
        </Space>
        <div>
          <Typography.Title level={4}>{formatDayTitle(selectedDate)}</Typography.Title>
          <Typography.Text type="secondary">双击空白处新建课程；拖动课程可在本泳道内改时间，换老师或教室请点击课程编辑。</Typography.Text>
        </div>
      </div>

      {/* 空态要给出路。「这天没课」本身没有信息量——排课的主路径是「师生填报可上课时间
          → 匹配出可排方案 → 确认成班」，手动建课只是逃生通道。所以空态该回答的是
          「卡在这条链路的哪一环」：时间还没收上来，还是收上来了但没落在这一天。
          候选查找的提示（emptyTips）只在真的发起过查找时才展示：没查找时那句
          「先选择学科和年级」和「这天没课」无关，是误导。 */}
      {!hasAnyItem && (
        <div className="schedule-day-empty-bar">
          <span>{emptyReason}</span>
          {nearestClassDate && (
            <Button type="link" size="small" onClick={() => onJumpToDate(nearestClassDate.date)}>
              最近的排课在 {formatDayTitle(nearestClassDate.date)}（{nearestClassDate.count} 节），去看看
            </Button>
          )}
          {candidateRequest && emptyTips.length > 0 && (
            <div className="schedule-tip-list">
              {emptyTips.map((tip) => <div key={tip}>{tip}</div>)}
            </div>
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
                  dayOfWeek,
                  startTime,
                  endTime,
                  label: `${weekLabel(dayOfWeek)} ${startTime}-${endTime}`
                });
              }}
              onDoubleClick={(event) => {
                if (event.currentTarget === event.target && canManage) onCreateClass(dayOfWeek);
              }}
            >
              {rows.map((row) => <span className="schedule-time-line" key={row.minute} style={{ top: row.top }} />)}
              {lane.layout.map((item) => (
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
          ))}
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

// 迷你日历同时承担「跳转」和「这个月哪几天有课」的导航职责。
// 后者是日视图能用起来的前提：日视图一次只看一天，如果日历上看不出哪天有课，
// 打开就是空白的那天时，用户没有任何线索知道该往哪翻，只能盲点上一天/下一天。
//
// highlight 区分两种视图：周视图选中的是一整周，日视图选中的是具体某一天。
// 日视图下仍然高亮整周的话，点 19 号和点 21 号看起来完全一样，等于没有反馈。
export function MiniMonthCalendar({
  month,
  selectedWeekStart,
  selectedDate,
  highlight = 'week',
  classCountByDate,
  onPickDate
}: {
  month: Date;
  selectedWeekStart: Date;
  selectedDate?: Date;
  highlight?: 'week' | 'day';
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
        const weekSelected = highlight === 'week' && localDateText(startOfWeek(day.date)) === selectedWeekKey;
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
    isGhost ? 'is-ghost' : '',
    item.kind === 'candidate' && item.id === selectedCandidateId ? 'is-selected' : ''
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
                  if (hiddenItem.kind === 'candidate') onPickCandidate(hiddenItem.record as ScheduleCandidate);
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
        {renderBody()}
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
        {renderBody(<span className="schedule-timeline-action">确认这个时间</span>)}
      </button>
    );
  }

  return (
    <div className={className} style={style} title={title}>
      {renderBody()}
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
    { title: '状态', dataIndex: 'status', width: 100, render: (value) => <Tag color={value === '已取消' ? 'default' : value === '待确认' ? 'gold' : 'green'}>{value}</Tag> }
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
  if (item.kind === 'candidate') {
    return studentSummaryText((item.record as ScheduleCandidate).availableStudents);
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
  candidates: ScheduleCandidate[],
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
    { [dayOfWeek]: candidates },
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
    const record = item.record as ScheduleClass | ScheduleCandidate;
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
export function layoutOverlappingItems(items: TimelineItem[], columnWidth = 0) {
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
  const liveItems = layoutOverlappingGroup(live, maxColumnsForWidth(Math.max(0, columnWidth - reserveRightPx))).map((item) => ({
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
        // 「N 个可排时段」和图例里的「可排方案」（推荐课程）只差一个字，容易被读成推荐结果。
        // 合并带说的是「师生填报的可上课时间」，标题里就把这层意思写全。
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

export function buildTimelineRange(items: TimelineItem[]) {
  void items;
  return { start: defaultTimelineStart, end: defaultTimelineEnd };
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
    roomName: record.roomName,
    classType: record.classType,
    durationMinutes: record.durationMinutes,
    dayOfWeek: target.dayOfWeek,
    startTime: target.startTime ?? record.startTime,
    endTime: target.endTime ?? record.endTime,
    startDate: target.startDate ?? record.startDate,
    endDate: target.endDate ?? record.endDate,
    studentIds: record.students.map((student) => student.id),
    expectedStudentCount: record.expectedStudentCount,
    reservationNote: record.reservationNote
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
