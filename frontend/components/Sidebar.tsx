'use client';

import { useEffect, useState } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import type { Endpoint } from '@/lib/types';

const STORAGE_KEY = 'webhookdb_recent';

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
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list.slice(0, 8)));
}

interface Props {
  open: boolean;
  onClose: () => void;
}

export function Sidebar({ open, onClose }: Props) {
  const router   = useRouter();
  const pathname = usePathname();
  const { user, logout } = useAuth();
  const [recent, setRecent]     = useState<Endpoint[]>([]);
  const [creating, setCreating] = useState(false);

  function handleLogout() {
    logout();
    router.push('/auth/signin');
  }

  useEffect(() => {
    const now = Date.now();
    setRecent(loadRecent().filter(e => new Date(e.expires_at).getFullYear() >= 9999 || new Date(e.expires_at).getTime() > now));
  }, [pathname]);

  async function handleNew() {
    setCreating(true);
    try {
      const ep = await api.endpoints.create();
      saveRecent(ep);
      setRecent(loadRecent().filter(e => new Date(e.expires_at).getFullYear() >= 9999 || new Date(e.expires_at).getTime() > Date.now()));
      router.push(`/endpoints/${ep.id}`);
      onClose();
    } finally {
      setCreating(false);
    }
  }

  function navigate(path: string) {
    router.push(path);
    onClose();
  }

  const activeId = pathname.startsWith('/endpoints/')
    ? pathname.split('/')[2]
    : null;

  return (
    <aside className={cn(
      'fixed left-0 top-0 h-full w-[240px] bg-white border-r border-slate-200 flex flex-col z-30',
      'transition-transform duration-200',
      'md:translate-x-0',
      open ? 'translate-x-0' : '-translate-x-full',
    )}>
      {/* Logo */}
      <div
        className="h-14 flex items-center gap-2.5 px-5 border-b border-slate-200 shrink-0 cursor-pointer"
        onClick={() => navigate('/')}
      >
        <span className="text-violet-600"><WebhookIcon /></span>
        <span className="text-sm font-semibold text-slate-900 tracking-tight">WebhookDB</span>
      </div>

      {/* New endpoint button */}
      <div className="px-3 pt-4 pb-2">
        <button
          onClick={handleNew}
          disabled={creating}
          className="w-full flex items-center justify-center gap-2 bg-violet-600 hover:bg-violet-700 disabled:bg-violet-400 text-white text-xs font-medium rounded-lg px-3 py-2.5 transition-colors"
        >
          {creating ? <SpinnerIcon /> : <PlusIcon />}
          {creating ? 'Generating…' : 'New endpoint'}
        </button>
      </div>

      {/* Recent endpoints */}
      <div className="flex-1 overflow-y-auto px-3 py-2">
        {recent.length > 0 ? (
          <>
            <p className="text-[10px] font-semibold text-slate-400 uppercase tracking-wider px-2 mb-1.5">Recent</p>
            <nav className="space-y-0.5">
              {recent.map(ep => {
                const isActive = ep.id === activeId;
                const expiresAt = new Date(ep.expires_at);
                const never = expiresAt.getFullYear() >= 9999;
                const hoursLeft = Math.max(0, Math.floor((expiresAt.getTime() - Date.now()) / 3_600_000));
                const expiry = never ? 'Never expires' : hoursLeft > 0 ? `${hoursLeft}h left` : '< 1h left';

                return (
                  <button
                    key={ep.id}
                    onClick={() => navigate(`/endpoints/${ep.id}`)}
                    className={cn(
                      'w-full flex flex-col items-start px-3 py-2 rounded-md text-left transition-colors',
                      isActive ? 'bg-violet-50 text-violet-700' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900',
                    )}
                  >
                    <span className="text-xs font-medium truncate w-full">{ep.name}</span>
                    <span className={cn(
                      'text-[10px] mt-0.5',
                      isActive ? 'text-violet-400' : never ? 'text-emerald-500' : hoursLeft < 2 ? 'text-amber-500' : 'text-slate-400',
                    )}>
                      {expiry}
                    </span>
                  </button>
                );
              })}
            </nav>
          </>
        ) : (
          <p className="text-xs text-slate-400 px-2 py-2">No recent endpoints.</p>
        )}
      </div>

      {/* Footer */}
      <div className="px-4 py-3 border-t border-slate-200 shrink-0">
        {user ? (
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-full bg-violet-100 flex items-center justify-center shrink-0">
              <span className="text-[10px] font-bold text-violet-700 uppercase">{user.name[0]}</span>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium text-slate-700 truncate">{user.name}</p>
              <p className="text-[10px] text-slate-400 truncate">{user.email}</p>
            </div>
            <button onClick={handleLogout} title="Sign out" className="text-slate-300 hover:text-red-500 transition-colors">
              <LogoutIcon />
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-2">
            <div className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
            <span className="text-xs text-slate-500">All systems operational</span>
          </div>
        )}
      </div>
    </aside>
  );
}

function WebhookIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M18 16.98h-5.99c-1.1 0-1.95.94-2.48 1.9A4 4 0 0 1 2 17c.01-.7.2-1.4.57-2" />
      <path d="m6 17 3.13-5.78c.53-.97.1-2.18-.5-3.1a4 4 0 1 1 6.89-4.06" />
      <path d="m12 6 3.13 5.73C15.66 12.7 16.9 13 18 13a4 4 0 0 1 0 8" />
    </svg>
  );
}
function PlusIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}
function SpinnerIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" className="animate-spin">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  );
}
function LogoutIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  );
}
