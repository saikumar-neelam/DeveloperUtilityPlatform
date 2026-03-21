import { cn } from '@/lib/utils';

const METHOD_STYLES: Record<string, string> = {
  GET:     'bg-blue-50    text-blue-700    border-blue-200',
  POST:    'bg-emerald-50 text-emerald-700 border-emerald-200',
  PUT:     'bg-amber-50   text-amber-700   border-amber-200',
  PATCH:   'bg-purple-50  text-purple-700  border-purple-200',
  DELETE:  'bg-red-50     text-red-700     border-red-200',
  OPTIONS: 'bg-slate-50   text-slate-600   border-slate-200',
};

export function MethodBadge({ method }: { method: string }) {
  const style = METHOD_STYLES[method.toUpperCase()] ?? METHOD_STYLES.OPTIONS;
  return (
    <span className={cn(
      'inline-flex items-center px-2 py-0.5 rounded text-[11px] font-mono font-semibold border',
      style,
    )}>
      {method.toUpperCase()}
    </span>
  );
}

export function StatusBadge({ code }: { code?: number }) {
  if (!code) return <span className="text-slate-400 text-sm">—</span>;

  const style =
    code < 300 ? 'bg-emerald-50 text-emerald-700 border-emerald-200' :
    code < 400 ? 'bg-blue-50    text-blue-700    border-blue-200'    :
    code < 500 ? 'bg-amber-50   text-amber-700   border-amber-200'   :
                 'bg-red-50     text-red-700     border-red-200';

  return (
    <span className={cn('inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-medium border', style)}>
      {code}
    </span>
  );
}
