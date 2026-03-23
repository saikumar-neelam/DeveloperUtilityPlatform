import type { NextConfig } from 'next';

const BACKEND = process.env.BACKEND_URL ?? 'http://localhost:8080';

const nextConfig: NextConfig = {
  output: 'standalone',
  // Proxy /api/* and /hook/* to the Go backend.
  // This means the webhook URL the user copies is simply
  // http://localhost:3000/hook/{id} — no CORS, no separate port.
  async rewrites() {
    return [
      { source: '/api/:path*',      destination: `${BACKEND}/api/:path*`      },
      { source: '/hook/:path*',     destination: `${BACKEND}/hook/:path*`     },
      { source: '/auth/register',   destination: `${BACKEND}/auth/register`   },
      { source: '/auth/login',      destination: `${BACKEND}/auth/login`      },
      { source: '/auth/me',         destination: `${BACKEND}/auth/me`         },
    ];
  },
};

export default nextConfig;
