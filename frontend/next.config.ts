import type { NextConfig } from 'next';

const BACKEND = process.env.BACKEND_URL ?? 'http://localhost:8080';

const nextConfig: NextConfig = {
  // Proxy /api/* and /hook/* to the Go backend.
  // This means the webhook URL the user copies is simply
  // http://localhost:3000/hook/{id} — no CORS, no separate port.
  async rewrites() {
    return [
      { source: '/api/:path*', destination: `${BACKEND}/api/:path*` },
      { source: '/hook/:path*', destination: `${BACKEND}/hook/:path*` },
    ];
  },
};

export default nextConfig;
