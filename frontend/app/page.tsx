'use client';

import { useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { timeAgo } from '@/lib/utils';
import type { Endpoint } from '@/lib/types';

const STORAGE_KEY = 'webhookdb_recent';
const MAX_RECENT = 8;

function loadRecent(): Endpoint[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]');
  } catch {
    return [];
  }
}

function saveRecent(ep: Endpoint) {
  const list = loadRecent().filter(e => e.id !== ep.id);
  list.unshift(ep);
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list.slice(0, MAX_RECENT)));
}

export default function LandingPage() {
  const router = useRouter();
  const { user } = useAuth();
  const [targetUrl, setTargetUrl] = useState('');
  const [ttl, setTtl]             = useState('24h');
  const [creating, setCreating]   = useState(false);
  const [error, setError]         = useState('');
  const [recent, setRecent]       = useState<Endpoint[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setRecent(loadRecent());
  }, []);

  async function handleGenerate() {
    setCreating(true);
    setError('');
    try {
      const ep = await api.endpoints.create(targetUrl.trim() || undefined, ttl);
      saveRecent(ep);
      router.push(`/endpoints/${ep.id}`);
    } catch {
      setError('Failed to generate endpoint. Is the backend running?');
      setCreating(false);
    }
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'Enter') handleGenerate();
  }

  const hookUrl = (id: string) =>
    typeof window !== 'undefined' ? `${window.location.origin}/hook/${id}` : `/hook/${id}`;

  const now = Date.now();
  const active = recent.filter(e => new Date(e.expires_at).getTime() > now);

  return (
    <div className="min-h-screen flex flex-col bg-white">

      {/* Top nav */}
      <header className="border-b border-slate-100 px-6 h-14 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2.5">
          <span className="text-violet-600"><WebhookIcon /></span>
          <span className="text-sm font-semibold text-slate-900 tracking-tight">WebhookDB</span>
        </div>
        <div className="flex items-center gap-3">
          {user ? (
            <a href="/dashboard" className="text-xs font-medium text-violet-600 hover:text-violet-700 transition-colors">
              Dashboard →
            </a>
          ) : (
            <>
              <a href="/auth/signin" className="text-xs text-slate-500 hover:text-slate-700 transition-colors">Sign in</a>
              <a href="/auth/signup" className="text-xs font-medium bg-violet-600 hover:bg-violet-700 text-white px-3 py-1.5 rounded-lg transition-colors">Get started</a>
            </>
          )}
        </div>
      </header>

      {/* Hero */}
      <main className="flex-1 flex flex-col items-center justify-center px-4 sm:px-6 py-10 sm:py-20">
        <div className="w-full max-w-xl text-center">

          {/* Badge */}
          <div className="inline-flex items-center gap-1.5 bg-violet-50 text-violet-700 text-xs font-medium px-3 py-1 rounded-full border border-violet-100 mb-6">
            <span className="w-1.5 h-1.5 rounded-full bg-violet-500" />
            No signup required · Free for 24 hours
          </div>

          <h1 className="text-3xl sm:text-4xl font-bold text-slate-900 leading-tight tracking-tight mb-4">
            Inspect webhooks<br />instantly.
          </h1>
          <p className="text-slate-500 text-lg mb-10">
            Generate a unique URL, send webhooks to it, and inspect every request in real time.
          </p>

          {/* Input + CTA */}
          <div className="bg-slate-50 border border-slate-200 rounded-xl p-4 mb-4 text-left space-y-3">
            <div>
              <label className="block text-xs font-medium text-slate-500 mb-1.5">
                Forward to (optional)
              </label>
              <input
                ref={inputRef}
                type="url"
                value={targetUrl}
                onChange={e => setTargetUrl(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="https://your-api.com/webhooks"
                className="w-full text-sm bg-white border border-slate-200 rounded-lg px-3 py-2.5 outline-none focus:ring-2 focus:ring-violet-500 focus:border-transparent placeholder:text-slate-400 transition"
                disabled={creating}
              />
              <p className="text-xs text-slate-400 mt-1.5">
                Leave blank to capture only. Add a URL to also forward received webhooks.
              </p>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-500 mb-1.5">
                Endpoint lifetime
              </label>
              <div className="flex flex-wrap gap-2">
                {[
                  { value: '24h',   label: '24 hours' },
                  { value: '7d',    label: '7 days' },
                  { value: '30d',   label: '30 days' },
                  { value: 'never', label: 'Never' },
                ].map(opt => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setTtl(opt.value)}
                    disabled={creating}
                    className={`text-xs px-3 py-1.5 rounded-lg border transition-colors ${
                      ttl === opt.value
                        ? 'bg-violet-600 border-violet-600 text-white'
                        : 'bg-white border-slate-200 text-slate-600 hover:border-violet-400'
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <button
            onClick={handleGenerate}
            disabled={creating}
            className="w-full bg-violet-600 hover:bg-violet-700 disabled:bg-violet-400 text-white font-medium text-sm rounded-xl px-6 py-3.5 transition-colors flex items-center justify-center gap-2"
          >
            {creating ? (
              <>
                <SpinnerIcon />
                Generating…
              </>
            ) : (
              <>
                Generate my webhook URL
                <ArrowRightIcon />
              </>
            )}
          </button>

          {error && (
            <p className="text-sm text-red-600 mt-3">{error}</p>
          )}
        </div>

        {/* How it works */}
        <div className="w-full max-w-xl mt-16 grid grid-cols-1 sm:grid-cols-3 gap-6">
          {[
            { step: '1', title: 'Generate URL',    desc: 'Click the button to get your unique webhook URL instantly.' },
            { step: '2', title: 'Send webhooks',   desc: 'Point any webhook provider at your URL and trigger events.' },
            { step: '3', title: 'Inspect live',    desc: 'See every request in real time — headers, body, and more.' },
          ].map(({ step, title, desc }) => (
            <div key={step} className="text-center">
              <div className="w-8 h-8 rounded-full bg-violet-50 border border-violet-100 text-violet-600 text-sm font-bold flex items-center justify-center mx-auto mb-3">
                {step}
              </div>
              <p className="text-sm font-medium text-slate-900 mb-1">{title}</p>
              <p className="text-xs text-slate-500 leading-relaxed">{desc}</p>
            </div>
          ))}
        </div>

        {/* Recent endpoints */}
        {active.length > 0 && (
          <div className="w-full max-w-xl mt-14">
            <h2 className="text-xs font-semibold text-slate-500 uppercase tracking-wide mb-3">
              Your recent endpoints
            </h2>
            <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
              {active.map((ep, i) => {
                const expiresAt = new Date(ep.expires_at);
                const isNever = expiresAt.getFullYear() >= 9999;
                const hoursLeft = Math.max(0, Math.floor((expiresAt.getTime() - now) / 3_600_000));
                const minsLeft  = Math.max(0, Math.floor(((expiresAt.getTime() - now) % 3_600_000) / 60_000));
                const expiry    = isNever ? 'Never expires' : hoursLeft > 0 ? `${hoursLeft}h ${minsLeft}m left` : `${minsLeft}m left`;

                return (
                  <div
                    key={ep.id}
                    onClick={() => router.push(`/endpoints/${ep.id}`)}
                    className={`flex items-center gap-4 px-4 py-3 cursor-pointer hover:bg-slate-50 transition-colors ${
                      i < active.length - 1 ? 'border-b border-slate-100' : ''
                    }`}
                  >
                    <div className="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-slate-900">{ep.name}</p>
                      <p className="text-xs text-slate-400 font-mono truncate">{hookUrl(ep.id)}</p>
                    </div>
                    <div className="text-right shrink-0">
                      <p className="text-xs text-slate-400">{timeAgo(ep.created_at)}</p>
                      <p className="text-xs text-amber-600">{expiry}</p>
                    </div>
                    <ChevronRightIcon />
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-100 px-6 py-4 text-center">
        <p className="text-xs text-slate-400">
          Choose your endpoint lifetime · All data is ephemeral
        </p>
      </footer>
    </div>
  );
}

function WebhookIcon() {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2" />
      <path d="m6 17 3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06" />
      <path d="m12 6 3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8" />
    </svg>
  );
}
function ArrowRightIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
      <path d="M5 12h14M12 5l7 7-7 7" />
    </svg>
  );
}
function SpinnerIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="animate-spin">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  );
}
function ChevronRightIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-slate-300">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}
