export function teachingPlanEntryQuery(pathname: string, search: string): string {
  if (pathname !== '/teaching-plans' && pathname !== '/login') return '';
  const id = new URLSearchParams(search).get('plan');
  return id && /^[a-zA-Z0-9_.-]{1,64}$/.test(id) && !id.includes('..') ? `?plan=${encodeURIComponent(id)}` : '';
}
