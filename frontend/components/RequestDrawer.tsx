'use client';

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { MethodBadge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { CopyButton } from '@/components/ui/CopyButton';
import { JsonViewer } from '@/components/ui/JsonViewer';
import { formatDate, formatBytes, cn } from '@/lib/utils';
import type { WebhookRequest } from '@/lib/types';

type Tab = 'body' | 'headers' | 'query';
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

interface Props {
  request: WebhookRequest | null;
  onClose: () => void;
  onReplay: (requestId: string) => Promise<void>;
}

export function RequestDrawer({ request, onClose, onReplay }: Props) {
  const [tab, setTab]             = useState<Tab>('body');
  const [body, setBody]           = useState<BodyState>({ status: 'idle' });
  const [detail, setDetail]       = useState<DetailState>({ status: 'idle' });
  const [replaying, setReplaying] = useState(false);
  const [replayMsg, setReplayMsg] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    setTab('body');
    setReplayMsg(null);
    setBody({ status: 'idle' });
    setDetail({ status: 'idle' });
    if (!request) return;

    // Fetch full request (headers + query params) from dedicated endpoint.
    setDetail({ status: 'loading' });
    api.requests.get(request.id)
      .then(r => setDetail({ status: 'ok', request: r }))
      .catch(() => setDetail({ status: 'error' }));

    // Fetch body bytes from S3 if non-empty.
    if (request.body_size === 0) return;
    setBody({ status: 'loading' });
    api.requests.body(request.id)
      .then(text => setBody({ status: 'ok', text }))
      .catch(() => setBody({ status: 'error' }));
  }, [request?.id]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  const handleReplay = async () => {
    if (!request) return;
    setReplaying(true); setReplayMsg(null);
    try {
      await onReplay(request.id);
      setReplayMsg({ ok: true, text: 'Replay queued successfully' });
    } catch {
      setReplayMsg({ ok: false, text: 'Replay failed — check target URL' });
    } finally {
      setReplaying(false);
    }
  };

  if (!request) return null;

  const TABS: { id: Tab; label: string }[] = [
    { id: 'body',    label: 'Body'    },
    { id: 'headers', label: 'Headers' },
    { id: 'query',   label: 'Query'   },
  ];

  const bodyJson = (() => {
    if (body.status !== 'ok') return null;
    try { return JSON.parse(body.text); } catch { return null; }
  })();

  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 bg-slate-900/20 backdrop-blur-[1px] z-40" onClick={onClose} />

      {/* Panel */}
      <div className="fixed right-0 top-0 h-full w-[540px] bg-white border-l border-slate-200 z-50 flex flex-col shadow-2xl">

        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-slate-200 shrink-0">
          <div className="flex items-center gap-3">
            <MethodBadge method={request.method} />
            <span className="text-xs text-slate-500">{formatDate(request.created_at)}</span>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-100 transition-colors"
          >
            <XIcon />
          </button>
        </div>

        {/* Meta strip */}
        <div className="flex items-center gap-5 px-5 py-2.5 bg-slate-50 border-b border-slate-200 shrink-0">
          <Meta label="ID" value={request.id} mono />
          <Meta label="Size" value={formatBytes(request.body_size)} />
          {request.content_type && (
            <Meta label="Type" value={request.content_type.split(';')[0]} mono />
          )}
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
            </button>
          ))}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto p-5">

          {tab === 'headers' && (
            detail.status === 'loading' ? <Spinner /> :
            detail.status === 'error'   ? <FetchError /> :
            detail.status === 'ok'      ? <JsonViewer data={detail.request.headers} /> :
            null
          )}
          {tab === 'query' && (
            detail.status === 'loading' ? <Spinner /> :
            detail.status === 'error'   ? <FetchError /> :
            detail.status === 'ok'      ? <JsonViewer data={detail.request.query_params} /> :
            null
          )}

          {tab === 'body' && (
            <>
              {request.body_size === 0 && (
                <div className="flex items-center justify-center h-24 text-sm text-slate-400">
                  No body was sent with this request.
                </div>
              )}
              {request.body_size > 0 && body.status === 'loading' && (
                <div className="flex items-center gap-2 text-sm text-slate-500 py-4">
                  <span className="w-3.5 h-3.5 rounded-full border-2 border-slate-300 border-t-violet-500 animate-spin" />
                  Loading body…
                </div>
              )}
              {request.body_size > 0 && body.status === 'error' && (
                <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-3 py-2.5">
                  Failed to load body from storage.
                </p>
              )}
              {body.status === 'ok' && (
                bodyJson !== null
                  ? <JsonViewer data={bodyJson} />
                  : (
                    <pre className="text-[13px] font-mono bg-slate-950 text-slate-300 rounded-lg p-4 overflow-auto whitespace-pre-wrap border border-slate-200">
                      {body.text}
                    </pre>
                  )
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="px-5 py-3.5 border-t border-slate-200 flex items-center justify-between shrink-0 bg-slate-50">
          <div>
            {replayMsg && (
              <span className={cn('text-xs font-medium', replayMsg.ok ? 'text-emerald-600' : 'text-red-600')}>
                {replayMsg.text}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {tab === 'body' && body.status === 'ok' && (
              <CopyButton text={body.text} label="Copy body" />
            )}
            {tab === 'headers' && detail.status === 'ok' && (
              <CopyButton text={JSON.stringify(detail.request.headers, null, 2)} label="Copy headers" />
            )}
            <Button variant="secondary" size="sm" onClick={handleReplay} disabled={replaying}>
              <ReplayIcon />
              {replaying ? 'Queuing…' : 'Replay'}
            </Button>
          </div>
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
