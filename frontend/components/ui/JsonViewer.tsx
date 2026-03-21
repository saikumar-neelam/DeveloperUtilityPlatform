import { cn } from '@/lib/utils';

interface JsonViewerProps {
  data: unknown;
  className?: string;
}

export function JsonViewer({ data, className }: JsonViewerProps) {
  const isEmpty =
    data === null ||
    data === undefined ||
    (typeof data === 'object' && Object.keys(data as object).length === 0);

  if (isEmpty) {
    return (
      <p className="text-sm text-zinc-600 italic py-2">No data</p>
    );
  }

  return (
    <pre className={cn(
      'text-[13px] font-mono leading-relaxed',
      'bg-zinc-950 border border-zinc-800 rounded-lg p-4',
      'overflow-auto text-zinc-300 whitespace-pre-wrap break-words',
      className,
    )}>
      <TokenizedJson value={data} />
    </pre>
  );
}

// Simple client-side JSON syntax colorizer — no external dependency.
function TokenizedJson({ value }: { value: unknown }) {
  const raw = JSON.stringify(value, null, 2);

  // Regex to split JSON into typed tokens.
  const TOKEN_RE = /("(?:\\.|[^"\\])*"\s*:)|("(?:\\.|[^"\\])*")|(true|false|null)|(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;
  const parts: React.ReactNode[] = [];
  let cursor = 0;

  let match: RegExpExecArray | null;
  while ((match = TOKEN_RE.exec(raw)) !== null) {
    // Plain text (braces, commas, whitespace)
    if (match.index > cursor) {
      parts.push(
        <span key={cursor} className="text-zinc-400">
          {raw.slice(cursor, match.index)}
        </span>,
      );
    }

    const [token] = match;
    if (match[1]) {
      // Object key  "key":
      const key = token.slice(0, token.lastIndexOf(':'));
      parts.push(<span key={match.index} className="text-indigo-300">{key}</span>);
      parts.push(<span key={match.index + 'c'} className="text-zinc-500">:</span>);
    } else if (match[2]) {
      // String value
      parts.push(<span key={match.index} className="text-emerald-300">{token}</span>);
    } else if (match[3]) {
      // Boolean / null
      parts.push(<span key={match.index} className="text-amber-300">{token}</span>);
    } else if (match[4]) {
      // Number
      parts.push(<span key={match.index} className="text-blue-300">{token}</span>);
    }

    cursor = match.index + token.length;
  }

  // Trailing plain text
  if (cursor < raw.length) {
    parts.push(<span key="end" className="text-zinc-400">{raw.slice(cursor)}</span>);
  }

  return <>{parts}</>;
}
