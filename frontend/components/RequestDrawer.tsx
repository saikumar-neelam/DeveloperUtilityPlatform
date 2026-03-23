'use client';

import { useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { MethodBadge, StatusBadge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { CopyButton } from '@/components/ui/CopyButton';
import { JsonViewer } from '@/components/ui/JsonViewer';
import { formatDate, formatBytes, cn } from '@/lib/utils';
import type { WebhookRequest, ReplayResult } from '@/lib/types';

type Tab = 'body' | 'headers' | 'query' | 'replay';

type BodyState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; text: string }
  | { status: 'error' };

type DetailState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'ok'; request: WebhookRequest }
  | { status: 'error' };

type ReplayState =
  | { status: 'idle' }
  | { status: 'sending' }
  | { status: 'polling' }
  | { status: 'done'; result: ReplayResult }
  | { status: 'error'; message: string };

interface Props {
  request: WebhookRequest | null;
  onClose: () => void;
  onReplay: (requestId: string, targetUrl?: string) => Promise<void>;
}

export function RequestDrawer({ request, onClose, onReplay }: Props) {
  const [tab, setTab]         = useState<Tab>('body');
  const [body, setBody]       = useState<BodyState>({ status: 'idle' });
  const [detail, setDetail]   = useState<DetailState>({ status: 'idle' });
  const [replay, setReplay]   = useState<ReplayState>({ status: 'idle' });
  const [targetUrl, setTargetUrl] = useState('');
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    setTab('body');
    setBody({ status: 'idle' });
    setDetail({ status: 'idle' });
    setReplay({ status: 'idle' });
    setTargetUrl('');
    if (pollRef.current) clearTimeout(pollRef.current);
    if (!request) return;

    setDetail({ status: 'loading' });
    api.requests.get(request.id)
      .then(r => setDetail({ status: 'ok', request: r }))
      .catch(() => setDetail({ status: 'error' }));

    if (request.body_size === 0) return;
    setBody({ status: 'loading' });
    api.requests.body(request.id)
      .then(text => setBody({ status: 'ok', text }))
      .catch(() => setBody({ status: 'error' }));
  }, [request?.id]);

  // Cleanup polling on unmount
  useEffect(() => () => { if (pollRef.current) clearTimeout(pollRef.current); }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  async function handleReplay() {
    if (!request) return;
    setReplay({ status: 'sending' });
    setTab('replay');
    try {
      await onReplay(request.id, targetUrl || undefined);
      setReplay({ status: 'polling' });
      pollForResult(request.id, 0);
    } catch {
      setReplay({ status: 'error', message: 'Failed to queue replay — check target URL' });
    }
  }

  function pollForResult(requestId: string, attempt: number) {
    if (attempt >= 12) {
      setReplay({ status: 'error', message: 'Timed out waiting for result' });
      return;
    }
    pollRef.current = setTimeout(async () => {
      const result = await api.requests.replayResult(requestId);
      if (result) {
        setReplay({ status: 'done', result });
      } else {
        pollForResult(requestId, attempt + 1);
      }
    }, 1000);
  }

  if (!request) return null;

  const TABS: { id: Tab; label: string }[] = [
    { id: 'body',    label: 'Body'    },
    { id: 'headers', label: 'Headers' },
    { id: 'query',   label: 'Query'   },
    { id: 'replay',  label: 'Replay'  },
  ];

  const bodyJson = (() => {
    if (body.status !== 'ok') return null;
    try { return JSON.parse(body.text); } catch { return null; }
  })();

  const replayResultJson = (() => {
    if (replay.status !== 'done') return null;
    try { return JSON.parse(replay.result.response_body); } catch { return null; }
  })();

  return (
    <>
      <div className="fixed inset-0 bg-slate-900/20 backdrop-blur-[1px] z-40" onClick={onClose} />

      <div className="fixed right-0 top-0 h-full w-full sm:w-[540px] bg-white border-l border-slate-200 z-50 flex flex-col shadow-2xl">

        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-slate-200 shrink-0">
          <div className="flex items-center gap-3">
            <MethodBadge method={request.method} />
            <span className="text-xs text-slate-500">{formatDate(request.created_at)}</span>
          </div>
          <button onClick={onClose} className="p-1.5 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors">
            <XIcon />
          </button>
        </div>

        {/* Meta strip */}
        <div className="flex items-center gap-5 px-5 py-2.5 bg-slate-50 border-b border-slate-200 shrink-0">
          <Meta label="ID" value={request.id} mono />
          <Meta label="Size" value={formatBytes(request.body_size)} />
          {request.content_type && <Meta label="Type" value={request.content_type.split(';')[0]} mono />}
        </div>

        {/* Tabs */}
        <div className="flex border-b border-slate-200 px-5 shrink-0">
          {TABS.map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setTab(id)}
              className={cn(
                'px-1 py-3 mr-5 text-sm border-b-2 -mb-px transition-colors',
                tab === id
                  ? 'border-violet-600 text-violet-700 font-medium'
                  : 'border-transparent text-slate-500 hover:text-slate-700',
              )}
            >
              {label}
              {id === 'replay' && replay.status === 'done' && (
                <span className="ml-1.5 inline-block w-1.5 h-1.5 rounded-full bg-emerald-500 mb-0.5" />
              )}
            </button>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto p-5">

          {tab === 'headers' && (
            detail.status === 'loading' ? <Spinner /> :
            detail.status === 'error'   ? <FetchError /> :
            detail.status === 'ok'      ? <JsonViewer data={detail.request.headers} /> : null
          )}
          {tab === 'query' && (
            detail.status === 'loading' ? <Spinner /> :
            detail.status === 'error'   ? <FetchError /> :
            detail.status === 'ok'      ? <JsonViewer data={detail.request.query_params} /> : null
          )}

          {tab === 'body' && (
            <>
              {request.body_size === 0 && (
                <div className="flex items-center justify-center h-24 text-sm text-slate-400">No body was sent with this request.</div>
              )}
              {request.body_size > 0 && body.status === 'loading' && (
                <div className="flex items-center gap-2 text-sm text-slate-500 py-4">
                  <span className="w-3.5 h-3.5 rounded-full border-2 border-slate-300 border-t-violet-500 animate-spin" />
                  Loading body…
                </div>
              )}
              {request.body_size > 0 && body.status === 'error' && (
                <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-3 py-2.5">Failed to load body.</p>
              )}
              {body.status === 'ok' && (
                bodyJson !== null
                  ? <JsonViewer data={bodyJson} />
                  : <pre className="text-[13px] font-mono bg-slate-950 text-slate-300 rounded-lg p-4 overflow-auto whitespace-pre-wrap border border-slate-200">{body.text}</pre>
              )}
            </>
          )}

          {tab === 'replay' && (
            <div className="space-y-4">
              {/* Target URL input */}
              <div>
                <label className="block text-xs font-medium text-slate-500 mb-1">Target URL</label>
                <input
                  type="url"
                  value={targetUrl}
                  onChange={e => setTargetUrl(e.target.value)}
                  placeholder="https://your-server.com/webhook (leave blank for endpoint default)"
                  className="w-full px-3 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:ring-1 focus:ring-violet-400"
                  disabled={replay.status === 'sending' || replay.status === 'polling'}
                />
              </div>

              <Button
                variant="primary"
                size="sm"
                onClick={handleReplay}
                disabled={replay.status === 'sending' || replay.status === 'polling'}
              >
                <ReplayIcon />
                {replay.status === 'sending' ? 'Sending…' :
                 replay.status === 'polling' ? 'Waiting for result…' : 'Send replay'}
              </Button>

              {/* Result */}
              {replay.status === 'polling' && (
                <div className="flex items-center gap-2 text-sm text-slate-500 py-2">
                  <span className="w-3.5 h-3.5 rounded-full border-2 border-slate-300 border-t-violet-500 animate-spin" />
                  Waiting for result…
                </div>
              )}
              {replay.status === 'error' && (
                <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-3 py-2.5">{replay.message}</p>
              )}
              {replay.status === 'done' && (
                <div className="space-y-3 border border-slate-200 rounded-xl p-4 bg-slate-50">
                  <div className="flex items-center gap-3">
                    <StatusBadge code={replay.result.status_code} />
                    <span className="text-xs text-slate-500">{replay.result.duration_ms}ms</span>
                    {replay.result.error && (
                      <span className="text-xs text-red-500">{replay.result.error}</span>
                    )}
                  </div>
                  {replay.result.response_body && (
                    <div>
                      <p className="text-xs font-medium text-slate-500 mb-1.5">Response body</p>
                      {replayResultJson !== null
                        ? <JsonViewer data={replayResultJson} />
                        : <pre className="text-[12px] font-mono bg-slate-950 text-slate-300 rounded-lg p-3 overflow-auto whitespace-pre-wrap">{replay.result.response_body}</pre>
                      }
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-5 py-3.5 border-t border-slate-200 flex items-center justify-end shrink-0 bg-slate-50 gap-2">
          {tab === 'body' && body.status === 'ok' && <CopyButton text={body.text} label="Copy body" />}
          {tab === 'headers' && detail.status === 'ok' && (
            <CopyButton text={JSON.stringify(detail.request.headers, null, 2)} label="Copy headers" />
          )}
          {tab !== 'replay' && (
            <Button variant="secondary" size="sm" onClick={() => { setTab('replay'); handleReplay(); }}>
              <ReplayIcon /> Replay
            </Button>
          )}
        </div>
      </div>
    </>
  );
}

function Meta({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <span className="text-xs text-slate-500">
      {label}{' '}
      <span className={cn('text-slate-700', mono && 'font-mono')}>{value}</span>
    </span>
  );
}

function Spinner() {
  return (
    <div className="flex items-center gap-2 text-sm text-slate-500 py-4">
      <span className="w-3.5 h-3.5 rounded-full border-2 border-slate-300 border-t-violet-500 animate-spin" />
      Loading…
    </div>
  );
}

function FetchError() {
  return <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-3 py-2.5">Failed to load data.</p>;
}

function XIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  );
}

function ReplayIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
      <path d="M3 3v5h5" />
    </svg>
  );
}
