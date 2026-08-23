import type { AvailabilitySlot, ScheduleCandidate, ScheduleClass } from '../../types/starline';

export const weekOptions = [
  { label: '周一', value: 1 },
  { label: '周二', value: 2 },
  { label: '周三', value: 3 },
  { label: '周四', value: 4 },
  { label: '周五', value: 5 },
  { label: '周六', value: 6 },
  { label: '周日', value: 7 }
];

export type CandidateLevel = 'full' | 'ready' | 'short';

export function weekLabel(day: number) {
  return weekOptions.find((item) => item.value === day)?.label ?? `周${day}`;
}

export function localDateText(date: Date) {
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${date.getFullYear()}-${month}-${day}`;
}

export function startOfWeek(date: Date) {
  const result = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const offset = (result.getDay() + 6) % 7;
  result.setDate(result.getDate() - offset);
  return result;
}

export function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

export function addDays(date: Date, days: number) {
  const result = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  result.setDate(result.getDate() + days);
  return result;
}

export function addMonths(date: Date, months: number) {
  return new Date(date.getFullYear(), date.getMonth() + months, 1);
}

export function formatWeekRange(weekStart: Date) {
  const weekEnd = addDays(weekStart, 6);
  if (weekStart.getFullYear() === weekEnd.getFullYear()) {
    return `${weekStart.getFullYear()} 年 ${weekStart.getMonth() + 1} 月 ${weekStart.getDate()} 日 - ${weekEnd.getMonth() + 1} 月 ${weekEnd.getDate()} 日`;
  }
  return `${weekStart.getFullYear()} 年 ${weekStart.getMonth() + 1} 月 ${weekStart.getDate()} 日 - ${weekEnd.getFullYear()} 年 ${weekEnd.getMonth() + 1} 月 ${weekEnd.getDate()} 日`;
}

export function classCapacity(classType?: string) {
  const match = /1V([1-4])/.exec(classType || '');
  return match ? Number(match[1]) : 4;
}

export function minimumStudentCount(classType: string) {
  if (classType === '1V1') return 1;
  if (classType === '1V2') return 2;
  if (classType === '1V3') return 2;
  if (classType === '1V4') return 3;
  return 1;
}

export function candidateLevel(candidate: ScheduleCandidate): CandidateLevel {
  if (candidate.studentCount >= candidate.capacity) return 'full';
  if (candidate.studentCount >= minimumStudentCount(candidate.classType)) return 'ready';
  return 'short';
}

export function candidateLevelMeta(level: CandidateLevel) {
  if (level === 'full') return { label: '满班推荐', color: 'green' };
  if (level === 'ready') return { label: '可开班', color: 'blue' };
  return { label: '人数不足', color: 'default' };
}

export function scheduleClassOccursOn(item: ScheduleClass, date: Date) {
  const dayOfWeek = date.getDay() === 0 ? 7 : date.getDay();
  if (item.dayOfWeek !== dayOfWeek) return false;
  const dateText = localDateText(date);
  return (!item.startDate || dateText >= item.startDate) && (!item.endDate || dateText <= item.endDate);
}

// 后端建课时除了查冲突，还硬性要求老师和每个学生都有一条覆盖该星期、时段、日期的可上课时间
// （见 buildScheduleClass 的 teacherAvailable/studentAvailable），否则直接 400。
// 这里用同一套判定在提交前提示，避免「周视图明明是空的却说时间冲突」。
export function availabilityCovers(slot: AvailabilitySlot, dayOfWeek: number, startTime: string, endTime: string, date: string) {
  if (slot.dayOfWeek !== dayOfWeek) return false;
  if (slot.startTime > startTime || slot.endTime < endTime) return false;
  if (date) {
    if (slot.startDate && date < slot.startDate) return false;
    if (slot.endDate && date > slot.endDate) return false;
  }
  return true;
}

export function isClockText(value?: string) {
  return /^\d{2}:\d{2}$/.test(value || '');
}
