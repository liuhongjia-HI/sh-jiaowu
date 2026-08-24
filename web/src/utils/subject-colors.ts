// 学科颜色的唯一来源。
//
// 在这之前排课页和资源页各写了一份调色板，同一门课在两处是不同颜色。
// 颜色现在由后端 settings 里的 subjectColors 维护（见 defaultSubjectColors），
// 运营改色不用发版；这里的 DEFAULT_SUBJECT_COLORS 与后端默认值保持一致，
// 只在设置还没拉回来时兜底，所以不会出现先闪一版旧色再跳成新色。

export type SubjectColorEntry = {
  subject: string;
  shortLabel: string;
  color: string;
  sortOrder: number;
};

// 与 learning-api 的 defaultSubjectColors 一一对应。客户长期用 Outlook 排课，
// 日历分类固定是 Eng / Math / Geo / Sci / CHN / His / Chem / Phy 这 8 个。
// 「综合科学」和「科学」面向不同年级、不会同时出现，共用 Sci 的颜色。
export const DEFAULT_SUBJECT_COLORS: SubjectColorEntry[] = [
  { subject: '英文', shortLabel: 'Eng', color: '#1A6FD4', sortOrder: 1 },
  { subject: '数学', shortLabel: 'Math', color: '#E8C400', sortOrder: 2 },
  { subject: '地理', shortLabel: 'Geo', color: '#3A9BBF', sortOrder: 3 },
  { subject: '科学', shortLabel: 'Sci', color: '#1B3FA8', sortOrder: 4 },
  { subject: '综合科学', shortLabel: 'Sci', color: '#1B3FA8', sortOrder: 5 },
  { subject: '语文', shortLabel: 'CHN', color: '#A855D8', sortOrder: 6 },
  { subject: '历史', shortLabel: 'His', color: '#8B5A2B', sortOrder: 7 },
  { subject: '化学', shortLabel: 'Chem', color: '#E8730C', sortOrder: 8 },
  { subject: '物理', shortLabel: 'Phy', color: '#C2185B', sortOrder: 9 }
];

let activeColors: SubjectColorEntry[] = DEFAULT_SUBJECT_COLORS;

// 把系统设置里的 subjectColors 装载进来。解析失败就保持当前这份，
// 一个手滑写坏的设置值不该让整个课表变成灰色。
export function loadSubjectColors(raw?: string) {
  if (!raw || !raw.trim()) return;
  try {
    const parsed = JSON.parse(raw) as SubjectColorEntry[];
    if (!Array.isArray(parsed) || parsed.length === 0) return;
    const valid = parsed.filter((item) => item && item.subject && isHexColor(item.color));
    if (valid.length > 0) activeColors = valid;
  } catch {
    // 保持默认值
  }
}

export function subjectColorEntries() {
  return [...activeColors].sort((left, right) => left.sortOrder - right.sortOrder);
}

function findEntry(subject: string) {
  const normalized = (subject || '').trim();
  const direct = activeColors.find((item) => item.subject === normalized);
  if (direct) return direct;
  // 「英语」和「英文」在历史数据里都出现过，按同一门处理。
  if (normalized === '英语') return activeColors.find((item) => item.subject === '英文');
  return undefined;
}

// 未配置的学科（比如运营新加了一门还没配色）按名称散列到一组中性色，
// 保证同一门课每次渲染都是同一个颜色，而不是刷新一次换一个。
const fallbackColors = ['#6B7280', '#4B5563', '#57534E', '#475569'];

export function subjectAccent(subject: string) {
  const entry = findEntry(subject);
  if (entry) return entry.color;
  const name = (subject || '').trim();
  if (!name) return fallbackColors[0];
  const index = [...name].reduce((sum, char) => sum + char.charCodeAt(0), 0) % fallbackColors.length;
  return fallbackColors[index];
}

export function subjectShortLabel(subject: string) {
  return findEntry(subject)?.shortLabel ?? (subject || '').trim();
}

export type SubjectPalette = { bg: string; border: string; accent: string; text: string };

// 课程块要四个色值，但只让运营挑一个主色——其余按固定比例推导，
// 免得运营还要理解底色、边框、文字色三者之间的关系。
export function subjectPalette(subject: string): SubjectPalette {
  const accent = subjectAccent(subject);
  return {
    accent,
    bg: mixWithWhite(accent, 0.88),
    border: mixWithWhite(accent, 0.58),
    text: darken(accent, 0.55)
  };
}

function isHexColor(value?: string) {
  return typeof value === 'string' && /^#[0-9a-fA-F]{6}$/.test(value.trim());
}

function toRgb(hex: string) {
  const value = hex.trim().replace('#', '');
  return {
    r: parseInt(value.slice(0, 2), 16),
    g: parseInt(value.slice(2, 4), 16),
    b: parseInt(value.slice(4, 6), 16)
  };
}

function toHex({ r, g, b }: { r: number; g: number; b: number }) {
  const part = (value: number) => Math.max(0, Math.min(255, Math.round(value))).toString(16).padStart(2, '0');
  return `#${part(r)}${part(g)}${part(b)}`;
}

// ratio 是白色的占比：0.88 表示 88% 白 + 12% 主色，得到很浅的底色。
function mixWithWhite(hex: string, ratio: number) {
  const { r, g, b } = toRgb(hex);
  return toHex({
    r: r + (255 - r) * ratio,
    g: g + (255 - g) * ratio,
    b: b + (255 - b) * ratio
  });
}

// 文字色统一压暗，保证在浅底上读得清。黄色这类本身很亮的主色
// 直接拿来当文字色会完全看不见，所以压暗是必须的，不是可选优化。
function darken(hex: string, ratio: number) {
  const { r, g, b } = toRgb(hex);
  return toHex({ r: r * (1 - ratio), g: g * (1 - ratio), b: b * (1 - ratio) });
}
