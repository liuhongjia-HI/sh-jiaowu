import { useEffect, useRef, useState } from 'react';
import { Alert, Button, Card, QRCode, Space, Typography } from 'antd';
import { resolveApiUrl } from '../services/http';

type Job = { studentName: string; courseName?: string; scope: { subject: string }; count: number; size: number; expiresAt?: string; materials?: { fileName: string; unit: string; chapter: string; lesson: string }[] };
type Session = { challenge: string; browserKey: string; expiresAt: string };
async function publicRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(resolveApiUrl(path), options);
  const result = await response.json();
  if (!response.ok || result.code !== 0) throw new Error(result.message || '领取失败，请重试');
  return result.data;
}
export default function StudentDownloadPickup() {
  const sizeText = (size: number) => size >= 1048576 ? `${(size / 1048576).toFixed(1)} MB` : `${Math.ceil(size / 1024)} KB`;
  const jobId = new URLSearchParams(window.location.search).get('job') || '';
  const [session, setSession] = useState<Session>();
  const [job, setJob] = useState<Job>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [pollRetry, setPollRetry] = useState(0);
  const revision = useRef(0);
  const mounted = useRef(true);
  const createSession = async () => {
    const current = ++revision.current;
    setLoading(true); setError(''); setJob(undefined); setSession(undefined);
    try {
      const result = await publicRequest<Session>('/download-pickups', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ jobId }) });
      if (mounted.current && current === revision.current) setSession(result);
    } catch (err) { if (mounted.current && current === revision.current) setError(err instanceof Error ? err.message : '无法生成二维码'); }
    finally { if (mounted.current && current === revision.current) setLoading(false); }
  };
  useEffect(() => {
    mounted.current = true;
    const timer = window.setTimeout(() => { if (/^download-[a-f0-9]{32}$/.test(jobId)) void createSession(); else setError('领取链接无效，请从小程序复制电脑领取链接。'); }, 0);
    return () => { mounted.current = false; revision.current++; window.clearTimeout(timer); };
  }, []);
  useEffect(() => {
    if (!session) return;
    let cancelled = false, timer: number;
    const poll = async () => {
      try {
        const data = await publicRequest<{ status: string; job?: Job }>(`/download-pickups/${session.challenge}`, { headers: { Authorization: `Bearer ${session.browserKey}` }, cache: 'no-store' });
        if (cancelled) return;
        if (data.status === 'approved' && data.job) { setJob(data.job); setError(''); return; }
        timer = window.setTimeout(poll, 2000);
      } catch (err) { if (!cancelled) setError(err instanceof Error ? err.message : '网络异常，请重试'); }
    };
    void poll();
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [session, pollRetry]);
  const download = async () => {
    if (!session || !job || downloading) return;
    setDownloading(true); setError('');
    try {
      const response = await fetch(resolveApiUrl(`/download-pickups/${session.challenge}/archive`), { headers: { Authorization: `Bearer ${session.browserKey}` } });
      if (!response.ok) { const result = await response.json(); throw new Error(result.message || '下载失败'); }
      const blob = await response.blob();
      const url = URL.createObjectURL(blob), link = document.createElement('a');
      link.href = url; link.download = `${job.scope.subject}课程讲义.zip`; link.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 60000);
    } catch (err) { setError(err instanceof Error ? err.message : '下载失败，请重试'); }
    finally { setDownloading(false); }
  };
  return <main className="student-download-pickup"><Card>
    <Typography.Title level={3}>课程资料 · 电脑领取</Typography.Title>
    {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 20 }} />}
    {!job && <>
      <Typography.Paragraph>在小程序「我的下载」点击「扫码确认电脑领取」，核对学生和资料后确认。</Typography.Paragraph>
      {session && !error && <div className="pickup-qr"><QRCode value={`starline-download:${session.challenge}`} size={224} /><Typography.Text type="secondary">等待手机确认 · 二维码有效期 10 分钟</Typography.Text></div>}
      <Space wrap>{session && error && <Button onClick={() => { setError(''); setPollRetry(value => value + 1); }}>重试查询</Button>}<Button loading={loading} disabled={loading || !/^download-[a-f0-9]{32}$/.test(jobId)} onClick={() => void createSession()}>刷新二维码</Button></Space>
    </>}
    {job && <>
      <Alert type="success" showIcon message="手机已确认，可以下载本次资料" />
      <Typography.Title level={4}>{job.studentName} · {job.courseName || job.scope.subject}</Typography.Title>
      <Typography.Paragraph>{job.count} 份文件 · {sizeText(job.size)}{job.expiresAt ? ` · 下载包有效至 ${new Date(job.expiresAt).toLocaleString()}` : ''}</Typography.Paragraph>
      <Button type="primary" size="large" loading={downloading} onClick={() => void download()}>下载 ZIP 文件</Button>
      <Button style={{ marginLeft: 12 }} onClick={() => void createSession()}>重新扫码确认</Button>
      <div className="pickup-file-list">{job.materials?.map((item, index) => <div key={index}><Typography.Text>{item.fileName}</Typography.Text><div><Typography.Text type="secondary">{[item.unit, item.chapter, item.lesson].filter(Boolean).join(' / ')}</Typography.Text></div></div>)}</div>
    </>}
  </Card></main>;
}
