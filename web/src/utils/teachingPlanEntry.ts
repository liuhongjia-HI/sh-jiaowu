export function teachingPlanEntryQuery(pathname: string, search: string): string {
  if (pathname !== '/teaching-plans' && pathname !== '/login') return '';
  const source = new URLSearchParams(search); const result = new URLSearchParams();
  const id = source.get('plan'); const notice = source.get('notice');
  if (id && /^[a-zA-Z0-9_.-]{1,64}$/.test(id) && !id.includes('..')) result.set('plan', id);
  if (notice && /^[a-f0-9]{64}$/.test(notice)) result.set('notice', notice);
  return result.size ? `?${result.toString()}` : '';
}
