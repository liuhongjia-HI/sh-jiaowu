import { useSearchParams } from 'react-router-dom';
import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react';

// Only list context belongs here; never persist selected records or form drafts.
export function useListState<T>(key: string, initial: T | (() => T)): [T, Dispatch<SetStateAction<T>>];
export function useListState<T = undefined>(key: string): [T | undefined, Dispatch<SetStateAction<T | undefined>>];
export function useListState<T>(key: string, initial?: T | (() => T)) {
  const storageKey = (() => {
    try {
      const user = JSON.parse(localStorage.getItem('starline_admin_user') || '{}');
      return `starline:list-state:v1:${user.userId || 'anonymous'}:${key}`;
    } catch { return `starline:list-state:v1:anonymous:${key}`; }
  })();
  const [value, setValue] = useState<T | undefined>(() => {
    const fallback = typeof initial === 'function' ? (initial as () => T)() : initial;
    try {
      const raw = sessionStorage.getItem(storageKey);
      if (!raw) return fallback;
      const saved = JSON.parse(raw).value;
      if (fallback instanceof Date) {
        const date = new Date(saved);
        return (Number.isNaN(date.getTime()) ? fallback : date) as T;
      }
      if (fallback !== undefined && fallback !== null && (Array.isArray(fallback) ? !Array.isArray(saved) : typeof saved !== typeof fallback)) return fallback;
      return saved as T;
    } catch { return fallback; }
  });
  useEffect(() => {
    try { sessionStorage.setItem(storageKey, JSON.stringify({ value })); } catch { /* Storage may be unavailable; the list remains usable. */ }
  }, [storageKey, value]);
  return [value, setValue] as const;
}

export function useListPagination(key: string, pageSize: number, filters = '') {
  const [current, setCurrent] = useListState(`${key}:page`, 1);
  const previousFilters = useRef(filters);
  useEffect(() => {
    if (previousFilters.current !== filters) setCurrent(1);
    previousFilters.current = filters;
  }, [filters, setCurrent]);
  return { current, pageSize, showSizeChanger: false, onChange: setCurrent };
}

export function clampListPage(page: number, total: number, size = 10) {
  return Math.min(Math.max(1, Number.isFinite(page) ? page : 1), Math.max(1, Math.ceil(total / size)));
}

// Explicit entry URLs take precedence. Detail IDs are deliberately excluded.
export function useListSearchParams(key: string, filterKeys: string[]) {
  const [saved, setSaved] = useListState<Record<string, string>>(`${key}:url`, {});
  const [params, setParams] = useSearchParams(saved);
  const serialized = JSON.stringify(Object.fromEntries(filterKeys.filter(key => params.has(key)).map(key => [key, params.get(key)!])));
  useEffect(() => { setSaved(JSON.parse(serialized)); }, [serialized, setSaved]);
  return [params, setParams] as const;
}
