'use client';

import { useState } from 'react';
import { cn } from '@/lib/utils';
import { CopyButton } from '@/components/ui/CopyButton';

interface Props { hookUrl: string }
type Lang = 'curl' | 'javascript' | 'python' | 'nodejs';

const TABS: { id: Lang; label: string }[] = [
  { id: 'curl',       label: 'cURL'       },
  { id: 'javascript', label: 'JavaScript' },
  { id: 'python',     label: 'Python'     },
  { id: 'nodejs',     label: 'Node.js'    },
];

function buildSnippets(url: string): Record<Lang, string> {
  return {
    curl: `curl -X POST "${url}" \\
  -H "Content-Type: application/json" \\
  -H "X-Custom-Header: my-value" \\
  -d '{
    "event": "order.created",
    "data": { "id": "ord_123", "amount": 4999, "currency": "usd" }
  }'`,

    javascript: `const res = await fetch("${url}", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "X-Custom-Header": "my-value",
  },
  body: JSON.stringify({
    event: "order.created",
    data: { id: "ord_123", amount: 4999, currency: "usd" },
  }),
});
const result = await res.json();
console.log(result); // { request_id, endpoint_id, status }`,

    python: `import requests

r = requests.post(
    "${url}",
    headers={"Content-Type": "application/json", "X-Custom-Header": "my-value"},
    json={"event": "order.created", "data": {"id": "ord_123", "amount": 4999}},
)
print(r.json())  # { request_id, endpoint_id, status }`,

    nodejs: `const axios = require("axios");

const { data } = await axios.post("${url}", {
  event: "order.created",
  data: { id: "ord_123", amount: 4999, currency: "usd" },
}, { headers: { "Content-Type": "application/json" } });

console.log(data); // { request_id, endpoint_id, status }`,
  };
}

export function CodeExamples({ hookUrl }: Props) {
  const [lang, setLang] = useState<Lang>('curl');
  const snippets = buildSnippets(hookUrl);

  return (
    <div className="bg-white border border-slate-200 rounded-xl shadow-sm overflow-hidden">
      {/* Card header */}
      <div className="flex items-center gap-2 px-5 py-3 border-b border-slate-200 bg-slate-50">
        <TerminalIcon />
        <span className="text-xs font-semibold text-slate-600 uppercase tracking-wide">Test your endpoint</span>
        <span className="text-xs text-slate-400 ml-1">— send a request and watch it appear in the table</span>
      </div>

      {/* Lang tabs */}
      <div className="flex items-center justify-between px-5 border-b border-slate-200 bg-white">
        <div className="flex">
          {TABS.map(({ id, label }) => (
            <button
              key={id}
              onClick={() => setLang(id)}
              className={cn(
                'px-1 py-3 mr-5 text-sm border-b-2 -mb-px transition-colors',
                lang === id
                  ? 'border-violet-600 text-violet-700 font-medium'
                  : 'border-transparent text-slate-500 hover:text-slate-700',
              )}
            >
              {label}
            </button>
          ))}
        </div>
        <CopyButton text={snippets[lang]} />
      </div>

      {/* Code */}
      <div className="bg-slate-950 px-5 py-4 overflow-x-auto">
        <pre className="text-[13px] font-mono leading-relaxed text-slate-300 whitespace-pre">
          <Highlighted code={snippets[lang]} lang={lang} />
        </pre>
      </div>
    </div>
  );
}

// ── Minimal syntax tokeniser ──────────────────────────────────────────────────

type TokType = 'keyword' | 'string' | 'number' | 'comment' | 'flag' | 'url' | 'plain';

const COLOR: Record<TokType, string> = {
  keyword: 'text-purple-400',
  string:  'text-emerald-300',
  number:  'text-sky-300',
  comment: 'text-slate-500',
  flag:    'text-amber-300',
  url:     'text-violet-300',
  plain:   'text-slate-300',
};

function Highlighted({ code, lang }: { code: string; lang: Lang }) {
  const rules: [RegExp, TokType][] = lang === 'curl'
    ? [
        [/^(curl|-X|-H|-d|--data|--header)\b/, 'flag'],
        [/^"https?:\/\/[^\s"]*"/, 'url'],
        [/^"(?:[^"\\]|\\.)*"/, 'string'],
        [/^'(?:[^'\\]|\\.)*'/, 'string'],
        [/^#[^\n]*/, 'comment'],
        [/^\d+/, 'number'],
      ]
    : lang === 'python'
    ? [
        [/^#[^\n]*/, 'comment'],
        [/^(import|from|def|return|print|await|async|if|else|for|in|True|False|None)\b/, 'keyword'],
        [/^"(?:[^"\\]|\\.)*"/, 'string'],
        [/^'(?:[^'\\]|\\.)*'/, 'string'],
        [/^-?\d+/, 'number'],
      ]
    : [
        [/^\/\/[^\n]*/, 'comment'],
        [/^(const|let|var|await|async|function|return|require|console\.log)\b/, 'keyword'],
        [/^"(?:[^"\\]|\\.)*"/, 'string'],
        [/^`(?:[^`\\]|\\.)*`/, 'string'],
        [/^'(?:[^'\\]|\\.)*'/, 'string'],
        [/^-?\d+/, 'number'],
      ];

  const tokens: { type: TokType; text: string }[] = [];
  let src = code;
  while (src.length > 0) {
    let hit = false;
    for (const [re, type] of rules) {
      const m = src.match(re);
      if (m) {
        tokens.push({ type, text: m[0] });
        src = src.slice(m[0].length);
        hit = true; break;
      }
    }
    if (!hit) {
      const last = tokens[tokens.length - 1];
      if (last?.type === 'plain') last.text += src[0];
      else tokens.push({ type: 'plain', text: src[0] });
      src = src.slice(1);
    }
  }

  return (
    <>
      {tokens.map((t, i) => (
        <span key={i} className={COLOR[t.type]}>{t.text}</span>
      ))}
    </>
  );
}

function TerminalIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-slate-400">
      <polyline points="4 17 10 11 4 5" /><line x1="12" y1="19" x2="20" y2="19" />
    </svg>
  );
}
