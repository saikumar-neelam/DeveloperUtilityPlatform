'use client';

import { useState } from 'react';
import { cn } from '@/lib/utils';

interface Props {
  hookUrl: string;
}

const TEMPLATES = [
  {
    label: 'GitHub Push',
    contentType: 'application/json',
    body: JSON.stringify({
      ref: 'refs/heads/main',
      repository: { full_name: 'user/repo', html_url: 'https://github.com/user/repo' },
      commits: [{ id: 'abc123', message: 'feat: add new feature', author: { name: 'Dev' } }],
      pusher: { name: 'dev', email: 'dev@example.com' },
    }, null, 2),
  },
  {
    label: 'GitHub PR',
    contentType: 'application/json',
    body: JSON.stringify({
      action: 'opened',
      number: 42,
      pull_request: {
        title: 'Fix: resolve critical bug',
        state: 'open',
        html_url: 'https://github.com/user/repo/pull/42',
        user: { login: 'dev' },
      },
    }, null, 2),
  },
  {
    label: 'Stripe Payment',
    contentType: 'application/json',
    body: JSON.stringify({
      id: 'evt_test_1234',
      type: 'payment_intent.succeeded',
      data: {
        object: {
          id: 'pi_test_1234',
          amount: 2000,
          currency: 'usd',
          status: 'succeeded',
          receipt_email: 'customer@example.com',
        },
      },
    }, null, 2),
  },
  {
    label: 'Stripe Subscription',
    contentType: 'application/json',
    body: JSON.stringify({
      id: 'evt_test_5678',
      type: 'customer.subscription.created',
      data: {
        object: {
          id: 'sub_test_5678',
          status: 'active',
          current_period_end: 1735689600,
          plan: { id: 'price_pro', nickname: 'Pro Monthly', amount: 2900 },
        },
      },
    }, null, 2),
  },
  {
    label: 'Slack Event',
    contentType: 'application/json',
    body: JSON.stringify({
      type: 'event_callback',
      team_id: 'T1234',
      event: {
        type: 'message',
        text: 'Hello from Slack!',
        user: 'U1234',
        channel: 'C1234',
        ts: '1628000000.000100',
      },
    }, null, 2),
  },
  {
    label: 'Generic POST',
    contentType: 'application/json',
    body: JSON.stringify({ event: 'test', timestamp: new Date().toISOString(), data: { key: 'value' } }, null, 2),
  },
  {
    label: 'Form POST',
    contentType: 'application/x-www-form-urlencoded',
    body: 'name=John+Doe&email=john%40example.com&message=Hello+World',
  },
];

interface SimResult {
  status: number;
  body: string;
  durationMs: number;
}

export function WebhookSimulator({ hookUrl }: Props) {
  const [open, setOpen]             = useState(false);
  const [selected, setSelected]     = useState(0);
  const [body, setBody]             = useState(TEMPLATES[0].body);
  const [contentType, setContentType] = useState(TEMPLATES[0].contentType);
  const [sending, setSending]       = useState(false);
  const [result, setResult]         = useState<SimResult | null>(null);

  function pickTemplate(i: number) {
    setSelected(i);
    setBody(TEMPLATES[i].body);
    setContentType(TEMPLATES[i].contentType);
    setResult(null);
  }

  async function handleSend() {
    setSending(true);
    setResult(null);
    const start = Date.now();
    try {
      const res = await fetch(hookUrl, {
        method: 'POST',
        headers: { 'Content-Type': contentType },
        body,
      });
      const text = await res.text();
      setResult({ status: res.status, body: text, durationMs: Date.now() - start });
    } catch (e) {
      setResult({ status: 0, body: String(e), durationMs: Date.now() - start });
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden mt-4">
      {/* Header */}
      <button
        onClick={() => setOpen(o => !o)}
        className="w-full px-4 py-3 flex items-center justify-between hover:bg-slate-50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <SimIcon />
          <span className="text-xs font-semibold text-slate-500 uppercase tracking-wide">Webhook Simulator</span>
        </div>
        <ChevronDownIcon open={open} />
      </button>

      {open && (
        <div className="border-t border-slate-100 p-4 space-y-4">
          {/* Template pills */}
          <div className="flex flex-wrap gap-2">
            {TEMPLATES.map((t, i) => (
              <button
                key={i}
                onClick={() => pickTemplate(i)}
                className={cn(
                  'text-xs px-3 py-1.5 rounded-lg border transition-colors',
                  selected === i
                    ? 'bg-violet-600 border-violet-600 text-white'
                    : 'bg-white border-slate-200 text-slate-600 hover:border-violet-400',
                )}
              >
                {t.label}
              </button>
            ))}
          </div>

          {/* Content-Type */}
          <div className="flex items-center gap-2">
            <span className="text-xs text-slate-500 w-24 shrink-0">Content-Type</span>
            <input
              value={contentType}
              onChange={e => setContentType(e.target.value)}
              className="flex-1 text-xs font-mono border border-slate-200 rounded-lg px-2.5 py-1.5 focus:outline-none focus:ring-1 focus:ring-violet-400"
            />
          </div>

          {/* Body editor */}
          <textarea
            value={body}
            onChange={e => setBody(e.target.value)}
            rows={8}
            className="w-full text-xs font-mono border border-slate-200 rounded-lg px-3 py-2.5 focus:outline-none focus:ring-1 focus:ring-violet-400 resize-y"
          />

          {/* Send + result */}
          <div className="flex items-center justify-between gap-4">
            <button
              onClick={handleSend}
              disabled={sending}
              className="flex items-center gap-2 bg-violet-600 hover:bg-violet-700 disabled:bg-violet-400 text-white text-xs font-medium px-4 py-2 rounded-lg transition-colors"
            >
              {sending ? (
                <><SpinnerIcon /> Sending…</>
              ) : (
                <><SendIcon /> Send to endpoint</>
              )}
            </button>
            {result && (
              <span className={cn(
                'text-xs font-mono font-medium',
                result.status >= 200 && result.status < 300 ? 'text-emerald-600' : 'text-red-500',
              )}>
                {result.status > 0 ? `${result.status}` : 'Error'} · {result.durationMs}ms
              </span>
            )}
          </div>

          {result && (
            <div className="bg-slate-50 border border-slate-200 rounded-lg p-3">
              <p className="text-xs text-slate-400 mb-1">Response</p>
              <pre className="text-xs font-mono text-slate-700 whitespace-pre-wrap break-all">{
                (() => { try { return JSON.stringify(JSON.parse(result.body), null, 2); } catch { return result.body; } })()
              }</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function SimIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-slate-400">
      <polygon points="5 3 19 12 5 21 5 3" />
    </svg>
  );
}
function SendIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <line x1="22" y1="2" x2="11" y2="13" />
      <polygon points="22 2 15 22 11 13 2 9 22 2" />
    </svg>
  );
}
function SpinnerIcon() {
  return <span className="w-3 h-3 rounded-full border-2 border-white/40 border-t-white animate-spin inline-block" />;
}
function ChevronDownIcon({ open }: { open: boolean }) {
  return (
    <svg
      width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
      className={cn('text-slate-400 transition-transform', open ? 'rotate-180' : '')}
    >
      <path d="m6 9 6 6 6-6" />
    </svg>
  );
}
