'use client';

import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/Button';
import type { CreateEndpointPayload, Endpoint } from '@/lib/types';

interface Props {
  onClose: () => void;
  onCreate: (payload: CreateEndpointPayload) => Promise<Endpoint>;
}

export function CreateEndpointModal({ onClose, onCreate }: Props) {
  const [name, setName]           = useState('');
  const [targetUrl, setTargetUrl] = useState('');
  const [loading, setLoading]     = useState(false);
  const [error, setError]         = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) { setError('Name is required'); return; }
    setLoading(true); setError('');
    try {
      await onCreate({ name: name.trim(), target_url: targetUrl.trim() || undefined });
      onClose();
    } catch {
      setError('Failed to create endpoint. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-slate-900/30 backdrop-blur-[2px]" onClick={onClose} />

      {/* Modal */}
      <div className="relative w-full max-w-md bg-white rounded-xl border border-slate-200 shadow-xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-200">
          <h2 className="text-sm font-semibold text-slate-900">Create endpoint</h2>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-600 transition-colors">
            <XIcon />
          </button>
        </div>

        {/* Body */}
        <form onSubmit={handleSubmit} className="px-6 py-5 space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1.5">
              Name <span className="text-slate-400 font-normal">(required)</span>
            </label>
            <input
              ref={inputRef}
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="e.g. stripe-production"
              className={INPUT}
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-700 mb-1.5">
              Replay target URL <span className="text-slate-400 font-normal">(optional)</span>
            </label>
            <input
              type="url"
              value={targetUrl}
              onChange={e => setTargetUrl(e.target.value)}
              placeholder="https://your-api.com/webhooks"
              className={INPUT}
            />
            <p className="mt-1.5 text-xs text-slate-400">
              Where replays are forwarded. Can be overridden per-replay.
            </p>
          </div>

          {error && (
            <div className="flex items-start gap-2 text-sm text-red-600 bg-red-50 border border-red-200 rounded-md px-3 py-2.5">
              <AlertIcon />
              {error}
            </div>
          )}

          {/* Footer */}
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="secondary" onClick={onClose}>Cancel</Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Creating…' : 'Create endpoint'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}

const INPUT =
  'w-full bg-white border border-slate-300 rounded-md px-3 py-2 text-sm text-slate-900 ' +
  'placeholder:text-slate-400 focus:outline-none focus:border-violet-500 ' +
  'focus:ring-2 focus:ring-violet-500/20 transition-colors';

function XIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M18 6 6 18M6 6l12 12" />
    </svg>
  );
}

function AlertIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="shrink-0 mt-0.5">
      <circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="12" /><line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  );
}
