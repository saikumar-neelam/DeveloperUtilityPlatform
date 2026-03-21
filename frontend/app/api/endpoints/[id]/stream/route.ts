// Streaming proxy for SSE — Next.js rewrites buffer responses, so we need a
// Route Handler that pipes the backend stream directly to the browser.
const BACKEND = process.env.BACKEND_URL ?? 'http://localhost:8080';

export async function GET(
  _req: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;

  const upstream = await fetch(`${BACKEND}/api/endpoints/${id}/stream`, {
    headers: { Accept: 'text/event-stream', 'Cache-Control': 'no-cache' },
    // @ts-expect-error — Node.js fetch needs this to disable body buffering
    duplex: 'half',
  });

  return new Response(upstream.body, {
    headers: {
      'Content-Type':  'text/event-stream',
      'Cache-Control': 'no-cache',
      'Connection':    'keep-alive',
      'X-Accel-Buffering': 'no',
    },
  });
}
