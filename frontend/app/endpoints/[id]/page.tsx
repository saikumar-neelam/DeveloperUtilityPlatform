'use client';

import { useEffect, useRef, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { timeAgo, cn } from '@/lib/utils';
import type { WebhookRequest } from '@/lib/types';
import { MethodBadge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { CopyButton } from '@/components/ui/CopyButton';
import { RequestDrawer } from '@/components/RequestDrawer';
import { CodeExamples } from '@/components/CodeExamples';

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

  // Start with relative path (matches SSR), update to full URL after hydration.
  const [hookUrl, setHookUrl] = useState(`/hook/${id}`);
  useEffect(() => {
    setHookUrl(`${window.location.origin}/hook/${id}`);
  }, [id]);

  // Load endpoint metadata (for expiry) and save to localStorage.
  useEffect(() => {
    api.endpoints.get(id)
      .then(ep => {
        const exp = new Date(ep.expires_at);
        setExpiresAt(exp);
        // Keep localStorage in sync.
        try {
          const raw = localStorage.getItem('webhookdb_recent') ?? '[]';
          const list = JSON.parse(raw).filter((e: { id: string }) => e.id !== ep.id);
          list.unshift(ep);
          localStorage.setItem('webhookdb_recent', JSON.stringify(list.slice(0, 8)));
        } catch { /* ignore */ }
      })
      .catch(() => { /* non-fatal — page still works */ });
  }, [id]);

  // Countdown ticker.
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

  // Load existing requests once on mount.
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

  // Subscribe to live SSE stream — prepend new requests as they arrive.
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
      } catch {
        // ignore malformed frames
      }
    };

    es.onerror = () => setConnected(false);

    return () => es.close();
  }, [id]);

  // Reset pulse indicator
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
      // Remove from localStorage.
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

  return (
    <>
      {/* Page header */}
      <div className="bg-white border-b border-slate-200 px-8 py-4 shrink-0">
        <div className="flex items-center gap-3 mb-1">
          <button
            onClick={() => router.push('/dashboard')}
            className="text-slate-400 hover:text-slate-700 p-1 rounded-md hover:bg-slate-100 transition-colors"
          >
            <BackIcon />
          </button>
          <nav className="text-sm text-slate-500 flex items-center gap-1.5">
            <span
              className="hover:text-slate-900 cursor-pointer transition-colors"
              onClick={() => router.push('/dashboard')}
            >
              Endpoints
            </span>
            <ChevronIcon />
            <span className="text-slate-900 font-medium font-mono text-xs">{id}</span>
          </nav>
        </div>

        <div className="flex items-center justify-between ml-8">
          {/* Hook URL */}
          <div className="flex items-center gap-2">
            <code className="text-xs font-mono text-slate-600 bg-slate-100 border border-slate-200 px-2.5 py-1 rounded-md">
              {hookUrl}
            </code>
            <CopyButton text={hookUrl} label="Copy URL" />
          </div>

          <div className="flex items-center gap-3">
            {/* Expiry countdown */}
            {countdown && (
              <span className={cn(
                'text-xs',
                countdown === 'Expired' ? 'text-red-500' : 'text-amber-600',
              )}>
                Expires in {countdown}
              </span>
            )}

            {/* Delete button */}
            <button
              onClick={() => setConfirmDelete(true)}
              className="flex items-center gap-1.5 text-xs text-slate-400 hover:text-red-600 border border-slate-200 hover:border-red-200 hover:bg-red-50 px-2.5 py-1.5 rounded-lg transition-colors"
            >
              <TrashIcon /> Delete
            </button>

            {/* Live indicator */}
            <div className="flex items-center gap-1.5 text-xs text-slate-400">
              <span className={cn(
                'w-1.5 h-1.5 rounded-full transition-all',
                connected ? 'bg-emerald-400' : 'bg-slate-300',
                pulse ? 'scale-150 bg-emerald-300' : '',
              )} />
              {connected ? 'Live' : 'Connecting…'}
            </div>
          </div>
        </div>
      </div>

      <div className="px-8 py-6 flex-1">
        {/* Code examples */}
        <CodeExamples hookUrl={hookUrl} />

        {/* Requests table */}
        <div className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden mt-6">

          {/* Table header */}
          <div className="flex items-center justify-between px-5 py-2.5 bg-slate-50 border-b border-slate-200">
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wide">
              Requests
            </h2>
            {requests.length > 0 && (
              <span className="text-xs text-slate-400">{requests.length} captured</span>
            )}
          </div>

          {/* Column headings */}
          {!loading && requests.length > 0 && (
            <div className="grid grid-cols-[90px_1fr_140px_100px] gap-4 px-5 py-2 bg-slate-50 border-b border-slate-100 sticky top-0 z-10">
              {['Method', 'Request ID', 'Received', 'Action'].map(col => (
                <span key={col} className="text-xs font-medium text-slate-400 uppercase tracking-wide">{col}</span>
              ))}
            </div>
          )}

          {/* Scrollable rows — max 15 rows (~600px) then scroll */}
          <div className="overflow-y-auto" style={{ maxHeight: '600px' }}>
            {/* Loading skeleton */}
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

            {/* Error */}
            {!loading && error && (
              <div className="px-5 py-4 text-sm text-red-600">{error}</div>
            )}

            {/* Empty state */}
            {!loading && !error && requests.length === 0 && (
              <div className="px-5 py-14 text-center">
                <p className="text-sm font-medium text-slate-600">Waiting for requests…</p>
                <p className="text-sm text-slate-400 mt-1">
                  Use one of the examples above to send your first webhook.
                </p>
              </div>
            )}

            {/* Rows */}
            {!loading && requests.map((req, i) => (
              <div
                key={req.id}
                onClick={() => setSelected(req)}
                className={cn(
                  'grid grid-cols-[90px_1fr_140px_100px] gap-4 items-center px-5 py-3.5 cursor-pointer transition-colors group',
                  selected?.id === req.id ? 'bg-violet-50' : 'hover:bg-slate-50',
                  i < requests.length - 1 ? 'border-b border-slate-100' : '',
                )}
              >
                <MethodBadge method={req.method} />
                <span className="text-xs font-mono text-slate-500 truncate">{req.id}</span>
                <span className="text-sm text-slate-500">{timeAgo(req.created_at)}</span>
                <div className="flex justify-start" onClick={e => e.stopPropagation()}>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => api.requests.replay(req.id)}
                    className="opacity-0 group-hover:opacity-100"
                  >
                    <ReplayIcon /> Replay
                  </Button>
                </div>
              </div>
            ))}
          </div>
          {/* end scrollable rows */}

        </div>
        {/* end table card */}
      </div>

      <RequestDrawer
        request={selected}
        onClose={() => setSelected(null)}
        onReplay={reqId => api.requests.replay(reqId)}
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

function SpinnerIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="animate-spin">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  );
}
