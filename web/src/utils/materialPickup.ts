export function materialPickupQuery(pathname: string, search: string): string {
  if (pathname !== '/teacher-library' && pathname !== '/login') return '';
  const id = new URLSearchParams(search).get('downloadJob');
  return id && /^download-[a-f0-9]{32}$/.test(id) ? `?downloadJob=${encodeURIComponent(id)}` : '';
}
