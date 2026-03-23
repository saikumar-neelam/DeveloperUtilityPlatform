'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { formatDate } from '@/lib/utils';
import type { Endpoint } from '@/lib/types';

export default function DashboardPage() {
  const router = useRouter();
  const [endpoints, setEndpoints] = useState<Endpoint[]>([]);
  const [loading, setLoading]     = useState(true);
  const [creating, setCreating]   = useState(false);

  useEffect(() => {
    api.endpoints.list()
      .then(setEndpoints)
      .finally(() => setLoading(false));
  }, []);

  async function handleNew() {
    setCreating(true);
    try {
      const ep = await api.endpoints.create();
      router.push(`/endpoints/${ep.id}`);
    } finally {
      setCreating(false);
    }
  }

  const now = Date.now();

  return (
    <div className="min-h-screen bg-slate-50">
      <main className="max-w-3xl mx-auto px-4 sm:px-6 py-6 sm:py-10">
        <div className="flex items-center justify-between mb-6 gap-3">
          <div>
            <h1 className="text-lg sm:text-xl font-bold text-slate-900">All endpoints</h1>
            <p className="text-sm text-slate-500 mt-0.5">Your webhook endpoints</p>
          </div>
          <button
            onClick={handleNew}
            disabled={creating}
            className="flex items-center gap-1.5 bg-violet-600 hover:bg-violet-700 disabled:bg-violet-400 text-white text-xs font-medium rounded-lg px-3 sm:px-4 py-2.5 transition-colors shrink-0"
          >
            <PlusIcon />
            {creating ? 'Creating…' : 'New endpoint'}
          </button>
        </div>

        {loading && (
          <div className="flex justify-center py-16">
            <span className="w-5 h-5 rounded-full border-2 border-slate-200 border-t-violet-500 animate-spin" />
          </div>
        )}

        {!loading && endpoints.length === 0 && (
          <div className="text-center py-16 sm:py-20 bg-white border border-slate-200 rounded-2xl">
            <p className="text-slate-500 text-sm mb-4">No endpoints yet. Create one to get started.</p>
            <button
              onClick={handleNew}
              disabled={creating}
              className="bg-violet-600 hover:bg-violet-700 text-white text-sm font-medium rounded-lg px-5 py-2.5 transition-colors"
            >
              {creating ? 'Creating…' : 'Create my first endpoint'}
            </button>
          </div>
        )}

        {!loading && endpoints.length > 0 && (
          <div className="bg-white border border-slate-200 rounded-2xl shadow-sm overflow-hidden">
            {endpoints.map((ep, i) => {
              const expiresAt = new Date(ep.expires_at);
              const never = expiresAt.getFullYear() >= 9999;
              const hoursLeft = Math.max(0, Math.floor((expiresAt.getTime() - now) / 3_600_000));
              const expiry = never ? 'Never expires' : hoursLeft > 0 ? `${hoursLeft}h left` : '< 1h left';
              return (
                <div
                  key={ep.id}
                  onClick={() => router.push(`/endpoints/${ep.id}`)}
                  className={`flex items-center gap-3 sm:gap-4 px-4 sm:px-5 py-3.5 cursor-pointer hover:bg-slate-50 transition-colors ${i < endpoints.length - 1 ? 'border-b border-slate-100' : ''}`}
                >
                  <div className="w-2 h-2 rounded-full bg-emerald-400 shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium text-slate-900 truncate">{ep.name}</p>
                    <p className="text-xs text-slate-400 font-mono truncate">/hook/{ep.id}</p>
                  </div>
                  <div className="text-right shrink-0 space-y-0.5">
                    <p className="text-xs text-slate-400 hidden sm:block">{formatDate(ep.created_at)}</p>
                    <p className={`text-[11px] ${never ? 'text-emerald-500' : hoursLeft < 2 ? 'text-amber-500' : 'text-slate-400'}`}>
                      {expiry}
                    </p>
                  </div>
                  <ChevronIcon />
                </div>
              );
            })}
          </div>
        )}
      </main>
    </div>
  );
}

function PlusIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}
function ChevronIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-slate-300 shrink-0">
      <path d="m9 18 6-6-6-6" />
    </svg>
  );
}
