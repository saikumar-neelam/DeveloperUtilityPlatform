'use client';

import { useEffect, useRef, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { timeAgo, cn } from '@/lib/utils';
import type { WebhookRequest, Endpoint, EndpointResponseConfig } from '@/lib/types';
import { MethodBadge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { CopyButton } from '@/components/ui/CopyButton';
import { RequestDrawer } from '@/components/RequestDrawer';
import { CodeExamples } from '@/components/CodeExamples';
import { WebhookSimulator } from '@/components/WebhookSimulator';

const METHODS = ['ALL', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE'];

export default function EndpointPage() {
  const router = useRouter();
  const { id } = useParams<{ id: string }>();

  const [requests, setRequests]   = useState<WebhookRequest[]>([]);
  const [selected, setSelected]   = useState<WebhookRequest | null>(null);
  const [loading, setLoading]     = useState(true);
  const [error, setError]         = useState('');
  const [pulse, setPulse]         = useState(false);
  const [connected, setConnected] = useState(false);
  const [expiresAt, setExpiresAt]       = useState<Date | null>(null);
  const [countdown, setCountdown]       = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting]           = useState(false);
  const seenIds = useRef<Set<string>>(new Set());

  // Filter / search
  const [methodFilter, setMethodFilter] = useState('ALL');
  const [search, setSearch]             = useState('');

  // Rename
  const [editingName, setEditingName] = useState(false);
  const [nameValue,   setNameValue]   = useState('');
  const nameInputRef = useRef<HTMLInputElement>(null);

  async function handleRename() {
    const trimmed = nameValue.trim();
    if (!trimmed || trimmed === endpoint?.name) { setEditingName(false); return; }
    try {
      await api.endpoints.rename(id, trimmed);
      setEndpoint(prev => prev ? { ...prev, name: trimmed } : prev);
    } catch { /* revert */ setNameValue(endpoint?.name ?? ''); }
    setEditingName(false);
  }

  // Response config panel
  const [endpoint, setEndpoint]         = useState<Endpoint | null>(null);
  const [respConfig, setRespConfig]     = useState<EndpointResponseConfig>({
    status: 200, content_type: 'application/json', headers: {}, body: '',
  });
  const [headerRows, setHeaderRows]     = useState<{ key: string; value: string }[]>([]);
  const [respOpen, setRespOpen]         = useState(false);
  const [saving, setSaving]             = useState(false);
  const [saveMsg, setSaveMsg]           = useState<{ ok: boolean; text: string } | null>(null);
  const [clearing, setClearing]         = useState(false);

  // Notification email
  const [notifyEmail, setNotifyEmail]   = useState('');
  const [notifySaving, setNotifySaving] = useState(false);
  const [notifyMsg, setNotifyMsg]       = useState<{ ok: boolean; text: string } | null>(null);

  const [hookUrl, setHookUrl] = useState(`/hook/${id}`);
  useEffect(() => {
    setHookUrl(`${window.location.origin}/hook/${id}`);
  }, [id]);

  // Load endpoint metadata
  useEffect(() => {
    api.endpoints.get(id).then(ep => {
      setEndpoint(ep);
      setNameValue(ep.name);
      const exp = new Date(ep.expires_at);
      // Year 9999 = "never expires"
      if (exp.getFullYear() >= 9999) {
        setCountdown('Never');
      } else {
        setExpiresAt(exp);
      }
      // Init response config from server values
      setRespConfig({
        status: ep.response_status || 200,
        content_type: ep.response_content_type || 'application/json',
        headers: ep.response_headers || {},
        body: ep.response_body || '',
      });
      setNotifyEmail(ep.notify_email || '');
      setHeaderRows(
        Object.entries(ep.response_headers || {}).map(([key, value]) => ({ key, value }))
      );
      try {
        const raw = localStorage.getItem('webhookdb_recent') ?? '[]';
        const list = JSON.parse(raw).filter((e: { id: string }) => e.id !== ep.id);
        list.unshift(ep);
        localStorage.setItem('webhookdb_recent', JSON.stringify(list.slice(0, 8)));
      } catch { /* ignore */ }
    }).catch(() => {});
  }, [id]);

  // Countdown ticker
  useEffect(() => {
    if (!expiresAt) return;
    const tick = () => {
      const ms = expiresAt.getTime() - Date.now();
      if (ms <= 0) { setCountdown('Expired'); return; }
      const h = Math.floor(ms / 3_600_000);
      const m = Math.floor((ms % 3_600_000) / 60_000);
      const s = Math.floor((ms % 60_000) / 1_000);
      setCountdown(h > 0 ? `${h}h ${m}m ${s}s` : `${m}m ${s}s`);
    };
    tick();
    const t = setInterval(tick, 1_000);
    return () => clearInterval(t);
  }, [expiresAt]);

  // Load existing requests
  useEffect(() => {
    api.requests.list(id)
      .then(data => {
        const list = data ?? [];
        list.forEach(r => seenIds.current.add(r.id));
        setRequests(list);
      })
      .catch(() => setError('Failed to load requests.'))
      .finally(() => setLoading(false));
  }, [id]);

  // SSE live feed
  useEffect(() => {
    const base = process.env.NEXT_PUBLIC_BACKEND_URL ?? '';
    const es = new EventSource(`${base}/api/endpoints/${id}/stream`);
    es.onopen = () => setConnected(true);
    es.onmessage = (e) => {
      try {
        const req: WebhookRequest = JSON.parse(e.data);
        if (seenIds.current.has(req.id)) return;
        seenIds.current.add(req.id);
        setRequests(prev => [req, ...prev]);
        setPulse(true);
      } catch { /* ignore */ }
    };
    es.onerror = () => setConnected(false);
    return () => es.close();
  }, [id]);

  useEffect(() => {
    if (pulse) {
      const t = setTimeout(() => setPulse(false), 600);
      return () => clearTimeout(t);
    }
  }, [pulse]);

  async function handleDelete() {
    setDeleting(true);
    try {
      await api.endpoints.delete(id);
      try {
        const raw = localStorage.getItem('webhookdb_recent') ?? '[]';
        const list = JSON.parse(raw).filter((e: { id: string }) => e.id !== id);
        localStorage.setItem('webhookdb_recent', JSON.stringify(list));
      } catch { /* ignore */ }
      router.push('/');
    } catch {
      setDeleting(false);
      setConfirmDelete(false);
    }
  }

  async function handleSaveResponse() {
    setSaving(true); setSaveMsg(null);
    const headers: Record<string, string> = {};
    headerRows.forEach(({ key, value }) => { if (key.trim()) headers[key.trim()] = value; });
    const config: EndpointResponseConfig = { ...respConfig, headers };
    try {
      await api.endpoints.updateResponse(id, config);
      setRespConfig(config);
      setSaveMsg({ ok: true, text: 'Saved' });
    } catch {
      setSaveMsg({ ok: false, text: 'Save failed' });
    } finally {
      setSaving(false);
      setTimeout(() => setSaveMsg(null), 2500);
    }
  }

  async function handleSaveNotify() {
    setNotifySaving(true); setNotifyMsg(null);
    try {
      await api.endpoints.updateNotify(id, notifyEmail.trim());
      setNotifyMsg({ ok: true, text: notifyEmail.trim() ? 'Saved' : 'Notifications off' });
    } catch {
      setNotifyMsg({ ok: false, text: 'Save failed' });
    } finally {
      setNotifySaving(false);
      setTimeout(() => setNotifyMsg(null), 2500);
    }
  }

  async function handleClearAll() {
    setClearing(true);
    try {
      await api.endpoints.deleteAllRequests(id);
      setRequests([]);
      seenIds.current.clear();
    } catch { /* ignore */ }
    setClearing(false);
  }

  // Filtered requests — search across method, content-type, headers, and body preview
  const filtered = requests.filter(req => {
    if (methodFilter !== 'ALL' && req.method !== methodFilter) return false;
    if (search.trim()) {
      const q = search.toLowerCase();
      if (req.method.toLowerCase().includes(q)) return true;
      if (req.content_type?.toLowerCase().includes(q)) return true;
      if (req.body_preview?.toLowerCase().includes(q)) return true;
      // search header keys and values
      for (const [key, vals] of Object.entries(req.headers ?? {})) {
        if (key.toLowerCase().includes(q)) return true;
        if (vals.some(v => v.toLowerCase().includes(q))) return true;
      }
      return false;
    }
    return true;
  });

  return (
    <>
      {/* Page header */}
      <div className="bg-white border-b border-slate-200 px-4 md:px-8 py-3 md:py-4 shrink-0">
        {/* Breadcrumb row */}
        <div className="flex items-center gap-2 mb-1.5">
          <button
            onClick={() => router.push('/dashboard')}
            className="text-slate-400 hover:text-slate-700 p-1 rounded-md hover:bg-slate-100 transition-colors"
          >
            <BackIcon />
          </button>
          <nav className="text-sm text-slate-500 flex items-center gap-1.5 min-w-0">
            <span className="hover:text-slate-900 cursor-pointer transition-colors shrink-0" onClick={() => router.push('/dashboard')}>
              Endpoints
            </span>
            <ChevronIcon />
            <span className="text-slate-900 font-medium font-mono text-xs truncate">{id}</span>
          </nav>
        </div>

        {/* Name + actions row */}
        <div className="flex items-start justify-between gap-2 ml-8">
          <div className="min-w-0 flex-1">
            {/* Editable name */}
            <div className="mb-1.5">
              {editingName ? (
                <input
                  ref={nameInputRef}
                  value={nameValue}
                  onChange={e => setNameValue(e.target.value)}
                  onBlur={handleRename}
                  onKeyDown={e => { if (e.key === 'Enter') handleRename(); if (e.key === 'Escape') { setEditingName(false); setNameValue(endpoint?.name ?? ''); } }}
                  className="text-base font-semibold text-slate-900 bg-transparent border-b-2 border-violet-500 outline-none w-full max-w-xs"
                  autoFocus
                />
              ) : (
                <button
                  onClick={() => { setEditingName(true); setTimeout(() => nameInputRef.current?.select(), 0); }}
                  className="group flex items-center gap-1.5 text-base font-semibold text-slate-900 hover:text-violet-700 transition-colors text-left"
                >
                  <span className="truncate max-w-[200px] md:max-w-none">{endpoint?.name ?? id}</span>
                  <PencilIcon />
                </button>
              )}
            </div>

            {/* Hook URL */}
            <div className="flex items-center gap-2 flex-wrap">
              <code className="text-xs font-mono text-slate-600 bg-slate-100 border border-slate-200 px-2 py-1 rounded-md truncate max-w-[180px] sm:max-w-xs md:max-w-none">
                {hookUrl}
              </code>
              <CopyButton text={hookUrl} label="Copy" />
              {countdown && (
                <span className={cn('text-xs',
                  countdown === 'Expired' ? 'text-red-500' :
                  countdown === 'Never'   ? 'text-emerald-600' : 'text-amber-600'
                )}>
                  {countdown === 'Never' ? 'Never expires' : countdown === 'Expired' ? 'Expired' : `${countdown}`}
                </span>
              )}
            </div>
          </div>

          {/* Action buttons */}
          <div className="flex items-center gap-1.5 shrink-0">
            <div className="flex items-center gap-1 text-xs text-slate-400 mr-1">
              <span className={cn(
                'w-1.5 h-1.5 rounded-full transition-all',
                connected ? 'bg-emerald-400' : 'bg-slate-300',
                pulse ? 'scale-150 bg-emerald-300' : '',
              )} />
              <span className="hidden sm:inline">{connected ? 'Live' : 'Connecting…'}</span>
            </div>
            <button
              onClick={handleClearAll}
              disabled={clearing || requests.length === 0}
              title="Clear all requests"
              className="flex items-center gap-1 text-xs text-slate-400 hover:text-amber-600 border border-slate-200 hover:border-amber-200 hover:bg-amber-50 px-2 py-1.5 rounded-lg transition-colors disabled:opacity-40"
            >
              <EraseIcon /> <span className="hidden sm:inline">{clearing ? 'Clearing…' : 'Clear'}</span>
            </button>
            <button
              onClick={() => setConfirmDelete(true)}
              title="Delete endpoint"
              className="flex items-center gap-1 text-xs text-slate-400 hover:text-red-600 border border-slate-200 hover:border-red-200 hover:bg-red-50 px-2 py-1.5 rounded-lg transition-colors"
            >
              <TrashIcon /> <span className="hidden sm:inline">Delete</span>
            </button>
          </div>
        </div>
      </div>

      <div className="px-4 md:px-8 py-4 md:py-6 flex-1">
        <CodeExamples hookUrl={hookUrl} />

        {/* Requests table */}
        <div className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden mt-4 md:mt-6">
          {/* Table header with search + filter */}
          <div className="px-4 py-3 bg-slate-50 border-b border-slate-200 space-y-2">
            <div className="flex items-center gap-2">
              <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wide shrink-0">Requests</h2>
              <span className="text-xs text-slate-400">{requests.length} captured</span>
              <div className="relative ml-auto">
                <SearchIcon className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
                <input
                  type="text"
                  placeholder="Search…"
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                  className="pl-8 pr-3 py-1.5 text-xs border border-slate-200 rounded-lg bg-white focus:outline-none focus:ring-1 focus:ring-violet-400 w-32 sm:w-44"
                />
              </div>
            </div>
            {/* Method filter pills */}
            <div className="flex gap-1 flex-wrap">
              {METHODS.map(m => (
                <button
                  key={m}
                  onClick={() => setMethodFilter(m)}
                  className={cn(
                    'px-2 py-1 rounded-md text-xs font-medium transition-colors',
                    methodFilter === m
                      ? 'bg-violet-600 text-white'
                      : 'bg-white border border-slate-200 text-slate-500 hover:border-slate-300',
                  )}
                >
                  {m}
                </button>
              ))}
            </div>
          </div>

          {/* Column headings — hidden on mobile */}
          {!loading && filtered.length > 0 && (
            <div className="hidden sm:grid grid-cols-[90px_1fr_140px_100px] gap-4 px-5 py-2 bg-slate-50 border-b border-slate-100 sticky top-0 z-10">
              {['Method', 'Request ID', 'Received', 'Action'].map(col => (
                <span key={col} className="text-xs font-medium text-slate-400 uppercase tracking-wide">{col}</span>
              ))}
            </div>
          )}

          <div className="overflow-y-auto" style={{ maxHeight: '600px' }}>
            {loading && (
              <div className="divide-y divide-slate-100">
                {[1, 2, 3].map(i => (
                  <div key={i} className="flex items-center gap-4 px-5 py-4 animate-pulse">
                    <div className="h-5 w-14 bg-slate-100 rounded" />
                    <div className="h-3.5 w-64 bg-slate-100 rounded" />
                    <div className="h-3.5 w-20 bg-slate-100 rounded ml-auto" />
                  </div>
                ))}
              </div>
            )}
            {!loading && error && (
              <div className="px-5 py-4 text-sm text-red-600">{error}</div>
            )}
            {!loading && !error && requests.length === 0 && (
              <div className="px-5 py-14 text-center">
                <p className="text-sm font-medium text-slate-600">Waiting for requests…</p>
                <p className="text-sm text-slate-400 mt-1">Use one of the examples above to send your first webhook.</p>
              </div>
            )}
            {!loading && !error && requests.length > 0 && filtered.length === 0 && (
              <div className="px-5 py-10 text-center text-sm text-slate-400">
                No requests match your filter.
              </div>
            )}
            {!loading && filtered.map((req, i) => (
              <div
                key={req.id}
                onClick={() => setSelected(req)}
                className={cn(
                  'cursor-pointer transition-colors group',
                  'flex sm:grid sm:grid-cols-[90px_1fr_140px_100px] gap-2 sm:gap-4 items-center px-4 sm:px-5 py-3 sm:py-3.5',
                  selected?.id === req.id ? 'bg-violet-50' : 'hover:bg-slate-50',
                  i < filtered.length - 1 ? 'border-b border-slate-100' : '',
                )}
              >
                <MethodBadge method={req.method} />
                <span className="text-xs font-mono text-slate-500 truncate flex-1 sm:flex-none">{req.id}</span>
                <span className="text-xs sm:text-sm text-slate-400 sm:text-slate-500 shrink-0">{timeAgo(req.created_at)}</span>
                <div className="hidden sm:flex justify-start" onClick={e => e.stopPropagation()}>
                  <Button variant="ghost" size="sm" onClick={() => api.requests.replay(req.id)} className="opacity-0 group-hover:opacity-100">
                    <ReplayIcon /> Replay
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Custom Response Config */}
        <div className="bg-white border border-slate-200 rounded-xl shadow-sm mt-6">
          <button
            onClick={() => setRespOpen(o => !o)}
            className="w-full flex items-center justify-between px-5 py-3.5 text-left"
          >
            <div className="flex items-center gap-3">
              <span className="text-xs font-semibold text-slate-500 uppercase tracking-wide">Response Configuration</span>
              <span className="text-xs text-slate-400">
                Customize what your endpoint returns to the sender
              </span>
            </div>
            <div className="flex items-center gap-2">
              <span className={cn(
                'text-xs font-mono px-2 py-0.5 rounded-full border',
                respConfig.status >= 200 && respConfig.status < 300 ? 'bg-emerald-50 text-emerald-700 border-emerald-200' :
                respConfig.status >= 400 ? 'bg-red-50 text-red-700 border-red-200' :
                'bg-slate-50 text-slate-600 border-slate-200',
              )}>
                {respConfig.status}
              </span>
              <ChevronDownIcon open={respOpen} />
            </div>
          </button>

          {respOpen && (
            <div className="px-5 pb-5 border-t border-slate-100 pt-4 space-y-4">
              {/* Status + Content-Type row */}
              <div className="flex gap-4">
                <div className="w-32">
                  <label className="block text-xs font-medium text-slate-500 mb-1">Status Code</label>
                  <input
                    type="number"
                    min={100} max={599}
                    value={respConfig.status}
                    onChange={e => setRespConfig(c => ({ ...c, status: parseInt(e.target.value) || 200 }))}
                    className="w-full px-3 py-1.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-violet-400 font-mono"
                  />
                </div>
                <div className="flex-1">
                  <label className="block text-xs font-medium text-slate-500 mb-1">Content-Type</label>
                  <select
                    value={respConfig.content_type}
                    onChange={e => setRespConfig(c => ({ ...c, content_type: e.target.value }))}
                    className="w-full px-3 py-1.5 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-violet-400 bg-white"
                  >
                    <option>application/json</option>
                    <option>text/plain</option>
                    <option>text/html</option>
                    <option>application/xml</option>
                    <option>application/x-www-form-urlencoded</option>
                  </select>
                </div>
              </div>

              {/* Response body */}
              <div>
                <label className="block text-xs font-medium text-slate-500 mb-1">Response Body</label>
                <textarea
                  rows={4}
                  value={respConfig.body}
                  onChange={e => setRespConfig(c => ({ ...c, body: e.target.value }))}
                  placeholder='{"status":"ok"}'
                  className="w-full px-3 py-2 text-sm font-mono border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-violet-400 resize-none"
                />
              </div>

              {/* Custom headers */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-xs font-medium text-slate-500">Response Headers</label>
                  <button
                    onClick={() => setHeaderRows(r => [...r, { key: '', value: '' }])}
                    className="text-xs text-violet-600 hover:text-violet-800 font-medium"
                  >
                    + Add header
                  </button>
                </div>
                {headerRows.length === 0 && (
                  <p className="text-xs text-slate-400">No custom headers</p>
                )}
                <div className="space-y-1.5">
                  {headerRows.map((row, i) => (
                    <div key={i} className="flex gap-2 items-center">
                      <input
                        type="text"
                        placeholder="Header-Name"
                        value={row.key}
                        onChange={e => setHeaderRows(rows => rows.map((r, j) => j === i ? { ...r, key: e.target.value } : r))}
                        className="flex-1 px-2.5 py-1.5 text-xs font-mono border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-violet-400"
                      />
                      <span className="text-slate-300 text-xs">:</span>
                      <input
                        type="text"
                        placeholder="value"
                        value={row.value}
                        onChange={e => setHeaderRows(rows => rows.map((r, j) => j === i ? { ...r, value: e.target.value } : r))}
                        className="flex-1 px-2.5 py-1.5 text-xs font-mono border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-violet-400"
                      />
                      <button
                        onClick={() => setHeaderRows(rows => rows.filter((_, j) => j !== i))}
                        className="text-slate-300 hover:text-red-400 transition-colors p-1"
                      >
                        <XSmallIcon />
                      </button>
                    </div>
                  ))}
                </div>
              </div>

              {/* Save */}
              <div className="flex items-center justify-end gap-3 pt-1">
                {saveMsg && (
                  <span className={cn('text-xs font-medium', saveMsg.ok ? 'text-emerald-600' : 'text-red-500')}>
                    {saveMsg.text}
                  </span>
                )}
                <Button variant="primary" size="sm" onClick={handleSaveResponse} disabled={saving}>
                  {saving ? 'Saving…' : 'Save response'}
                </Button>
              </div>
            </div>
          )}
        </div>

        <WebhookSimulator hookUrl={hookUrl} />

        {/* Email Notifications */}
        <div className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden mt-4">
          <div className="px-4 py-3 flex items-center gap-2 border-b border-slate-100">
            <BellIcon />
            <span className="text-xs font-semibold text-slate-500 uppercase tracking-wide">Email Notifications</span>
          </div>
          <div className="px-4 py-4 flex items-center gap-3">
            <input
              type="email"
              value={notifyEmail}
              onChange={e => setNotifyEmail(e.target.value)}
              placeholder="you@example.com — leave blank to disable"
              className="flex-1 text-sm border border-slate-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-violet-500 focus:border-transparent"
            />
            {notifyMsg && (
              <span className={cn('text-xs font-medium whitespace-nowrap', notifyMsg.ok ? 'text-emerald-600' : 'text-red-500')}>
                {notifyMsg.text}
              </span>
            )}
            <Button variant="primary" size="sm" onClick={handleSaveNotify} disabled={notifySaving}>
              {notifySaving ? 'Saving…' : 'Save'}
            </Button>
          </div>
          <p className="px-4 pb-3 text-xs text-slate-400">
            Get an email every time this endpoint receives a webhook. Leave blank to disable.
          </p>
        </div>
      </div>

      <RequestDrawer
        request={selected}
        onClose={() => setSelected(null)}
        onReplay={(reqId, targetUrl) => api.requests.replay(reqId, targetUrl)}
      />

      {/* Delete confirmation modal */}
      {confirmDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-xl shadow-xl border border-slate-200 p-6 w-full max-w-sm mx-4">
            <div className="flex items-center gap-3 mb-3">
              <div className="w-9 h-9 rounded-full bg-red-50 flex items-center justify-center shrink-0">
                <span className="text-red-600"><TrashIcon /></span>
              </div>
              <h2 className="text-sm font-semibold text-slate-900">Delete endpoint?</h2>
            </div>
            <p className="text-sm text-slate-500 mb-5">
              This will permanently delete the endpoint and all captured requests. This action cannot be undone.
            </p>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setConfirmDelete(false)}
                disabled={deleting}
                className="px-4 py-2 text-sm text-slate-600 hover:text-slate-900 border border-slate-200 hover:bg-slate-50 rounded-lg transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-lg transition-colors disabled:opacity-60 flex items-center gap-2"
              >
                {deleting ? <SpinnerIcon /> : <TrashIcon />}
                {deleting ? 'Deleting…' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function BackIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="m15 18-6-6 6-6" />
    </svg>
  );
}

function ChevronIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}

function ChevronDownIcon({ open }: { open: boolean }) {
  return (
    <svg
      width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      className={cn('text-slate-400 transition-transform', open && 'rotate-180')}
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}

function ReplayIcon() {
  return (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
      <path d="M3 3v5h5" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
      <path d="M10 11v6M14 11v6" />
      <path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" />
    </svg>
  );
}

function BellIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-slate-400">
      <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
      <path d="M13.73 21a2 2 0 0 1-3.46 0" />
    </svg>
  );
}

function EraseIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M20 20H7L3 16l10-10 7 7-1.5 1.5" />
      <path d="M6.5 17.5l-3-3" />
    </svg>
  );
}

function SpinnerIcon() {
  return <span className="w-3.5 h-3.5 rounded-full border-2 border-white/40 border-t-white animate-spin inline-block" />;
}

function SearchIcon({ className }: { className?: string }) {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className={className}>
      <circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" />
    </svg>
  );
}

function XSmallIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  );
}
function PencilIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      className="opacity-0 group-hover:opacity-100 transition-opacity text-slate-400">
      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
    </svg>
  );
}
